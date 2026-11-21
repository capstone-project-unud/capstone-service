package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) verifyTopicAdmin(c fiber.Ctx) error {
	if e := require(c, "TU", "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Verifikasi topik belum tersedia. Hubungi administrator.")
	}
	var in struct {
		Version int    `json:"version"`
		Status  string `json:"status"`
		Note    string `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || (in.Status != "VALID" && in.Status != "REVISION") || (in.Status == "REVISION" && !domain.Text(in.Note, 5, 2000)) {
		return fiber.NewError(422, "Status verifikasi (VALID/REVISION) dan catatan revisi wajib valid.")
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.projects SET topic_verify_status=$2,topic_verify_note=$3,topic_verified_by=$4,topic_verified_at=now() WHERE id=$1`,
		p.ID, in.Status, strings.TrimSpace(in.Note), actor(c).ID); e != nil {
		return e
	}
	return s.finish(c, tx, p, "TOPIC_ADMIN_VERIFIED", in)
}

func (s *Server) saveAssignmentLetter(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA", "ADMIN", "TU"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Surat tugas belum tersedia. Hubungi administrator.")
	}
	var in struct {
		Version    int    `json:"version"`
		FileID     string `json:"file_id"`
		Number     string `json:"number"`
		IssuedDate string `json:"issued_date"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.FileID) || !domain.Text(in.Number, 3, 100) {
		return fiber.NewError(422, "Nomor Surat Tugas, tanggal terbit, dan berkas PDF wajib diisi.")
	}
	issued, err := time.Parse("2006-01-02", strings.TrimSpace(in.IssuedDate))
	if err != nil {
		return fiber.NewError(422, "Format tanggal terbit YYYY-MM-DD tidak valid.")
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if p.Status != "FINALIZED" {
		return fiber.NewError(409, "Surat Tugas hanya dapat dilampirkan setelah pembimbing ditetapkan.")
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.projects SET assignment_letter_file_id=$2,assignment_letter_number=$3,assignment_letter_issued_at=$4 WHERE id=$1`,
		p.ID, in.FileID, strings.TrimSpace(in.Number), issued.Format("2006-01-02")); e != nil {
		return e
	}
	return s.finish(c, tx, p, "ASSIGNMENT_LETTER_ATTACHED", in)
}

func loadTopicSeminars(ctx context.Context, db querier, u domain.User, projectID string) ([]domain.TopicSeminar, error) {
	rows, e := db.Query(ctx, `
		SELECT ts.id,ts.project_id,p.title,i.name,ts.scheduled_at,ts.location,ts.panelist_1_id,ts.panelist_1_name,ts.panelist_2_id,ts.panelist_2_name,
		       ts.status,ts.feasibility_notes,ts.decision_note,ts.decided_at,ts.version,ts.updated_at
		FROM capstone_service.topic_seminars ts
		JOIN capstone_service.projects p ON p.id=ts.project_id
		JOIN capstone_service.interest_fields i ON i.id=p.interest_id
		WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=ts.project_id AND (m.student_id=$2 OR m.candidate_id=$2 OR m.supervisor_id=$2)) OR ts.panelist_1_id=$2 OR ts.panelist_2_id=$2)
		  AND ($3='' OR ts.project_id::text=$3)
		ORDER BY ts.updated_at DESC LIMIT 200`, u.Has("ADMIN", "KOORDINATOR_TA", "TU"), u.ID, projectID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []domain.TopicSeminar{}
	for rows.Next() {
		var it domain.TopicSeminar
		if e = rows.Scan(&it.ID, &it.ProjectID, &it.ProjectTitle, &it.InterestName, &it.ScheduledAt, &it.Location, &it.Panelist1ID, &it.Panelist1Name, &it.Panelist2ID, &it.Panelist2Name, &it.Status, &it.FeasibilityNotes, &it.DecisionNote, &it.DecidedAt, &it.Version, &it.UpdatedAt); e != nil {
			return nil, e
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Server) listTopicSeminars(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.TopicSeminar{}})
	}
	items, e := loadTopicSeminars(c.Context(), s.DB, actor(c), c.Query("project_id"))
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) registerTopicSeminar(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Seminar topik belum tersedia. Hubungi administrator.")
	}
	var in struct {
		ProjectID string `json:"project_id"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.ProjectID) {
		return fiber.ErrUnprocessableEntity
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	p, e := loadProject(c.Context(), tx, in.ProjectID, true)
	if e != nil {
		return e
	}
	if !canRead(actor(c), p) {
		return fiber.ErrForbidden
	}
	if p.Status != "SUBMITTED" && p.Status != "FINALIZED" {
		return fiber.NewError(422, "Ajukan usulan topik beserta dokumen bertanda tangan sebelum mendaftar Seminar Kelayakan Topik.")
	}
	var existingID string
	err := tx.QueryRow(c.Context(), `SELECT id FROM capstone_service.topic_seminars WHERE project_id=$1`, p.ID).Scan(&existingID)
	if err == nil {
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.topic_seminars SET status='REGISTERED',version=version+1,updated_at=now() WHERE id=$1 AND status IN ('REVISI','DITOLAK')`, existingID)
	} else if errors.Is(err, pgx.ErrNoRows) {
		existingID = uuid.NewString()
		_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.topic_seminars(id,project_id,status) VALUES($1,$2,'REGISTERED')`, existingID, p.ID)
	} else {
		return err
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), p.ID, "TOPIC_SEMINAR_REGISTERED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"id": existingID})
}

func (s *Server) scheduleTopicSeminar(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA", "ADMIN", "TU"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		ScheduledAt string `json:"scheduled_at"`
		Location    string `json:"location"`
		Panelist1ID string `json:"panelist_1_id"`
		Panelist2ID string `json:"panelist_2_id"`
		Version     int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Location, 3, 250) || !domain.ValidID(in.Panelist1ID) || !domain.ValidID(in.Panelist2ID) || in.Panelist1ID == in.Panelist2ID {
		return fiber.NewError(422, "Jadwal, lokasi, dan dua dosen tim bidang minat berbeda wajib diisi.")
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(in.ScheduledAt))
	if err != nil {
		return fiber.NewError(422, "Format waktu seminar topik tidak valid.")
	}
	p1, e := s.identity(c.Context(), in.Panelist1ID, "DOSEN")
	if e != nil {
		return e
	}
	p2, e := s.identity(c.Context(), in.Panelist2ID, "DOSEN")
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.topic_seminars WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.topic_seminars SET scheduled_at=$2,location=$3,panelist_1_id=$4,panelist_1_name=$5,panelist_2_id=$6,panelist_2_name=$7,status='SCHEDULED',scheduled_by=$8,version=version+1,updated_at=now() WHERE id=$1`,
		c.Params("id"), t, strings.TrimSpace(in.Location), p1.ID, p1.FullName, p2.ID, p2.FullName, actor(c).ID); e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "TOPIC_SEMINAR_SCHEDULED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) evaluateTopicSeminar(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Notes   string `json:"feasibility_notes"`
		Version int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Notes, 5, 3000) {
		return fiber.NewError(422, "Catatan hasil telaah kelayakan topik minimal 5 karakter.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var p1, p2 *string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,panelist_1_id,panelist_2_id,version FROM capstone_service.topic_seminars WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &p1, &p2, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	u := actor(c)
	if (p1 == nil || *p1 != u.ID) && (p2 == nil || *p2 != u.ID) {
		return fiber.ErrForbidden
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.topic_seminars SET feasibility_notes=$2,version=version+1,updated_at=now() WHERE id=$1`, c.Params("id"), strings.TrimSpace(in.Notes)); e != nil {
		return e
	}
	if e = audit(c.Context(), tx, u, projectID, "TOPIC_SEMINAR_EVALUATED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) decideTopicSeminar(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Status  string `json:"status"`
		Note    string `json:"decision_note"`
		Version int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || (in.Status != "LAYAK" && in.Status != "REVISI" && in.Status != "DITOLAK") || !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Keputusan resmi (LAYAK/REVISI/DITOLAK) dan catatan resmi wajib diisi.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.topic_seminars WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.topic_seminars SET status=$2,decision_note=$3,decided_by=$4,decided_at=now(),version=version+1,updated_at=now() WHERE id=$1`,
		c.Params("id"), in.Status, strings.TrimSpace(in.Note), actor(c).ID); e != nil {
		return e
	}
	if in.Status == "REVISI" {
		if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.projects SET status='REVISION',revision_note=$2,version=version+1,updated_at=now() WHERE id=$1 AND status='SUBMITTED'`, projectID, in.Note); e != nil {
			return e
		}
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "TOPIC_SEMINAR_DECIDED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
