package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/reports"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type finalExamItem struct {
	ProjectID    string                        `json:"project_id"`
	Title        string                        `json:"title"`
	InterestName string                        `json:"interest_name"`
	Students     []domain.Participant          `json:"students"`
	Readiness    domain.FinalExamReadiness     `json:"readiness"`
	Applications []domain.FinalExamApplication `json:"applications"`
	Exam         *domain.FinalExam             `json:"exam"`
}

func computeFinalExamReadiness(ctx context.Context, db querier, p domain.Project) (domain.FinalExamReadiness, error) {
	var r domain.FinalExamReadiness
	studentSet := map[string]bool{}
	for _, m := range p.Students {
		studentSet[m.StudentID] = true
	}
	r.TotalStudents = len(studentSet)

	_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capstone_service.proposal_seminars WHERE project_id=$1 AND status='APPROVED')`, p.ID).Scan(&r.SemproPassed)
	if revisionSchema(ctx, db) == nil {
		var passed, recorded int
		err := db.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE (SELECT count(*) FROM capstone_service.proposal_exam_evaluations v WHERE v.exam_id=e.id AND v.recommendation='APPROVED')=5) FROM capstone_service.proposal_exams e JOIN capstone_service.proposal_applications a ON a.id=e.application_id WHERE a.project_id=$1`, p.ID).Scan(&recorded, &passed)
		if err != nil {
			return r, err
		}
		if recorded > 0 {
			r.SemproPassed = passed == r.TotalStudents
		}
	}
	var revPending int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM capstone_service.seminar_revision_items WHERE project_id=$1 AND stage='SEMPRO' AND status <> 'APPROVED'`, p.ID).Scan(&revPending)
	r.SemproRevApproved = r.SemproPassed && revPending == 0

	var bCount int
	_ = db.QueryRow(ctx, `SELECT count(DISTINCT coalesce(doc_type_code,series)) FROM capstone_service.project_documents WHERE project_id=$1 AND coalesce(doc_type_code,series) IN ('B-100','B-200','B-300','B-400')`, p.ID).Scan(&bCount)
	r.BSeriesComplete = bCount == 4

	_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capstone_service.project_documents WHERE project_id=$1 AND doc_type_code='LAPORAN_AKHIR')`, p.ID).Scan(&r.FinalReportReady)
	_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capstone_service.project_documents WHERE project_id=$1 AND doc_type_code='BUKTI_PLAGIARISME')`, p.ID).Scan(&r.PlagiarismReady)
	_ = db.QueryRow(ctx, `SELECT count(*) FROM capstone_service.guidance_logs WHERE project_id=$1 AND status='APPROVED'`, p.ID).Scan(&r.GuidanceCount)

	if p.AssignmentLetterIssuedAt != nil {
		if dt, err := time.Parse("2006-01-02", *p.AssignmentLetterIssuedAt); err == nil {
			r.AssignmentValid = time.Since(dt) < 365*24*time.Hour
		}
	}
	var hasPersetujuanUA bool
	_ = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capstone_service.project_documents WHERE project_id=$1 AND doc_type_code='PERSETUJUAN_UA' AND status='APPROVED')`, p.ID).Scan(&hasPersetujuanUA)
	_ = db.QueryRow(ctx, `SELECT count(*) FROM capstone_service.final_exam_applications WHERE project_id=$1 AND tu_status='VALID' AND ((advisor_1_approved AND advisor_2_approved) OR $2) AND sks_completed >= 140 AND min_grade_ok AND plagiarism_pct < 20`, p.ID, hasPersetujuanUA).Scan(&r.StudentsVerified)

	r.Ready = r.SemproRevApproved && r.BSeriesComplete && r.GuidanceCount >= 1 && r.StudentsVerified >= r.TotalStudents && r.TotalStudents > 0
	return r, nil
}

func loadFinalExamForProject(ctx context.Context, db querier, projectID string, participants []domain.Participant) (*domain.FinalExam, error) {
	var fe domain.FinalExam
	var rawHist []byte
	e := db.QueryRow(ctx, `
		SELECT id,project_id,scheduled_at,location,examiner_1_id,examiner_1_name,examiner_2_id,examiner_2_name,examiner_3_id,examiner_3_name,
		       status,outcome_note,minutes_file_id,revision_approved_file_id,schedule_history,decided_at,version
		FROM capstone_service.final_exams WHERE project_id=$1`, projectID).Scan(
		&fe.ID, &fe.ProjectID, &fe.ScheduledAt, &fe.Location, &fe.Examiner1ID, &fe.Examiner1Name, &fe.Examiner2ID, &fe.Examiner2Name, &fe.Examiner3ID, &fe.Examiner3Name,
		&fe.Status, &fe.OutcomeNote, &fe.MinutesFileID, &fe.RevisionApprovedFileID, &rawHist, &fe.DecidedAt, &fe.Version,
	)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	fe.ScheduleHistory = []map[string]any{}
	if len(rawHist) > 0 {
		_ = json.Unmarshal(rawHist, &fe.ScheduleHistory)
	}
	fe.Evaluations = []domain.SemproEvaluation{}
	rows, e := db.Query(ctx, `
		SELECT id,exam_id,student_id,student_name,nim,evaluator_id,evaluator_name,evaluator_role,scores,total_score,grade_label,recommendation,notes,version,updated_at
		FROM capstone_service.final_exam_evaluations WHERE exam_id=$1 ORDER BY student_name, evaluator_role, evaluator_name`, fe.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var ev domain.SemproEvaluation
		var raw []byte
		if e = rows.Scan(&ev.ID, &ev.SeminarID, &ev.StudentID, &ev.StudentName, &ev.NIM, &ev.EvaluatorID, &ev.EvaluatorName, &ev.EvaluatorRole, &raw, &ev.TotalScore, &ev.GradeLabel, &ev.Recommendation, &ev.Notes, &ev.Version, &ev.UpdatedAt); e != nil {
			return nil, e
		}
		ev.Scores = map[string]float64{}
		_ = json.Unmarshal(raw, &ev.Scores)
		fe.Evaluations = append(fe.Evaluations, ev)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	revs, e := loadRevisionItems(ctx, db, "FINAL_EXAM", fe.ID)
	if e != nil {
		return nil, e
	}
	fe.Attendance = []domain.FinalExamAttendance{}
	attRows, e := db.Query(ctx, `SELECT lecturer_id,confirmed_at FROM capstone_service.final_exam_attendance WHERE exam_id=$1 ORDER BY lecturer_id`, fe.ID)
	if e == nil {
		defer attRows.Close()
		for attRows.Next() {
			var att domain.FinalExamAttendance
			if scanErr := attRows.Scan(&att.LecturerID, &att.ConfirmedAt); scanErr != nil {
				return nil, scanErr
			}
			fe.Attendance = append(fe.Attendance, att)
		}
		if attErr := attRows.Err(); attErr != nil {
			return nil, attErr
		}
	} else if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return nil, e
	}
	fe.RevisionItems = revs
	fe.StudentSummaries = summarizeStudentScores(participants, fe.Evaluations)
	return &fe, nil
}

func (s *Server) listFinalExam(c fiber.Ctx) error {
	u := actor(c)
	if !u.Has("MAHASISWA", "DOSEN", "KOORDINATOR_TA", "TU", "ADMIN") {
		return fiber.ErrForbidden
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []finalExamItem{}})
	}
	global := u.Has("ADMIN", "KOORDINATOR_TA", "TU")
	projectFilter := c.Query("project_id")
	rows, e := s.DB.Query(c.Context(), `
		SELECT p.id FROM capstone_service.projects p
		WHERE p.status='FINALIZED'
		  AND ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND (m.student_id=$2 OR m.supervisor_id=$2))
		          OR EXISTS(SELECT 1 FROM capstone_service.final_exams fe WHERE fe.project_id=p.id AND (fe.examiner_1_id=$2 OR fe.examiner_2_id=$2 OR fe.examiner_3_id=$2)))
		  AND ($3='' OR p.id::text=$3)
		ORDER BY p.updated_at DESC`, global, u.ID, projectFilter)
	if e != nil {
		return e
	}
	var ids []string
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
	items := []finalExamItem{}
	for _, pid := range ids {
		p, e := loadProject(c.Context(), s.DB, pid, false)
		if e != nil {
			return e
		}
		readiness, e := computeFinalExamReadiness(c.Context(), s.DB, p)
		if e != nil {
			return e
		}
		apps := []domain.FinalExamApplication{}
		aRows, e := s.DB.Query(c.Context(), `
			SELECT id,project_id,student_id,student_name,nim,sks_completed,min_grade_ok,transcript_file_id,spp_file_id,biodata_summary,ijazah_file_id,photo_file_id,pa_letter_file_id,
			       plagiarism_pct,plagiarism_file_id,advisor_1_approved,advisor_2_approved,advisor_note,advisor_approved_at,tu_status,tu_note,tu_reviewed_at,coordinator_status,coordinator_note,version,updated_at
			FROM capstone_service.final_exam_applications WHERE project_id=$1 ORDER BY nim`, pid)
		if e != nil {
			return e
		}
		for aRows.Next() {
			var a domain.FinalExamApplication
			a.ProjectTitle = p.Title
			if e = aRows.Scan(&a.ID, &a.ProjectID, &a.StudentID, &a.StudentName, &a.NIM, &a.SKSCompleted, &a.MinGradeOK, &a.TranscriptFileID, &a.SPPFileID, &a.BiodataSummary, &a.IjazahFileID, &a.PhotoFileID, &a.PALetterFileID, &a.PlagiarismPct, &a.PlagiarismFileID, &a.Advisor1Approved, &a.Advisor2Approved, &a.AdvisorNote, &a.AdvisorApprovedAt, &a.TUStatus, &a.TUNote, &a.TUReviewedAt, &a.CoordinatorStatus, &a.CoordinatorNote, &a.Version, &a.UpdatedAt); e != nil {
				aRows.Close()
				return e
			}
			apps = append(apps, a)
		}
		aRows.Close()
		fe, e := loadFinalExamForProject(c.Context(), s.DB, pid, p.Students)
		if e != nil {
			return e
		}
		items = append(items, finalExamItem{ProjectID: p.ID, Title: p.Title, InterestName: p.InterestName, Students: p.Students, Readiness: readiness, Applications: apps, Exam: fe})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) saveFinalExamApp(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Ujian akhir belum tersedia. Hubungi administrator.")
	}
	var in struct {
		ProjectID        string  `json:"project_id"`
		SKSCompleted     int     `json:"sks_completed"`
		MinGradeOK       bool    `json:"min_grade_ok"`
		TranscriptFileID *string `json:"transcript_file_id"`
		SPPFileID        *string `json:"spp_file_id"`
		BiodataSummary   string  `json:"biodata_summary"`
		IjazahFileID     *string `json:"ijazah_file_id"`
		PhotoFileID      *string `json:"photo_file_id"`
		PALetterFileID   *string `json:"pa_letter_file_id"`
		PlagiarismPct    float64 `json:"plagiarism_pct"`
		PlagiarismFileID *string `json:"plagiarism_file_id"`
		Version          int     `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.ProjectID) || in.SKSCompleted < 140 || !in.MinGradeOK || in.PlagiarismPct < 0 || in.PlagiarismPct >= 20 {
		return fiber.NewError(422, "Pendaftaran Ujian Akhir mewajibkan >= 140 SKS (nilai minimal C) dan similarity plagiarisme < 20%.")
	}
	for _, f := range []*string{in.TranscriptFileID, in.SPPFileID, in.IjazahFileID, in.PhotoFileID, in.PALetterFileID, in.PlagiarismFileID} {
		if f == nil || !domain.ValidID(*f) {
			return fiber.NewError(422, "Unggah lengkap 6 berkas individu syarat Ujian Akhir (Transkrip, SPP, Ijazah, Foto 4x6, Surat PA, dan Bukti Plagiarisme).")
		}
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
	u := actor(c)
	if !canRead(u, p) || p.Status != "FINALIZED" {
		return fiber.ErrForbidden
	}
	for _, f := range []*string{in.TranscriptFileID, in.SPPFileID, in.IjazahFileID, in.PhotoFileID, in.PALetterFileID, in.PlagiarismFileID} {
		if e = s.ownedFile(c.Context(), tx, *f, u.ID, ""); e != nil {
			return e
		}
	}
	var id string
	var existingVer int
	err := tx.QueryRow(c.Context(), `SELECT id,version FROM capstone_service.final_exam_applications WHERE project_id=$1 AND student_id=$2 FOR UPDATE`, p.ID, u.ID).Scan(&id, &existingVer)
	if errors.Is(err, pgx.ErrNoRows) {
		id = uuid.NewString()
		_, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.final_exam_applications(id,project_id,student_id,student_name,nim,sks_completed,min_grade_ok,transcript_file_id,spp_file_id,biodata_summary,ijazah_file_id,photo_file_id,pa_letter_file_id,plagiarism_pct,plagiarism_file_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			id, p.ID, u.ID, u.FullName, u.NIM, in.SKSCompleted, in.MinGradeOK, in.TranscriptFileID, in.SPPFileID, strings.TrimSpace(in.BiodataSummary), in.IjazahFileID, in.PhotoFileID, in.PALetterFileID, in.PlagiarismPct, in.PlagiarismFileID)
	} else if err == nil {
		if e = stale(existingVer, in.Version); e != nil {
			return e
		}
		_, e = tx.Exec(c.Context(), `
			UPDATE capstone_service.final_exam_applications
			SET sks_completed=$2,min_grade_ok=$3,transcript_file_id=$4,spp_file_id=$5,biodata_summary=$6,ijazah_file_id=$7,photo_file_id=$8,pa_letter_file_id=$9,
			    plagiarism_pct=$10,plagiarism_file_id=$11,tu_status='PENDING',coordinator_status='PENDING',version=version+1,updated_at=now()
			WHERE id=$1`,
			id, in.SKSCompleted, in.MinGradeOK, in.TranscriptFileID, in.SPPFileID, strings.TrimSpace(in.BiodataSummary), in.IjazahFileID, in.PhotoFileID, in.PALetterFileID, in.PlagiarismPct, in.PlagiarismFileID)
	} else {
		return err
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, u, p.ID, "FINAL_EXAM_APPLIED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}

func (s *Server) reviewFinalExamAppTU(c fiber.Ctx) error {
	if e := require(c, "TU", "ADMIN"); e != nil {
		return e
	}
	return s.updateFinalExamAppStatus(c, "tu")
}

func (s *Server) reviewFinalExamAppCoordinator(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	return s.updateFinalExamAppStatus(c, "coordinator")
}

func (s *Server) updateFinalExamAppStatus(c fiber.Ctx, kind string) error {
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Status  string `json:"status"`
		Note    string `json:"note"`
		Version int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrBadRequest
	}
	if kind == "tu" && in.Status != "VALID" && in.Status != "REVISION" {
		return fiber.ErrUnprocessableEntity
	}
	if kind == "coordinator" && in.Status != "READY" && in.Status != "REVISION" {
		return fiber.ErrUnprocessableEntity
	}
	if in.Status == "REVISION" && !domain.Text(in.Note, 3, 2000) {
		return fiber.NewError(422, "Catatan perbaikan wajib diisi minimal 3 karakter.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.final_exam_applications WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	if kind == "tu" {
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.final_exam_applications SET tu_status=$2,tu_note=$3,tu_reviewer=$4,tu_reviewed_at=now(),version=version+1,updated_at=now() WHERE id=$1`, c.Params("id"), in.Status, strings.TrimSpace(in.Note), actor(c).ID)
	} else {
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.final_exam_applications SET coordinator_status=$2,coordinator_note=$3,version=version+1,updated_at=now() WHERE id=$1`, c.Params("id"), in.Status, strings.TrimSpace(in.Note))
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "FINAL_EXAM_APP_REVIEWED", map[string]any{"kind": kind, "status": in.Status}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) reviewFinalExamAppAdvisor(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Approved bool   `json:"approved"`
		Note     string `json:"note"`
		Version  int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrBadRequest
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID, studentID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,student_id,version FROM capstone_service.final_exam_applications WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &studentID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	var slot int
	err := tx.QueryRow(c.Context(), `SELECT slot FROM capstone_service.project_students WHERE project_id=$1 AND student_id=$2 AND supervisor_id=$3 LIMIT 1`, projectID, studentID, actor(c).ID).Scan(&slot)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.ErrForbidden
	}
	if err != nil {
		return err
	}
	col := "advisor_1_approved"
	if slot == 2 {
		col = "advisor_2_approved"
	}
	_, e = tx.Exec(c.Context(), fmt.Sprintf(`UPDATE capstone_service.final_exam_applications SET %s=$2,advisor_note=$3,advisor_approved_at=now(),version=version+1,updated_at=now() WHERE id=$1`, col),
		c.Params("id"), in.Approved, strings.TrimSpace(in.Note))
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "FINAL_EXAM_ADVISOR_SIGN", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) scheduleFinalExam(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Ujian akhir belum tersedia. Hubungi administrator.")
	}
	var in domain.SemproScheduleInput
	if c.Bind().Body(&in) != nil {
		return fiber.ErrBadRequest
	}
	scheduledAt, e := in.Validate()
	if e != nil {
		return fiber.NewError(422, e.Error())
	}
	ex1, e := s.identity(c.Context(), in.Examiner1ID, "DOSEN")
	if e != nil {
		return e
	}
	ex2, e := s.identity(c.Context(), in.Examiner2ID, "DOSEN")
	if e != nil {
		return e
	}
	ex3, e := s.identity(c.Context(), in.Examiner3ID, "DOSEN")
	if e != nil {
		return e
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
	readiness, e := computeFinalExamReadiness(c.Context(), tx, p)
	if e != nil {
		return e
	}
	if !readiness.Ready {
		return fiber.NewError(422, "Persyaratan Ujian Akhir (revisi Sempro tuntas, B-100..B-400, 140 SKS, dan persetujuan pembimbing/TU) belum lengkap.")
	}
	conflict, e := scheduleConflict(c.Context(), tx, "final_exams", p.ID, scheduledAt, in.Location, []string{ex1.ID, ex2.ID, ex3.ID})
	if e != nil {
		return e
	}
	if conflict != "" {
		return fiber.NewError(409, fmt.Sprintf("Terjadi benturan jadwal ruang atau penguji dengan kelompok %q.", conflict))
	}
	var id, status string
	var ver int
	e = tx.QueryRow(c.Context(), `SELECT id,status,version FROM capstone_service.final_exams WHERE project_id=$1 FOR UPDATE`, p.ID).Scan(&id, &status, &ver)
	if errors.Is(e, pgx.ErrNoRows) {
		id = uuid.NewString()
		_, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.final_exams(id,project_id,scheduled_at,location,examiner_1_id,examiner_1_name,examiner_2_id,examiner_2_name,examiner_3_id,examiner_3_name,scheduled_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			id, p.ID, scheduledAt, in.Location, ex1.ID, ex1.FullName, ex2.ID, ex2.FullName, ex3.ID, ex3.FullName, actor(c).ID)
	} else if e == nil {
		if status == "PASSED" {
			return fiber.NewError(409, "Ujian Akhir sudah disahkan lulus.")
		}
		if e = stale(ver, in.Version); e != nil {
			return e
		}
		hist, _ := json.Marshal([]map[string]any{{"changed_at": time.Now(), "actor_id": actor(c).ID, "scheduled_at": scheduledAt, "location": in.Location}})
		nextStatus := status
		if status == "FAILED" {
			nextStatus = "SCHEDULED"
		}
		_, e = tx.Exec(c.Context(), `
			UPDATE capstone_service.final_exams SET scheduled_at=$2,location=$3,examiner_1_id=$4,examiner_1_name=$5,examiner_2_id=$6,examiner_2_name=$7,examiner_3_id=$8,examiner_3_name=$9,status=$10,schedule_history=coalesce(schedule_history,'[]'::jsonb)||$11::jsonb,version=version+1,updated_at=now() WHERE id=$1`,
			id, scheduledAt, in.Location, ex1.ID, ex1.FullName, ex2.ID, ex2.FullName, ex3.ID, ex3.FullName, nextStatus, string(hist))
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), p.ID, "FINAL_EXAM_SCHEDULED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}

func (s *Server) evaluateFinalExam(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	examID := c.Params("id")
	if !domain.ValidID(examID) {
		return fiber.ErrBadRequest
	}
	var in struct {
		StudentID        string             `json:"student_id"`
		Scores           map[string]float64 `json:"scores"`
		Recommendation   string             `json:"recommendation"`
		Notes            string             `json:"notes"`
		ConfirmSignature bool               `json:"confirm_signature"`
	}
	if c.Bind().Body(&in) != nil {
		return fiber.ErrBadRequest
	}
	check := domain.FinalExamEvaluateInput{StudentID: in.StudentID, Scores: in.Scores, Recommendation: in.Recommendation, Notes: in.Notes}
	totalScore, gradeLabel, e := check.Validate()
	if e != nil {
		return fiber.NewError(422, e.Error())
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID, ex1, ex2, ex3, status string
	if e = tx.QueryRow(c.Context(), `SELECT project_id,examiner_1_id,examiner_2_id,examiner_3_id,status FROM capstone_service.final_exams WHERE id=$1 FOR UPDATE`, examID).Scan(&projectID, &ex1, &ex2, &ex3, &status); e != nil {
		return e
	}
	if status == "PASSED" {
		return fiber.NewError(409, "Ujian Akhir telah disahkan lulus sehingga nilai terkunci.")
	}
	role := ""
	switch actor(c).ID {
	case ex1:
		role = "EXAMINER_1"
	case ex2:
		role = "EXAMINER_2"
	case ex3:
		role = "EXAMINER_3"
	default:
		var isSup bool
		if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND supervisor_id=$2 AND student_id=$3)`, projectID, actor(c).ID, check.StudentID).Scan(&isSup); e != nil {
			return e
		}
		if !isSup {
			return fiber.ErrForbidden
		}
		role = "SUPERVISOR"
	}
	sigKey := ""
	if in.ConfirmSignature {
		sigKey, e = signatureKey(c.Context(), tx, actor(c).ID)
		if e != nil {
			return e
		}
	}
	var sName, nim string
	if e = tx.QueryRow(c.Context(), `SELECT student_name,nim FROM capstone_service.project_students WHERE project_id=$1 AND student_id=$2 LIMIT 1`, projectID, check.StudentID).Scan(&sName, &nim); e != nil {
		return e
	}
	raw, _ := json.Marshal(check.Scores)
	_, e = tx.Exec(c.Context(), `
		INSERT INTO capstone_service.final_exam_evaluations(id,exam_id,project_id,student_id,student_name,nim,evaluator_id,evaluator_name,evaluator_role,scores,total_score,grade_label,recommendation,notes,signature_key)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (exam_id,student_id,evaluator_id)
		DO UPDATE SET evaluator_role=EXCLUDED.evaluator_role,evaluator_name=EXCLUDED.evaluator_name,scores=EXCLUDED.scores,total_score=EXCLUDED.total_score,grade_label=EXCLUDED.grade_label,recommendation=EXCLUDED.recommendation,notes=EXCLUDED.notes,signature_key=CASE WHEN EXCLUDED.signature_key<>'' THEN EXCLUDED.signature_key ELSE capstone_service.final_exam_evaluations.signature_key END,version=capstone_service.final_exam_evaluations.version+1,updated_at=now()`,
		uuid.NewString(), examID, projectID, check.StudentID, sName, nim, actor(c).ID, actor(c).FullName, role, raw, totalScore, gradeLabel, check.Recommendation, check.Notes, sigKey)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "FINAL_EXAM_EVALUATED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"total_score": totalScore, "grade_label": gradeLabel})
}

func (s *Server) decideFinalExam(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	examID := c.Params("id")
	if !domain.ValidID(examID) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Status  string `json:"status"`
		Note    string `json:"note"`
		Version int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || (in.Status != "PASSED" && in.Status != "REVISION" && in.Status != "FAILED") {
		return fiber.ErrUnprocessableEntity
	}
	if in.Status != "PASSED" && !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Catatan hasil ujian wajib diisi minimal 5 karakter.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.final_exams WHERE id=$1 FOR UPDATE`, examID).Scan(&projectID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	if in.Status == "PASSED" {
		var incomplete bool
		if e = tx.QueryRow(c.Context(), `
			SELECT EXISTS(
				SELECT 1 FROM (SELECT DISTINCT student_id FROM capstone_service.project_students WHERE project_id=$1) st
				WHERE (SELECT count(*) FROM capstone_service.final_exam_evaluations ev WHERE ev.exam_id=$2 AND ev.student_id=st.student_id AND ev.evaluator_role IN ('EXAMINER_1','EXAMINER_2','EXAMINER_3')) < 3
			)`, projectID, examID).Scan(&incomplete); e != nil {
			return e
		}
		if incomplete {
			return fiber.NewError(422, "Seluruh penilaian individu Proyek Akhir II dari ketiga penguji wajib lengkap sebelum kelulusan disahkan.")
		}
	}
	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.final_exams SET status=$2,outcome_note=$3,decided_by=$4,decided_at=now(),version=version+1,updated_at=now() WHERE id=$1`, examID, in.Status, strings.TrimSpace(in.Note), actor(c).ID)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "FINAL_EXAM_DECIDED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) attachFinalExamRevisionApproval(c fiber.Ctx) error {
	return s.attachStageRevisionApproval(c, "final_exams", "FINAL_EXAM_REVISION_ENDORSED")
}

func (s *Server) attendFinalExam(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	examID := c.Params("id")
	if !domain.ValidID(examID) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Confirm bool `json:"confirm_signature"`
	}
	if c.Bind().Body(&in) != nil || !in.Confirm {
		return fiber.NewError(422, "Konfirmasikan penggunaan tanda tangan pada presensi.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID, ex1, ex2, ex3 string
	if e = tx.QueryRow(c.Context(), `SELECT project_id,examiner_1_id,examiner_2_id,examiner_3_id FROM capstone_service.final_exams WHERE id=$1`, examID).Scan(&projectID, &ex1, &ex2, &ex3); e != nil {
		return e
	}
	u := actor(c)
	allowed := u.ID == ex1 || u.ID == ex2 || u.ID == ex3
	if !allowed {
		if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND supervisor_id=$2)`, projectID, u.ID).Scan(&allowed); e != nil {
			return e
		}
	}
	if !allowed {
		return fiber.ErrForbidden
	}
	key, e := signatureKey(c.Context(), tx, u.ID)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.final_exam_attendance(exam_id,lecturer_id,signature_key) VALUES($1,$2,$3) ON CONFLICT(exam_id,lecturer_id) DO UPDATE SET signature_key=excluded.signature_key,confirmed_at=now()`, examID, u.ID, key); e != nil {
		return e
	}
	if e = audit(c.Context(), tx, u, projectID, "FINAL_EXAM_ATTENDED", fiber.Map{"exam_id": examID}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) finalExamReport(c fiber.Ctx) error {
	examID := c.Params("id")
	if !domain.ValidID(examID) {
		return fiber.ErrBadRequest
	}
	var projectID string
	if e := s.DB.QueryRow(c.Context(), `SELECT project_id FROM capstone_service.final_exams WHERE id=$1`, examID).Scan(&projectID); e != nil {
		return e
	}
	p, e := loadProject(c.Context(), s.DB, projectID, false)
	if e != nil {
		return e
	}
	fe, e := loadFinalExamForProject(c.Context(), s.DB, projectID, p.Students)
	if e != nil || fe == nil {
		return fiber.ErrNotFound
	}
	u := actor(c)
	allowed := u.Has("ADMIN", "KOORDINATOR_TA", "TU", "KOPRODI") || fe.Examiner1ID == u.ID || fe.Examiner2ID == u.ID || fe.Examiner3ID == u.ID
	if !allowed {
		for _, m := range p.Students {
			if m.StudentID == u.ID || (m.SupervisorID != nil && *m.SupervisorID == u.ID) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return fiber.ErrForbidden
	}
	kind := c.Query("kind", "minutes")
	wita := time.FixedZone("WITA", 8*60*60)
	if kind == "attendance" {
		type panelRow struct{ ID, Name, Role string }
		signers := []panelRow{
			{fe.Examiner1ID, fe.Examiner1Name, "Penguji 1"},
			{fe.Examiner2ID, fe.Examiner2Name, "Penguji 2"},
			{fe.Examiner3ID, fe.Examiner3Name, "Penguji 3"},
		}
		seen := map[string]bool{fe.Examiner1ID: true, fe.Examiner2ID: true, fe.Examiner3ID: true}
		for _, m := range p.Students {
			if m.SupervisorID != nil && !seen[*m.SupervisorID] {
				seen[*m.SupervisorID] = true
				signers = append(signers, panelRow{*m.SupervisorID, m.SupervisorName, fmt.Sprintf("Pembimbing %d (%s)", m.Slot, m.StudentName)})
			}
		}
		r := reports.ExamReport{
			Kind:      "Presensi Ujian Akhir Capstone",
			Reference: fe.ID,
			Student:   fmt.Sprintf("%d Mahasiswa", len(fe.StudentSummaries)),
			NIM:       p.InterestName,
			Title:     p.Title,
			Date:      fe.ScheduledAt.In(wita).Format("02 January 2006, 15:04"),
			Location:  fe.Location,
			Notes:     "Tanda tangan dicantumkan setelah masing-masing dosen mengonfirmasi kehadiran di Ujian Akhir.",
		}
		for _, sp := range signers {
			sig := reports.ExamSignatory{Name: sp.Name, Role: sp.Role}
			var key string
			var confirmed time.Time
			if err := s.DB.QueryRow(c.Context(), `SELECT signature_key,confirmed_at FROM capstone_service.final_exam_attendance WHERE exam_id=$1 AND lecturer_id=$2`, fe.ID, sp.ID).Scan(&key, &confirmed); err == nil && key != "" {
				if bytesSig, errGet := s.Files.Get(c.Context(), key); errGet == nil {
					sig.Signature = bytesSig
					sig.SignedAt = confirmed.In(wita).Format("02/01/2006 15:04 WITA")
				}
			}
			r.Panel = append(r.Panel, sig)
		}
		b, e := reports.ProposalExamPDF(r)
		if e != nil {
			return e
		}
		c.Set("Content-Type", "application/pdf")
		c.Set("Content-Disposition", `attachment; filename="Presensi_Ujian_Akhir.pdf"`)
		return c.Send(b)
	}
	summaryLines := []string{}
	for _, st := range fe.StudentSummaries {
		summaryLines = append(summaryLines, fmt.Sprintf("%s (%s): %.2f (%s) [%d/3 penguji]", st.StudentName, st.NIM, st.AverageScore, st.GradeLabel, st.ExaminerCount))
	}
	r := reports.ExamReport{
		Kind:      "Berita Acara Ujian Akhir Capstone",
		Reference: fe.ID,
		Student:   fmt.Sprintf("%d Mahasiswa Terdaftar", len(fe.StudentSummaries)),
		NIM:       p.InterestName,
		Title:     p.Title,
		Date:      fe.ScheduledAt.In(wita).Format("02 January 2006, 15:04"),
		Location:  fe.Location,
		Result:    "Status Ujian Akhir: " + fe.Status + "\n" + strings.Join(summaryLines, "\n"),
		Notes:     fe.OutcomeNote,
	}
	type evalPanel struct{ ID, Name, Role string }
	panels := []evalPanel{
		{fe.Examiner1ID, fe.Examiner1Name, "Penguji 1"},
		{fe.Examiner2ID, fe.Examiner2Name, "Penguji 2"},
		{fe.Examiner3ID, fe.Examiner3Name, "Penguji 3"},
	}
	for _, pnl := range panels {
		sig := reports.ExamSignatory{Name: pnl.Name, Role: pnl.Role}
		var key string
		var at time.Time
		if err := s.DB.QueryRow(c.Context(), `SELECT signature_key,updated_at FROM capstone_service.final_exam_evaluations WHERE exam_id=$1 AND evaluator_id=$2 AND signature_key<>'' ORDER BY updated_at DESC LIMIT 1`, fe.ID, pnl.ID).Scan(&key, &at); err == nil {
			if bytesSig, errGet := s.Files.Get(c.Context(), key); errGet == nil {
				sig.Signature = bytesSig
				sig.SignedAt = at.In(wita).Format("02/01/2006 15:04 WITA")
			}
		} else if err := s.DB.QueryRow(c.Context(), `SELECT signature_key,confirmed_at FROM capstone_service.final_exam_attendance WHERE exam_id=$1 AND lecturer_id=$2`, fe.ID, pnl.ID).Scan(&key, &at); err == nil && key != "" {
			if bytesSig, errGet := s.Files.Get(c.Context(), key); errGet == nil {
				sig.Signature = bytesSig
				sig.SignedAt = at.In(wita).Format("02/01/2006 15:04 WITA")
			}
		}
		r.Panel = append(r.Panel, sig)
	}
	b, e := reports.ProposalExamPDF(r)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="Berita_Acara_Ujian_Akhir.pdf"`)
	return c.Send(b)
}
