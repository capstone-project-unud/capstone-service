package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func loadRevisionItems(ctx context.Context, db querier, stage, seminarID string) ([]domain.RevisionItem, error) {
	rows, e := db.Query(ctx, `
		SELECT id,stage,seminar_id,project_id,student_id,examiner_id,examiner_name,note,student_response,response_file_id,status,reviewer_note,reviewed_at,version,updated_at
		FROM capstone_service.seminar_revision_items
		WHERE stage=$1 AND seminar_id=$2
		ORDER BY created_at ASC`, stage, seminarID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []domain.RevisionItem{}
	for rows.Next() {
		var it domain.RevisionItem
		if e = rows.Scan(&it.ID, &it.Stage, &it.SeminarID, &it.ProjectID, &it.StudentID, &it.ExaminerID, &it.ExaminerName, &it.Note, &it.StudentResponse, &it.ResponseFileID, &it.Status, &it.ReviewerNote, &it.ReviewedAt, &it.Version, &it.UpdatedAt); e != nil {
			return nil, e
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Server) saveSemproApproval(c fiber.Ctx) error {
	if e := require(c, "DOSEN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Persetujuan proposal belum tersedia. Hubungi administrator.")
	}
	projectID := c.Params("id")
	if !domain.ValidID(projectID) {
		return fiber.ErrBadRequest
	}
	var in struct {
		StudentID *string `json:"student_id"`
		Status    string  `json:"status"`
		Note      string  `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidOptional(in.StudentID) || (in.Status != "APPROVED" && in.Status != "REVISION") {
		return fiber.NewError(422, "Status persetujuan harus APPROVED atau REVISION.")
	}
	u := actor(c)
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var studentIDs []string
	if in.StudentID != nil {
		if !u.Has("KOORDINATOR_TA") {
			var sup bool
			if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND student_id=$2 AND supervisor_id=$3)`, projectID, *in.StudentID, u.ID).Scan(&sup); e != nil {
				return e
			}
			if !sup {
				return fiber.ErrForbidden
			}
		}
		studentIDs = []string{*in.StudentID}
	} else {
		rows, err := tx.Query(c.Context(), `SELECT DISTINCT student_id FROM capstone_service.project_students WHERE project_id=$1 AND ($2 OR supervisor_id=$3)`, projectID, u.Has("KOORDINATOR_TA"), u.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var sid string
			if err = rows.Scan(&sid); err != nil {
				rows.Close()
				return err
			}
			studentIDs = append(studentIDs, sid)
		}
		rows.Close()
		if !u.Has("KOORDINATOR_TA") && len(studentIDs) == 0 {
			return fiber.ErrForbidden
		}
	}
	for _, sid := range studentIDs {
		_, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.sempro_advisor_approvals(id,project_id,student_id,supervisor_id,supervisor_name,status,note,signed_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,now())
			ON CONFLICT (project_id,supervisor_id,coalesce(student_id,'00000000-0000-0000-0000-000000000000'::uuid))
			DO UPDATE SET status=EXCLUDED.status,note=EXCLUDED.note,signed_at=now(),updated_at=now()`,
			uuid.NewString(), projectID, sid, u.ID, u.FullName, in.Status, strings.TrimSpace(in.Note))
		if e != nil {
			return e
		}
	}
	if e = audit(c.Context(), tx, u, projectID, "SEMPRO_ADVISOR_SIGN", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) createRevisionItem(c fiber.Ctx) error {
	if e := require(c, "DOSEN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Catatan revisi belum tersedia. Hubungi administrator.")
	}
	var in struct {
		Stage     string  `json:"stage"`
		SeminarID string  `json:"seminar_id"`
		ProjectID string  `json:"project_id"`
		StudentID *string `json:"student_id"`
		Note      string  `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || (in.Stage != "SEMPRO" && in.Stage != "FINAL_EXAM") || !domain.ValidID(in.SeminarID) || !domain.ValidID(in.ProjectID) || !domain.ValidOptional(in.StudentID) || !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Tahap, seminar, proyek, dan catatan revisi (minimal 5 karakter) wajib diisi.")
	}
	u := actor(c)
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if !u.Has("KOORDINATOR_TA") {
		var allowed bool
		if in.Stage == "SEMPRO" {
			e = tx.QueryRow(c.Context(), `
				SELECT EXISTS(SELECT 1 FROM capstone_service.proposal_seminars s WHERE s.id=$1 AND s.project_id=$2 AND ($3 IN (s.examiner_1_id, s.examiner_2_id, s.examiner_3_id)))
				    OR EXISTS(SELECT 1 FROM capstone_service.proposal_exams pe JOIN capstone_service.proposal_applications pa ON pa.id=pe.application_id WHERE pe.id=$1 AND pa.project_id=$2 AND pe.panel @> jsonb_build_array(jsonb_build_object('id',$3::text)))
				    OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=$2 AND m.supervisor_id=$3 AND ($4::uuid IS NULL OR m.student_id=$4))`,
				in.SeminarID, in.ProjectID, u.ID, in.StudentID).Scan(&allowed)
		} else {
			e = tx.QueryRow(c.Context(), `
				SELECT EXISTS(SELECT 1 FROM capstone_service.final_exams fe WHERE fe.id=$1 AND fe.project_id=$2 AND ($3 IN (fe.examiner_1_id, fe.examiner_2_id, fe.examiner_3_id)))
				    OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=$2 AND m.supervisor_id=$3 AND ($4::uuid IS NULL OR m.student_id=$4))`,
				in.SeminarID, in.ProjectID, u.ID, in.StudentID).Scan(&allowed)
		}
		if e != nil {
			return e
		}
		if !allowed {
			return fiber.ErrForbidden
		}
	}
	id := uuid.NewString()
	_, e = tx.Exec(c.Context(), `
		INSERT INTO capstone_service.seminar_revision_items(id,stage,seminar_id,project_id,student_id,examiner_id,examiner_name,note,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'OPEN')`,
		id, in.Stage, in.SeminarID, in.ProjectID, in.StudentID, u.ID, u.FullName, strings.TrimSpace(in.Note))
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, u, in.ProjectID, "REVISION_ITEM_ADDED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}

func (s *Server) respondRevisionItem(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Response string  `json:"student_response"`
		FileID   *string `json:"response_file_id"`
		Version  int     `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Response, 5, 2000) || !domain.ValidOptional(in.FileID) {
		return fiber.NewError(422, "Tanggapan perbaikan minimal 5 karakter wajib diisi.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.seminar_revision_items WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	var member bool
	if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND student_id=$2)`, projectID, actor(c).ID).Scan(&member); e != nil {
		return e
	}
	if !member {
		return fiber.ErrForbidden
	}
	if in.FileID != nil {
		if e = s.ownedFile(c.Context(), tx, *in.FileID, actor(c).ID, "application/pdf"); e != nil {
			return e
		}
	}
	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.seminar_revision_items SET student_response=$2,response_file_id=$3,status='SUBMITTED',version=version+1,updated_at=now() WHERE id=$1`,
		c.Params("id"), strings.TrimSpace(in.Response), in.FileID)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "REVISION_ITEM_RESPONDED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) reviewRevisionItem(c fiber.Ctx) error {
	if e := require(c, "DOSEN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Status  string `json:"status"`
		Note    string `json:"reviewer_note"`
		Version int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || (in.Status != "APPROVED" && in.Status != "REVISION") || (in.Status == "REVISION" && !domain.Text(in.Note, 3, 2000)) {
		return fiber.NewError(422, "Keputusan (APPROVED/REVISION) dan catatan perbaikan wajib valid.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID, examinerID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,examiner_id,version FROM capstone_service.seminar_revision_items WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &examinerID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	if examinerID != actor(c).ID && !actor(c).Has("KOORDINATOR_TA") {
		return fiber.ErrForbidden
	}
	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.seminar_revision_items SET status=$2,reviewer_note=$3,reviewed_at=now(),version=version+1,updated_at=now() WHERE id=$1`,
		c.Params("id"), in.Status, strings.TrimSpace(in.Note))
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "REVISION_ITEM_REVIEWED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) attachSemproRevisionApproval(c fiber.Ctx) error {
	return s.attachStageRevisionApproval(c, "proposal_seminars", "SEMPRO_REVISION_ENDORSED")
}

func (s *Server) attachProposalExamRevisionApproval(c fiber.Ctx) error {
	return s.attachStageRevisionApproval(c, "proposal_exams", "PROPOSAL_EXAM_REVISION_ENDORSED")
}

func (s *Server) attachStageRevisionApproval(c fiber.Ctx, table, action string) error {
	if e := require(c, "KOORDINATOR_TA", "MAHASISWA"); e != nil {
		return e
	}
	id := c.Params("id")
	if !domain.ValidID(id) {
		return fiber.ErrBadRequest
	}
	var in struct {
		FileID  string `json:"file_id"`
		Version int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.FileID) {
		return fiber.NewError(422, "Berkas Lembar Pengesahan Revisi PDF wajib valid.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var ver int
	if table == "proposal_exams" {
		e = tx.QueryRow(c.Context(), `SELECT a.project_id,e.version FROM capstone_service.proposal_exams e JOIN capstone_service.proposal_applications a ON a.id=e.application_id WHERE e.id=$1 FOR UPDATE OF e`, id).Scan(&projectID, &ver)
	} else {
		e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.`+table+` WHERE id=$1 FOR UPDATE`, id).Scan(&projectID, &ver)
	}
	if e != nil {
		return e
	}
	if !actor(c).Has("KOORDINATOR_TA") {
		var member bool
		if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND student_id=$2)`, projectID, actor(c).ID).Scan(&member); e != nil {
			return e
		}
		if !member {
			return fiber.ErrForbidden
		}
	}
	if e = s.ownedFile(c.Context(), tx, in.FileID, actor(c).ID, "application/pdf"); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	stage := "SEMPRO"
	if table == "final_exams" {
		stage = "FINAL_EXAM"
	}
	var openCount int
	if e = tx.QueryRow(c.Context(), `SELECT count(*) FROM capstone_service.seminar_revision_items WHERE stage=$1 AND seminar_id=$2 AND status <> 'APPROVED'`, stage, id).Scan(&openCount); e != nil {
		return e
	}
	if openCount > 0 {
		return fiber.NewError(422, "Seluruh poin revisi penguji harus disetujui sebelum mengesahkan lembar pengesahan revisi.")
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.`+table+` SET revision_approved_file_id=$2,version=version+1,updated_at=now() WHERE id=$1`, id, in.FileID); e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, action, in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
