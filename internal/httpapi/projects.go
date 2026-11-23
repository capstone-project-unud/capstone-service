package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
)

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadProject(ctx context.Context, db querier, id string, lock bool) (domain.Project, error) {
	var p domain.Project
	p.Students = []domain.Participant{}
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF p"
	}
	full, e := hasFullSOPSchema(ctx, db)
	if e != nil {
		return p, e
	}
	if full {
		e = db.QueryRow(ctx, `
			SELECT p.id,p.group_id,p.academic_period_id,coalesce(ap.name,''),p.owner_id,p.title,p.summary,p.interest_id,i.name,
			       p.description_file_id,p.signed_description,p.status,p.revision_note,p.topic_verify_status,p.topic_verify_note,
			       p.topic_verified_at,p.assignment_letter_file_id,p.assignment_letter_number,
			       CASE WHEN p.assignment_letter_issued_at IS NULL THEN NULL ELSE p.assignment_letter_issued_at::text END,
			       p.version,p.finalized_at,p.updated_at
			FROM capstone_service.projects p
			JOIN capstone_service.interest_fields i ON i.id=p.interest_id
			LEFT JOIN capstone_service.academic_periods ap ON ap.id=p.academic_period_id
			WHERE p.id=$1`+suffix, id).Scan(
			&p.ID, &p.GroupID, &p.PeriodID, &p.PeriodName, &p.OwnerID, &p.Title, &p.Summary, &p.InterestID, &p.InterestName,
			&p.DescriptionFileID, &p.SignedDescription, &p.Status, &p.RevisionNote, &p.TopicVerifyStatus, &p.TopicVerifyNote,
			&p.TopicVerifiedAt, &p.AssignmentLetterFileID, &p.AssignmentLetterNumber, &p.AssignmentLetterIssuedAt,
			&p.Version, &p.FinalizedAt, &p.UpdatedAt,
		)
	} else {
		p.TopicVerifyStatus = "PENDING"
		e = db.QueryRow(ctx, `SELECT p.id,p.owner_id,p.title,p.summary,p.interest_id,i.name,p.description_file_id,p.signed_description,p.status,p.revision_note,p.version,p.finalized_at,p.updated_at FROM capstone_service.projects p JOIN capstone_service.interest_fields i ON i.id=p.interest_id WHERE p.id=$1`+suffix, id).Scan(&p.ID, &p.OwnerID, &p.Title, &p.Summary, &p.InterestID, &p.InterestName, &p.DescriptionFileID, &p.SignedDescription, &p.Status, &p.RevisionNote, &p.Version, &p.FinalizedAt, &p.UpdatedAt)
	}
	if e != nil {
		return p, e
	}
	studentCols := `student_id,student_name,nim,slot,candidate_id,candidate_name,supervisor_id,supervisor_name,title_status,confirmation_status,note,title_actor,title_at,confirmation_actor,confirmation_at`
	if full {
		studentCols = `student_id,student_name,nim,is_leader,responsibility,slot,candidate_id,candidate_name,supervisor_id,supervisor_name,is_external_supervisor,external_supervisor_name,external_supervisor_institution,title_status,confirmation_status,note,title_actor,title_at,confirmation_actor,confirmation_at`
	}
	rows, e := db.Query(ctx, `SELECT `+studentCols+` FROM capstone_service.project_students WHERE project_id=$1 ORDER BY nim,slot`, id)
	if e != nil {
		return p, e
	}
	defer rows.Close()
	for rows.Next() {
		var r domain.Participant
		if full {
			e = rows.Scan(&r.StudentID, &r.StudentName, &r.NIM, &r.IsLeader, &r.Responsibility, &r.Slot, &r.CandidateID, &r.CandidateName, &r.SupervisorID, &r.SupervisorName, &r.IsExternalSupervisor, &r.ExternalSupervisorName, &r.ExternalSupervisorInstitution, &r.TitleStatus, &r.ConfirmationStatus, &r.Note, &r.TitleActor, &r.TitleAt, &r.ConfirmationActor, &r.ConfirmationAt)
		} else {
			e = rows.Scan(&r.StudentID, &r.StudentName, &r.NIM, &r.Slot, &r.CandidateID, &r.CandidateName, &r.SupervisorID, &r.SupervisorName, &r.TitleStatus, &r.ConfirmationStatus, &r.Note, &r.TitleActor, &r.TitleAt, &r.ConfirmationActor, &r.ConfirmationAt)
			r.IsLeader = r.StudentID == p.OwnerID
		}
		if e != nil {
			return p, e
		}
		p.Students = append(p.Students, r)
	}
	return p, rows.Err()
}
func canRead(u domain.User, p domain.Project) bool {
	if u.Staff() {
		return true
	}
	for _, m := range p.Students {
		if m.StudentID == u.ID {
			return true
		}
	}
	return false
}
func (s *Server) project(c fiber.Ctx) (domain.Project, error) {
	if !domain.ValidID(c.Params("id")) {
		return domain.Project{}, fiber.ErrBadRequest
	}
	p, e := loadProject(c.Context(), s.DB, c.Params("id"), false)
	if e == nil && !canRead(actor(c), p) {
		return p, fiber.ErrForbidden
	}
	return p, e
}
func (s *Server) getProject(c fiber.Ctx) error {
	p, e := s.project(c)
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"project": p})
}
func (s *Server) projectList(c fiber.Ctx, all bool) ([]domain.Project, int, error) {
	u := actor(c)
	q, interest, status := strings.TrimSpace(c.Query("q")), c.Query("interest_id"), c.Query("status")
	if len(q) > 150 || (interest != "" && !domain.ValidID(interest)) || (status != "" && status != "DRAFT" && status != "SUBMITTED" && status != "REVISION" && status != "FINALIZED") {
		return nil, 0, fiber.ErrBadRequest
	}
	where := ` FROM capstone_service.projects p WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND m.student_id=$2)) AND ($3='' OR p.title ILIKE '%'||$3||'%') AND ($4='' OR p.interest_id::text=$4) AND ($5='' OR p.status=$5)`
	args := []any{u.Staff(), u.ID, q, interest, status}
	var total int
	if e := s.DB.QueryRow(c.Context(), "SELECT count(*)"+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	limit := 20
	offset := 0
	if all {
		limit = 10000
		if total > limit {
			return nil, 0, fiber.NewError(422, "Data melebihi 10000 judul. Gunakan filter bidang minat.")
		}
	} else {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		if page < 1 {
			page = 1
		}
		if page > 100000 {
			page = 100000
		}
		offset = (page - 1) * 20
	}
	args = append(args, limit, offset)
	rows, e := s.DB.Query(c.Context(), "SELECT p.id"+where+" ORDER BY p.updated_at DESC,p.id LIMIT $6 OFFSET $7", args...)
	if e != nil {
		return nil, 0, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, 0, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, 0, e
	}
	items := []domain.Project{}
	for _, id := range ids {
		p, e := loadProject(c.Context(), s.DB, id, false)
		if e != nil {
			return nil, 0, e
		}
		items = append(items, p)
	}
	return items, total, nil
}
func (s *Server) listProjects(c fiber.Ctx) error {
	items, total, e := s.projectList(c, false)
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items, "total": total, "limit": 20})
}
func (s *Server) createProject(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	var in domain.ProjectInput
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrUnprocessableEntity
	}
	if e := in.Validate(actor(c).ID); e != nil {
		return fiber.NewError(422, e.Error())
	}
	return s.persistNewProject(c, in)
}
func (s *Server) updateProject(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	var in domain.ProjectInput
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrUnprocessableEntity
	}
	if e := in.Validate(actor(c).ID); e != nil {
		return fiber.NewError(422, e.Error())
	}
	return s.persistProject(c, false, in)
}
func (s *Server) persistNewProject(c fiber.Ctx, in domain.ProjectInput) error {
	return s.persistProject(c, true, in)
}
func (s *Server) persistProject(c fiber.Ctx, create bool, in domain.ProjectInput) error {
	id := c.Params("id")
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
	full, e := hasFullSOPSchema(ctx, tx)
	if e != nil {
		return e
	}
	if !create {
		p, e := loadProject(ctx, tx, id, true)
		if e != nil {
			return e
		}
		if p.OwnerID != actor(c).ID {
			return fiber.ErrForbidden
		}
		if e = stale(p.Version, in.Version); e != nil {
			return e
		}
		if p.Status != "DRAFT" && p.Status != "REVISION" {
			return fiber.NewError(409, "Judul sedang ditinjau atau sudah final. Koordinator perlu meminta revisi sebelum diubah.")
		}
		if p.PeriodID != nil && in.PeriodID == nil {
			in.PeriodID = p.PeriodID
		}
	}
	var active bool
	if e = tx.QueryRow(ctx, "SELECT is_active FROM capstone_service.interest_fields WHERE id=$1", in.InterestID).Scan(&active); e != nil {
		return e
	}
	if !active {
		return fiber.NewError(422, "Bidang minat tidak aktif.")
	}
	if in.DescriptionFileID != nil {
		if e = s.ownedFile(ctx, tx, *in.DescriptionFileID, actor(c).ID, "application/pdf"); e != nil {
			return e
		}
	}
	var resolvedGroupID *string
	var resolvedPeriodID *string
	if full {
		resolvedPeriodID = in.PeriodID
		if resolvedPeriodID == nil {
			var pid string
			if err := tx.QueryRow(ctx, `SELECT id FROM capstone_service.academic_periods ORDER BY is_active DESC, start_date DESC LIMIT 1`).Scan(&pid); err == nil {
				resolvedPeriodID = &pid
			}
		}
		if create {
			if in.GroupID != nil {
				resolvedGroupID = in.GroupID
				if _, e = tx.Exec(ctx, `UPDATE capstone_service.capstone_groups SET status='LOCKED',version=version+1,updated_at=now() WHERE id=$1`, *resolvedGroupID); e != nil {
					return e
				}
			} else if resolvedPeriodID != nil {
				gid := uuid.NewString()
				resolvedGroupID = &gid
				if _, e = tx.Exec(ctx, `INSERT INTO capstone_service.capstone_groups(id,code,academic_period_id,interest_id,working_title,initial_description,leader_id,status) VALUES($1,$2,$3,$4,$5,$6,$7,'LOCKED')`,
					gid, generateGroupCode(), *resolvedPeriodID, in.InterestID, in.Title, in.Summary, actor(c).ID); e != nil {
					return e
				}
			}
		}
	}
	if create {
		if full {
			_, e = tx.Exec(ctx, `INSERT INTO capstone_service.projects(id,group_id,academic_period_id,owner_id,title,summary,interest_id,description_file_id,signed_description) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				id, resolvedGroupID, resolvedPeriodID, actor(c).ID, in.Title, in.Summary, in.InterestID, in.DescriptionFileID, in.SignedDescription)
		} else {
			_, e = tx.Exec(ctx, `INSERT INTO capstone_service.projects(id,owner_id,title,summary,interest_id,description_file_id,signed_description) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, actor(c).ID, in.Title, in.Summary, in.InterestID, in.DescriptionFileID, in.SignedDescription)
		}
	} else {
		_, e = tx.Exec(ctx, `UPDATE capstone_service.projects SET title=$2,summary=$3,interest_id=$4,description_file_id=$5,signed_description=$6,version=version+1,updated_at=now(),status='DRAFT' WHERE id=$1`, id, in.Title, in.Summary, in.InterestID, in.DescriptionFileID, in.SignedDescription)
		if e == nil {
			_, e = tx.Exec(ctx, "DELETE FROM capstone_service.project_students WHERE project_id=$1", id)
		}
	}
	if e != nil {
		return e
	}
	hasExplicitLeader := false
	for _, m := range in.Members {
		if m.IsLeader {
			hasExplicitLeader = true
			break
		}
	}
	for _, m := range in.Members {
		student, e := s.identity(ctx, m.StudentID, "MAHASISWA")
		if e != nil {
			return e
		}
		isLeader := m.IsLeader
		if !hasExplicitLeader {
			isLeader = m.StudentID == actor(c).ID
		}
		resp := strings.TrimSpace(m.Responsibility)
		if resp == "" {
			if isLeader {
				resp = "Ketua Kelompok"
			} else {
				resp = "Anggota Tim"
			}
		}
		if full && create && in.GroupID == nil && resolvedGroupID != nil {
			if _, e = tx.Exec(ctx, `INSERT INTO capstone_service.capstone_group_members(group_id,student_id,student_name,nim,is_leader,responsibility) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (group_id,student_id) DO NOTHING`,
				*resolvedGroupID, student.ID, student.FullName, student.NIM, isLeader, resp); e != nil {
				return e
			}
		}
		for i, candidate := range []*string{m.Candidate1, m.Candidate2} {
			name := ""
			if candidate != nil {
				u, e := s.identity(ctx, *candidate, "DOSEN")
				if e != nil {
					return e
				}
				name = u.FullName
			}
			if full {
				extFlag := m.IsExternalSupervisor || m.ExternalSupervisorName != ""
				extName := ""
				extInst := ""
				if i == 1 && extFlag {
					extName = m.ExternalSupervisorName
					extInst = m.ExternalSupervisorInstitution
				}
				_, e = tx.Exec(ctx, `INSERT INTO capstone_service.project_students(project_id,academic_period_id,student_id,student_name,nim,is_leader,responsibility,slot,candidate_id,candidate_name,is_external_supervisor,external_supervisor_name,external_supervisor_institution) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
					id, resolvedPeriodID, student.ID, student.FullName, student.NIM, isLeader, resp, i+1, candidate, name, i == 1 && extFlag, extName, extInst)
			} else {
				_, e = tx.Exec(ctx, `INSERT INTO capstone_service.project_students(project_id,student_id,student_name,nim,slot,candidate_id,candidate_name) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, student.ID, student.FullName, student.NIM, i+1, candidate, name)
			}
			if e != nil {
				return e
			}
		}
	}
	if e = audit(ctx, tx, actor(c), id, "PROJECT_SAVED", in); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	p, e := loadProject(ctx, s.DB, id, false)
	if e != nil {
		return e
	}
	code := 200
	if create {
		code = 201
	}
	return c.Status(code).JSON(fiber.Map{"project": p})
}
func (s *Server) lock(c fiber.Ctx, version int) (pgx.Tx, domain.Project, error) {
	if !domain.ValidID(c.Params("id")) {
		return nil, domain.Project{}, fiber.ErrBadRequest
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return nil, domain.Project{}, e
	}
	p, e := loadProject(c.Context(), tx, c.Params("id"), true)
	if e == nil {
		e = stale(p.Version, version)
	}
	if e != nil {
		tx.Rollback(c.Context())
		return nil, p, e
	}
	return tx, p, nil
}
func (s *Server) finish(c fiber.Ctx, tx pgx.Tx, p domain.Project, action string, detail any) error {
	if _, e := tx.Exec(c.Context(), "UPDATE capstone_service.projects SET version=version+1,updated_at=now() WHERE id=$1", p.ID); e != nil {
		return e
	}
	if e := audit(c.Context(), tx, actor(c), p.ID, action, detail); e != nil {
		return e
	}
	if e := tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
func (s *Server) submitProject(c fiber.Ctx) error {
	var in struct {
		Version int `json:"version"`
	}
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrUnprocessableEntity
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if p.OwnerID != actor(c).ID || !actor(c).Has("MAHASISWA") {
		return fiber.ErrForbidden
	}
	if p.Status != "DRAFT" && p.Status != "REVISION" {
		return fiber.ErrConflict
	}
	if p.DescriptionFileID == nil || !p.SignedDescription {
		return fiber.NewError(422, "Unggah deskripsi PDF yang telah mendapat pengesahan calon pembimbing.")
	}
	_, e = tx.Exec(c.Context(), "UPDATE capstone_service.projects SET status='SUBMITTED',revision_note='' WHERE id=$1", p.ID)
	if e != nil {
		return e
	}
	return s.finish(c, tx, p, "PROJECT_SUBMITTED", nil)
}
func (s *Server) requestRevision(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	var in struct {
		Version int    `json:"version"`
		Note    string `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Catatan revisi 5–2000 karakter wajib diisi.")
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if p.Status != "SUBMITTED" {
		return fiber.ErrConflict
	}
	_, e = tx.Exec(c.Context(), "UPDATE capstone_service.projects SET status='REVISION',revision_note=$2 WHERE id=$1", p.ID, in.Note)
	if e != nil {
		return e
	}
	return s.finish(c, tx, p, "REVISION_REQUESTED", in)
}
func (s *Server) updateCandidates(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	var in struct {
		Version    int     `json:"version"`
		Candidate1 *string `json:"candidate_1"`
		Candidate2 *string `json:"candidate_2"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidOptional(in.Candidate1) || !domain.ValidOptional(in.Candidate2) {
		return fiber.ErrUnprocessableEntity
	}
	if in.Candidate1 != nil && in.Candidate2 != nil && *in.Candidate1 == *in.Candidate2 {
		return fiber.NewError(422, "Calon pembimbing harus berbeda.")
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if !canRead(actor(c), p) {
		return fiber.ErrForbidden
	}
	if p.Status == "FINALIZED" {
		return fiber.NewError(409, "Pembimbing sudah ditetapkan.")
	}
	for i, id := range []*string{in.Candidate1, in.Candidate2} {
		name := ""
		if id != nil {
			u, e := s.identity(c.Context(), *id, "DOSEN")
			if e != nil {
				return e
			}
			name = u.FullName
		}
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.project_students SET candidate_id=$4,candidate_name=$5,supervisor_id=NULL,supervisor_name='',title_status='PENDING',confirmation_status='PENDING',title_actor=NULL,title_at=NULL,confirmation_actor=NULL,confirmation_at=NULL,note='' WHERE project_id=$1 AND student_id=$2 AND slot=$3 AND candidate_id IS DISTINCT FROM $4`, p.ID, actor(c).ID, i+1, id, name)
		if e != nil {
			return e
		}
	}
	return s.finish(c, tx, p, "CANDIDATES_CHANGED", in)
}
func (s *Server) decision(c fiber.Ctx) error {
	if e := require(c, "DOSEN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	var in struct {
		Version   int    `json:"version"`
		StudentID string `json:"student_id"`
		Slot      int    `json:"slot"`
		Kind      string `json:"kind"`
		Status    string `json:"status"`
		Note      string `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.StudentID) || (in.Slot != 1 && in.Slot != 2) || (in.Kind != "title" && in.Kind != "confirmation") || (in.Status != "APPROVED" && in.Status != "REJECTED") || !domain.Text(in.Note, 0, 2000) {
		return fiber.ErrUnprocessableEntity
	}
	if in.Status == "REJECTED" && !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Catatan penolakan wajib diisi.")
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if p.Status != "SUBMITTED" {
		return fiber.NewError(409, "Judul belum diajukan atau sudah final.")
	}
	var target *domain.Participant
	for i := range p.Students {
		r := &p.Students[i]
		if r.StudentID == in.StudentID && r.Slot == in.Slot {
			target = r
			break
		}
	}
	if target == nil {
		return fiber.ErrNotFound
	}
	if target.CandidateID == nil && target.SupervisorID == nil {
		return fiber.NewError(422, "Pilih calon pembimbing terlebih dahulu.")
	}
	expected := target.CandidateID
	if target.SupervisorID != nil {
		expected = target.SupervisorID
	}
	if !actor(c).Has("KOORDINATOR_TA") && (expected == nil || *expected != actor(c).ID) {
		return fiber.ErrForbidden
	}
	column := "title"
	if in.Kind == "confirmation" {
		column = "confirmation"
	}
	_, e = tx.Exec(c.Context(), fmt.Sprintf("UPDATE capstone_service.project_students SET %s_status=$4,%s_actor=$5,%s_at=now(),note=$6 WHERE project_id=$1 AND student_id=$2 AND slot=$3", column, column, column), p.ID, in.StudentID, in.Slot, in.Status, actor(c).ID, in.Note)
	if e != nil {
		return e
	}
	return s.finish(c, tx, p, "SUPERVISOR_DECISION", in)
}
func (s *Server) assignment(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	var in struct {
		Version      int     `json:"version"`
		StudentID    string  `json:"student_id"`
		Slot         int     `json:"slot"`
		SupervisorID *string `json:"supervisor_id"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.StudentID) || (in.Slot != 1 && in.Slot != 2) || !domain.ValidOptional(in.SupervisorID) {
		return fiber.ErrUnprocessableEntity
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if p.Status != "SUBMITTED" {
		return fiber.ErrConflict
	}
	name := ""
	if in.SupervisorID != nil {
		u, e := s.identity(c.Context(), *in.SupervisorID, "DOSEN")
		if e != nil {
			return e
		}
		name = u.FullName
	}
	found := false
	for _, r := range p.Students {
		if r.StudentID == in.StudentID {
			if r.Slot == in.Slot {
				found = true
			}
			if r.Slot != in.Slot && r.SupervisorID != nil && in.SupervisorID != nil && *r.SupervisorID == *in.SupervisorID {
				return fiber.NewError(422, "Pembimbing 1 dan 2 harus berbeda.")
			}
		}
	}
	if !found {
		return fiber.ErrNotFound
	}
	// Preserve lecturer decisions only when the assigned person is the same candidate.
	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.project_students SET supervisor_id=$4,supervisor_name=$5,
 title_status=CASE WHEN coalesce(supervisor_id,candidate_id) IS NOT DISTINCT FROM $4 THEN title_status ELSE 'PENDING' END,
 confirmation_status=CASE WHEN coalesce(supervisor_id,candidate_id) IS NOT DISTINCT FROM $4 THEN confirmation_status ELSE 'PENDING' END,
 title_actor=CASE WHEN coalesce(supervisor_id,candidate_id) IS NOT DISTINCT FROM $4 THEN title_actor END,
 title_at=CASE WHEN coalesce(supervisor_id,candidate_id) IS NOT DISTINCT FROM $4 THEN title_at END,
 confirmation_actor=CASE WHEN coalesce(supervisor_id,candidate_id) IS NOT DISTINCT FROM $4 THEN confirmation_actor END,
 confirmation_at=CASE WHEN coalesce(supervisor_id,candidate_id) IS NOT DISTINCT FROM $4 THEN confirmation_at END
 WHERE project_id=$1 AND student_id=$2 AND slot=$3`, p.ID, in.StudentID, in.Slot, in.SupervisorID, name)
	if e != nil {
		return e
	}
	return s.finish(c, tx, p, "SUPERVISOR_ASSIGNED", in)
}
func (s *Server) finalize(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	var in struct {
		Version int `json:"version"`
	}
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrUnprocessableEntity
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if e = domain.ReadyToFinalize(p); e != nil {
		return fiber.NewError(422, e.Error())
	}
	_, e = tx.Exec(c.Context(), "UPDATE capstone_service.projects SET status='FINALIZED',finalized_at=now(),finalized_by=$2 WHERE id=$1", p.ID, actor(c).ID)
	if e != nil {
		return e
	}
	return s.finish(c, tx, p, "PROJECT_FINALIZED", p)
}
func (s *Server) history(c fiber.Ctx) error {
	p, e := s.project(c)
	if e != nil {
		return e
	}
	rows, e := s.DB.Query(c.Context(), "SELECT id,actor_id,action,created_at FROM capstone_service.audit_logs WHERE project_id=$1 ORDER BY id DESC LIMIT 100", p.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var uid, action string
		var at any
		if e = rows.Scan(&id, &uid, &action, &at); e != nil {
			return e
		}
		items = append(items, map[string]any{"id": id, "actor_id": uid, "action": action, "created_at": at})
	}
	if e = rows.Err(); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items})
}
