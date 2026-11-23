package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func generateGroupCode() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return "GRP-" + strings.ToUpper(hex.EncodeToString(b[:]))
}

func loadGroup(ctx context.Context, db querier, id string) (domain.CapstoneGroup, error) {
	var g domain.CapstoneGroup
	g.Members = []domain.GroupMember{}
	g.Advisors = []domain.GroupAdvisorBrief{}
	e := db.QueryRow(ctx, `
		SELECT g.id,g.code,g.academic_period_id,ap.name,g.interest_id,i.name,g.working_title,g.initial_description,
		       g.leader_id,coalesce((SELECT m.student_name FROM capstone_service.capstone_group_members m WHERE m.group_id=g.id AND m.student_id=g.leader_id LIMIT 1),''),
		       g.status,p.id,coalesce(p.title,''),coalesce(p.status,''),g.version,g.updated_at
		FROM capstone_service.capstone_groups g
		JOIN capstone_service.academic_periods ap ON ap.id=g.academic_period_id
		JOIN capstone_service.interest_fields i ON i.id=g.interest_id
		LEFT JOIN capstone_service.projects p ON p.group_id=g.id
		WHERE g.id=$1`, id).Scan(
		&g.ID, &g.Code, &g.AcademicPeriodID, &g.AcademicPeriodName, &g.InterestID, &g.InterestName, &g.WorkingTitle, &g.InitialDescription,
		&g.LeaderID, &g.LeaderName, &g.Status, &g.ProjectID, &g.ProjectTitle, &g.ProjectStatus, &g.Version, &g.UpdatedAt,
	)
	if e != nil {
		return g, e
	}
	mRows, e := db.Query(ctx, `SELECT student_id,student_name,nim,is_leader,responsibility,joined_at FROM capstone_service.capstone_group_members WHERE group_id=$1 ORDER BY is_leader DESC, nim ASC`, id)
	if e != nil {
		return g, e
	}
	for mRows.Next() {
		var m domain.GroupMember
		if e = mRows.Scan(&m.StudentID, &m.StudentName, &m.NIM, &m.IsLeader, &m.Responsibility, &m.JoinedAt); e != nil {
			mRows.Close()
			return g, e
		}
		g.Members = append(g.Members, m)
	}
	e = mRows.Err()
	mRows.Close()
	if e != nil {
		return g, e
	}
	if g.ProjectID != nil {
		aRows, e := db.Query(ctx, `SELECT student_id,student_name,slot,candidate_id,candidate_name,supervisor_id,supervisor_name,is_external_supervisor,external_supervisor_name,external_supervisor_institution,title_status,confirmation_status FROM capstone_service.project_students WHERE project_id=$1 ORDER BY nim, slot`, *g.ProjectID)
		if e != nil {
			return g, e
		}
		for aRows.Next() {
			var ab domain.GroupAdvisorBrief
			if e = aRows.Scan(&ab.StudentID, &ab.StudentName, &ab.Slot, &ab.CandidateID, &ab.CandidateName, &ab.SupervisorID, &ab.SupervisorName, &ab.IsExternalSupervisor, &ab.ExternalSupervisorName, &ab.ExternalSupervisorInstitution, &ab.TitleStatus, &ab.ConfirmationStatus); e != nil {
				aRows.Close()
				return g, e
			}
			g.Advisors = append(g.Advisors, ab)
		}
		e = aRows.Err()
		aRows.Close()
		if e != nil {
			return g, e
		}
	}
	return g, nil
}

func (s *Server) listGroups(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.CapstoneGroup{}})
	}
	u := actor(c)
	periodID := c.Query("academic_period_id")
	status := c.Query("status")
	q := strings.TrimSpace(c.Query("q"))
	rows, e := s.DB.Query(c.Context(), `
		SELECT g.id FROM capstone_service.capstone_groups g
		WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.capstone_group_members m WHERE m.group_id=g.id AND m.student_id=$2))
		  AND ($3='' OR g.academic_period_id::text=$3)
		  AND ($4='' OR g.status=$4)
		  AND ($5='' OR g.working_title ILIKE '%'||$5||'%' OR g.code ILIKE '%'||$5||'%')
		ORDER BY g.updated_at DESC LIMIT 200`, u.Staff(), u.ID, periodID, status, q)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	items := []domain.CapstoneGroup{}
	for _, id := range ids {
		g, e := loadGroup(c.Context(), s.DB, id)
		if e != nil {
			return e
		}
		items = append(items, g)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) saveGroup(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Kelompok belum tersedia. Hubungi administrator.")
	}
	var in domain.GroupInput
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrUnprocessableEntity
	}
	leaderID, err := in.Validate(actor(c).ID)
	if err != nil {
		return fiber.NewError(422, err.Error())
	}
	id := c.Params("id")
	create := id == ""
	if create {
		id = uuid.NewString()
	} else if !domain.ValidID(id) {
		return fiber.ErrBadRequest
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	ctx := c.Context()
	if !create {
		var oldStatus string
		var ver int
		if e = tx.QueryRow(ctx, `SELECT status,version FROM capstone_service.capstone_groups WHERE id=$1 FOR UPDATE`, id).Scan(&oldStatus, &ver); e != nil {
			return e
		}
		if e = stale(ver, in.Version); e != nil {
			return e
		}
		var member bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capstone_service.capstone_group_members WHERE group_id=$1 AND student_id=$2)`, id, actor(c).ID).Scan(&member); e != nil {
			return e
		}
		if !member {
			return fiber.ErrForbidden
		}
		if oldStatus == "LOCKED" {
			return fiber.NewError(409, "Kelompok sudah dikunci karena topik telah diajukan.")
		}
	}
	var groupOpen bool
	if e = tx.QueryRow(ctx, `SELECT group_open FROM capstone_service.academic_periods WHERE id=$1`, in.AcademicPeriodID).Scan(&groupOpen); e != nil {
		return e
	}
	if !groupOpen {
		return fiber.NewError(422, "Pendaftaran kelompok belum dibuka pada periode akademik tersebut.")
	}
	var interestActive bool
	if e = tx.QueryRow(ctx, `SELECT is_active FROM capstone_service.interest_fields WHERE id=$1`, in.InterestID).Scan(&interestActive); e != nil {
		return e
	}
	if !interestActive {
		return fiber.NewError(422, "Bidang minat tidak aktif.")
	}
	for _, m := range in.Members {
		var conflictGroup string
		err := tx.QueryRow(ctx, `
			SELECT g.code FROM capstone_service.capstone_group_members gm
			JOIN capstone_service.capstone_groups g ON g.id=gm.group_id
			WHERE gm.student_id=$1 AND g.academic_period_id=$2 AND g.id::text <> $3 LIMIT 1`, m.StudentID, in.AcademicPeriodID, id).Scan(&conflictGroup)
		if err == nil {
			return fiber.NewError(409, fmt.Sprintf("Mahasiswa sudah terdaftar pada kelompok %s di periode yang sama.", conflictGroup))
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	nextStatus := "DRAFT"
	if in.Finalize {
		nextStatus = "READY"
	}
	if create {
		_, e = tx.Exec(ctx, `
			INSERT INTO capstone_service.capstone_groups(id,code,academic_period_id,interest_id,working_title,initial_description,leader_id,status)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, generateGroupCode(), in.AcademicPeriodID, in.InterestID, in.WorkingTitle, in.InitialDescription, leaderID, nextStatus)
	} else {
		_, e = tx.Exec(ctx, `
			UPDATE capstone_service.capstone_groups
			SET academic_period_id=$2,interest_id=$3,working_title=$4,initial_description=$5,leader_id=$6,status=$7,version=version+1,updated_at=now()
			WHERE id=$1`, id, in.AcademicPeriodID, in.InterestID, in.WorkingTitle, in.InitialDescription, leaderID, nextStatus)
		if e == nil {
			_, e = tx.Exec(ctx, `DELETE FROM capstone_service.capstone_group_members WHERE group_id=$1`, id)
		}
	}
	if e != nil {
		return e
	}
	for _, m := range in.Members {
		student, e := s.identity(ctx, m.StudentID, "MAHASISWA")
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `
			INSERT INTO capstone_service.capstone_group_members(group_id,student_id,student_name,nim,is_leader,responsibility)
			VALUES($1,$2,$3,$4,$5,$6)`, id, student.ID, student.FullName, student.NIM, m.IsLeader, m.Responsibility); e != nil {
			return e
		}
	}
	if e = audit(ctx, tx, actor(c), "", "GROUP_SAVED", in); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	g, e := loadGroup(ctx, s.DB, id)
	if e != nil {
		return e
	}
	code := 200
	if create {
		code = 201
	}
	return c.Status(code).JSON(fiber.Map{"group": g})
}

func (s *Server) createTopicFromGroup(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Kelompok belum tersedia. Hubungi administrator.")
	}
	groupID := c.Params("id")
	if !domain.ValidID(groupID) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Title             string               `json:"title"`
		Summary           string               `json:"summary"`
		DescriptionFileID *string              `json:"description_file_id"`
		SignedDescription bool                 `json:"signed_description"`
		Candidates        []domain.MemberInput `json:"members"`
	}
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrUnprocessableEntity
	}
	g, e := loadGroup(c.Context(), s.DB, groupID)
	if e != nil {
		return e
	}
	if g.ProjectID != nil {
		return fiber.NewError(409, "Kelompok ini sudah memiliki pengajuan topik capstone.")
	}
	if g.Status != "READY" {
		return fiber.NewError(422, "Lengkapi tanggung jawab anggota dan tetapkan status kelompok Siap terlebih dahulu.")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = g.WorkingTitle
	}
	summary := strings.TrimSpace(in.Summary)
	if summary == "" {
		summary = g.InitialDescription
	}
	candMap := map[string]domain.MemberInput{}
	for _, cm := range in.Candidates {
		candMap[cm.StudentID] = cm
	}
	members := []domain.MemberInput{}
	isMember := false
	for _, gm := range g.Members {
		if gm.StudentID == actor(c).ID {
			isMember = true
		}
		cm := candMap[gm.StudentID]
		members = append(members, domain.MemberInput{
			StudentID:      gm.StudentID,
			IsLeader:       gm.IsLeader,
			Responsibility: gm.Responsibility,
			Candidate1:     cm.Candidate1,
			Candidate2:     cm.Candidate2,
		})
	}
	if !isMember {
		return fiber.ErrForbidden
	}
	pIn := domain.ProjectInput{
		GroupID:           &g.ID,
		PeriodID:          &g.AcademicPeriodID,
		Title:             title,
		Summary:           summary,
		InterestID:        g.InterestID,
		DescriptionFileID: in.DescriptionFileID,
		SignedDescription: in.SignedDescription,
		Members:           members,
	}
	if err := pIn.Validate(actor(c).ID); err != nil {
		return fiber.NewError(422, err.Error())
	}
	return s.persistNewProject(c, pIn)
}
