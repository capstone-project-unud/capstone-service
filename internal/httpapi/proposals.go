package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type Proposal struct {
	ID                 string     `json:"id"`
	ProjectID          string     `json:"project_id"`
	Title              string     `json:"title"`
	StudentID          string     `json:"student_id"`
	StudentName        string     `json:"student_name"`
	NIM                string     `json:"nim"`
	SimakFileID        string     `json:"simak_file_id"`
	ApprovalFileID     string     `json:"approval_file_id"`
	SimakStatus        string     `json:"simak_status"`
	ApprovalStatus     string     `json:"approval_status"`
	SimakNote          string     `json:"simak_note"`
	ApprovalNote       string     `json:"approval_note"`
	SimakReviewer      *string    `json:"simak_reviewer"`
	ApprovalReviewer   *string    `json:"approval_reviewer"`
	SimakReviewedAt    *time.Time `json:"simak_reviewed_at"`
	ApprovalReviewedAt *time.Time `json:"approval_reviewed_at"`
	Version            int        `json:"version"`
	SubmittedAt        time.Time  `json:"submitted_at"`
	Ready              bool       `json:"ready"`
}

func (s *Server) listProposals(c fiber.Ctx) error {
	u := actor(c)
	if !u.Has("MAHASISWA", "TU", "ADMIN", "KOORDINATOR_TA") {
		return fiber.ErrForbidden
	}
	rows, e := s.DB.Query(c.Context(), `SELECT a.id,a.project_id,p.title,a.student_id,m.student_name,m.nim,a.simak_file_id,a.approval_file_id,a.simak_status,a.approval_status,a.simak_note,a.approval_note,a.simak_reviewer,a.approval_reviewer,a.simak_reviewed_at,a.approval_reviewed_at,a.version,a.submitted_at FROM capstone_service.proposal_applications a JOIN capstone_service.projects p ON p.id=a.project_id JOIN capstone_service.project_students m ON m.project_id=a.project_id AND m.student_id=a.student_id AND m.slot=1 WHERE ($1 OR a.student_id=$2) ORDER BY a.submitted_at DESC LIMIT 1001`, u.Has("TU", "ADMIN", "KOORDINATOR_TA"), u.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []Proposal{}
	for rows.Next() {
		var a Proposal
		if e = rows.Scan(&a.ID, &a.ProjectID, &a.Title, &a.StudentID, &a.StudentName, &a.NIM, &a.SimakFileID, &a.ApprovalFileID, &a.SimakStatus, &a.ApprovalStatus, &a.SimakNote, &a.ApprovalNote, &a.SimakReviewer, &a.ApprovalReviewer, &a.SimakReviewedAt, &a.ApprovalReviewedAt, &a.Version, &a.SubmittedAt); e != nil {
			return e
		}
		a.Ready = a.SimakStatus == "VALID" && a.ApprovalStatus == "VALID"
		items = append(items, a)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	if len(items) > 1000 {
		return fiber.NewError(422, "Daftar melebihi 1000 pendaftaran.")
	}
	return c.JSON(fiber.Map{"items": items})
}
func (s *Server) saveProposal(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	var in struct {
		ProjectID      string `json:"project_id"`
		SimakFileID    string `json:"simak_file_id"`
		ApprovalFileID string `json:"approval_file_id"`
		Version        int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.ProjectID) || !domain.ValidID(in.SimakFileID) || !domain.ValidID(in.ApprovalFileID) || in.SimakFileID == in.ApprovalFileID {
		return fiber.NewError(422, "Unggah bukti SIMAK dan surat persetujuan sebagai dua berkas berbeda.")
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
	if !canRead(actor(c), p) || p.Status != "FINALIZED" {
		return fiber.NewError(422, "Judul Anda belum ditetapkan.")
	}
	supervisors := 0
	for _, r := range p.Students {
		if r.StudentID == actor(c).ID && r.SupervisorID != nil {
			supervisors++
		}
	}
	if supervisors != 2 {
		return fiber.NewError(422, "Dua pembimbing perlu ditetapkan sebelum mendaftar UP.")
	}
	for _, id := range []string{in.SimakFileID, in.ApprovalFileID} {
		if e = s.ownedFile(c.Context(), tx, id, actor(c).ID, ""); e != nil {
			return e
		}
	}
	if e = s.ownedFile(c.Context(), tx, in.ApprovalFileID, actor(c).ID, "application/pdf"); e != nil {
		return e
	}
	id := uuid.NewString()
	var existing Proposal
	e = tx.QueryRow(c.Context(), "SELECT id,version,simak_status,approval_status,simak_file_id,approval_file_id FROM capstone_service.proposal_applications WHERE student_id=$1 FOR UPDATE", actor(c).ID).Scan(&existing.ID, &existing.Version, &existing.SimakStatus, &existing.ApprovalStatus, &existing.SimakFileID, &existing.ApprovalFileID)
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	if e == nil {
		if e = stale(existing.Version, in.Version); e != nil {
			return e
		}
		if existing.SimakStatus == "VALID" && existing.ApprovalStatus == "VALID" {
			return fiber.NewError(409, "Kedua berkas sudah divalidasi TU.")
		}
		id = existing.ID
		if existing.SimakFileID == in.SimakFileID && existing.ApprovalFileID == in.ApprovalFileID {
			return fiber.NewError(422, "Unggah perbaikan berkas sebelum mengajukan ulang.")
		}
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_applications SET simak_file_id=$2,approval_file_id=$3,
 simak_status=CASE WHEN simak_file_id=$2 THEN simak_status ELSE 'PENDING' END,approval_status=CASE WHEN approval_file_id=$3 THEN approval_status ELSE 'PENDING' END,
 simak_note=CASE WHEN simak_file_id=$2 THEN simak_note ELSE '' END,approval_note=CASE WHEN approval_file_id=$3 THEN approval_note ELSE '' END,
 simak_reviewer=CASE WHEN simak_file_id=$2 THEN simak_reviewer END,approval_reviewer=CASE WHEN approval_file_id=$3 THEN approval_reviewer END,
 simak_reviewed_at=CASE WHEN simak_file_id=$2 THEN simak_reviewed_at END,approval_reviewed_at=CASE WHEN approval_file_id=$3 THEN approval_reviewed_at END,
 version=version+1,submitted_at=now() WHERE id=$1`, id, in.SimakFileID, in.ApprovalFileID)
	} else {
		_, e = tx.Exec(c.Context(), "INSERT INTO capstone_service.proposal_applications(id,project_id,student_id,simak_file_id,approval_file_id) VALUES($1,$2,$3,$4,$5)", id, p.ID, actor(c).ID, in.SimakFileID, in.ApprovalFileID)
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), p.ID, "PROPOSAL_SUBMITTED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}
func (s *Server) reviewProposal(c fiber.Ctx) error {
	if e := require(c, "TU"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
		Status  string `json:"status"`
		Note    string `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || (in.Kind != "simak" && in.Kind != "approval") || (in.Status != "VALID" && in.Status != "REVISION") || !domain.Text(in.Note, 0, 2000) || (in.Status == "REVISION" && !domain.Text(in.Note, 5, 2000)) {
		return fiber.NewError(422, "Keputusan dan catatan pemeriksaan tidak valid.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var project string
	var version int
	if e = tx.QueryRow(c.Context(), "SELECT project_id,version FROM capstone_service.proposal_applications WHERE id=$1 FOR UPDATE", c.Params("id")).Scan(&project, &version); e != nil {
		return e
	}
	if e = stale(version, in.Version); e != nil {
		return e
	}
	_, e = tx.Exec(c.Context(), fmt.Sprintf("UPDATE capstone_service.proposal_applications SET %s_status=$2,%s_note=$3,%s_reviewer=$4,%s_reviewed_at=now(),version=version+1 WHERE id=$1", in.Kind, in.Kind, in.Kind, in.Kind), c.Params("id"), in.Status, in.Note, actor(c).ID)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), project, "PROPOSAL_FILE_REVIEWED", map[string]any{"id": c.Params("id"), "input": in}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
