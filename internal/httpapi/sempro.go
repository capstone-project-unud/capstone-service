package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/reports"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type semproItem struct {
	ProjectID    string                  `json:"project_id"`
	Title        string                  `json:"title"`
	InterestName string                  `json:"interest_name"`
	Students     []domain.Participant    `json:"students"`
	Readiness    domain.SemproReadiness  `json:"readiness"`
	Seminar      *domain.ProposalSeminar `json:"seminar"`
}

func computeSemproReadiness(ctx context.Context, db querier, p domain.Project, hasSchema bool) (domain.SemproReadiness, error) {
	r := domain.SemproReadiness{
		Finalized:            p.Status == "FINALIZED",
		B100Status:           "MISSING",
		ProposalUsulanStatus: "MISSING",
	}
	if !r.Finalized {
		return r, nil
	}
	if e := db.QueryRow(ctx, `SELECT count(*) FROM capstone_service.guidance_logs WHERE project_id=$1 AND status='APPROVED'`, p.ID).Scan(&r.GuidanceApprovedCount); e != nil {
		return r, e
	}
	if e := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capstone_service.proposal_applications WHERE project_id=$1 AND simak_status='VALID' AND approval_status='VALID')`, p.ID).Scan(&r.ProposalReady); e != nil {
		return r, e
	}
	hasFull, e := hasFullSOPSchema(ctx, db)
	if e != nil {
		return r, e
	}
	if hasSchema {
		var b100 string
		if e := db.QueryRow(ctx, `SELECT status FROM capstone_service.project_documents WHERE project_id=$1 AND coalesce(doc_type_code,series)='B-100' LIMIT 1`, p.ID).Scan(&b100); e == nil {
			r.B100Status = b100
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return r, e
		}
		if hasFull {
			var propDoc string
			if e := db.QueryRow(ctx, `SELECT status FROM capstone_service.project_documents WHERE project_id=$1 AND doc_type_code='PROPOSAL_USULAN' LIMIT 1`, p.ID).Scan(&propDoc); e == nil {
				r.ProposalUsulanStatus = propDoc
			} else if !errors.Is(e, pgx.ErrNoRows) {
				return r, e
			}
			_ = db.QueryRow(ctx, `SELECT count(*) FROM capstone_service.sempro_advisor_approvals WHERE project_id=$1 AND status='APPROVED'`, p.ID).Scan(&r.AdvisorApprovalCount)
		}
	}
	r.Ready = r.Finalized && r.GuidanceApprovedCount >= 1 && r.ProposalReady && r.B100Status == "APPROVED"
	return r, nil
}

func loadSeminarForProject(ctx context.Context, db querier, projectID string, participants []domain.Participant) (*domain.ProposalSeminar, error) {
	hasFull, e := hasFullSOPSchema(ctx, db)
	if e != nil {
		return nil, e
	}
	var sem domain.ProposalSeminar
	sem.AdvisorApprovals = []domain.SemproAdvisorApproval{}
	sem.RevisionItems = []domain.RevisionItem{}
	sem.ScheduleHistory = []map[string]any{}
	if hasFull {
		var rawHist []byte
		e = db.QueryRow(ctx, `
			SELECT id,project_id,scheduled_at,location,examiner_1_id,examiner_1_name,examiner_2_id,examiner_2_name,examiner_3_id,examiner_3_name,
			       status,outcome_note,revision_file_id,revision_note,revision_submitted_at,minutes_file_id,revision_approved_file_id,schedule_history,decided_at,version
			FROM capstone_service.proposal_seminars
			WHERE project_id=$1`, projectID).Scan(
			&sem.ID, &sem.ProjectID, &sem.ScheduledAt, &sem.Location,
			&sem.Examiner1ID, &sem.Examiner1Name,
			&sem.Examiner2ID, &sem.Examiner2Name,
			&sem.Examiner3ID, &sem.Examiner3Name,
			&sem.Status, &sem.OutcomeNote, &sem.RevisionFileID, &sem.RevisionNote, &sem.RevisionSubmittedAt,
			&sem.MinutesFileID, &sem.RevisionApprovedFileID, &rawHist, &sem.DecidedAt, &sem.Version,
		)
		if len(rawHist) > 0 {
			_ = json.Unmarshal(rawHist, &sem.ScheduleHistory)
		}
	} else {
		e = db.QueryRow(ctx, `
			SELECT id,project_id,scheduled_at,location,examiner_1_id,examiner_1_name,examiner_2_id,examiner_2_name,examiner_3_id,examiner_3_name,status,outcome_note,revision_file_id,revision_note,revision_submitted_at,decided_at,version
			FROM capstone_service.proposal_seminars
			WHERE project_id=$1`, projectID).Scan(
			&sem.ID, &sem.ProjectID, &sem.ScheduledAt, &sem.Location,
			&sem.Examiner1ID, &sem.Examiner1Name,
			&sem.Examiner2ID, &sem.Examiner2Name,
			&sem.Examiner3ID, &sem.Examiner3Name,
			&sem.Status, &sem.OutcomeNote, &sem.RevisionFileID, &sem.RevisionNote, &sem.RevisionSubmittedAt, &sem.DecidedAt, &sem.Version,
		)
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}

	sem.Evaluations = []domain.SemproEvaluation{}
	rows, e := db.Query(ctx, `
		SELECT id,seminar_id,student_id,student_name,nim,evaluator_id,evaluator_name,evaluator_role,scores,total_score,grade_label,recommendation,notes,version,updated_at
		FROM capstone_service.seminar_evaluations
		WHERE seminar_id=$1
		ORDER BY student_name, evaluator_role, evaluator_name`, sem.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var ev domain.SemproEvaluation
		var rawScores []byte
		if e = rows.Scan(&ev.ID, &ev.SeminarID, &ev.StudentID, &ev.StudentName, &ev.NIM, &ev.EvaluatorID, &ev.EvaluatorName, &ev.EvaluatorRole, &rawScores, &ev.TotalScore, &ev.GradeLabel, &ev.Recommendation, &ev.Notes, &ev.Version, &ev.UpdatedAt); e != nil {
			return nil, e
		}
		ev.Scores = map[string]float64{}
		_ = json.Unmarshal(rawScores, &ev.Scores)
		sem.Evaluations = append(sem.Evaluations, ev)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}

	if hasFull {
		apRows, e := db.Query(ctx, `SELECT supervisor_id,supervisor_name,status,note,signed_at FROM capstone_service.sempro_advisor_approvals WHERE project_id=$1 ORDER BY supervisor_name`, projectID)
		if e != nil {
			return nil, e
		}
		for apRows.Next() {
			var ap domain.SemproAdvisorApproval
			if e = apRows.Scan(&ap.SupervisorID, &ap.SupervisorName, &ap.Status, &ap.Note, &ap.SignedAt); e != nil {
				apRows.Close()
				return nil, e
			}
			sem.AdvisorApprovals = append(sem.AdvisorApprovals, ap)
		}
		apRows.Close()

		revItems, e := loadRevisionItems(ctx, db, "SEMPRO", sem.ID)
		if e != nil {
			return nil, e
		}
		sem.RevisionItems = revItems
	}

	sem.StudentSummaries = summarizeStudentScores(participants, sem.Evaluations)
	return &sem, nil
}

func summarizeStudentScores(participants []domain.Participant, evals []domain.SemproEvaluation) []domain.StudentScoreBrief {
	type agg struct {
		name     string
		nim      string
		examSum  float64
		examCnt  int
		allSum   float64
		allCnt   int
		supCnt   int
	}
	byStudent := map[string]*agg{}
	for _, p := range participants {
		if _, ok := byStudent[p.StudentID]; !ok {
			byStudent[p.StudentID] = &agg{name: p.StudentName, nim: p.NIM}
		}
	}
	for _, ev := range evals {
		a, ok := byStudent[ev.StudentID]
		if !ok {
			a = &agg{name: ev.StudentName, nim: ev.NIM}
			byStudent[ev.StudentID] = a
		}
		a.allSum += ev.TotalScore
		a.allCnt++
		if ev.EvaluatorRole == "SUPERVISOR" {
			a.supCnt++
		} else {
			a.examSum += ev.TotalScore
			a.examCnt++
		}
	}
	out := make([]domain.StudentScoreBrief, 0, len(byStudent))
	for id, a := range byStudent {
		avg := 0.0
		if a.examCnt > 0 {
			avg = math.Round((a.examSum/float64(a.examCnt))*100) / 100
		} else if a.allCnt > 0 {
			avg = math.Round((a.allSum/float64(a.allCnt))*100) / 100
		}
		grade := "BELUM DINILAI"
		if a.allCnt > 0 {
			grade = domain.GradeCategory(avg)
		}
		out = append(out, domain.StudentScoreBrief{
			StudentID:       id,
			StudentName:     a.name,
			NIM:             a.nim,
			ExaminerCount:   a.examCnt,
			SupervisorCount: a.supCnt,
			AverageScore:    avg,
			GradeLabel:      grade,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NIM < out[j].NIM })
	return out
}

func (s *Server) listSempro(c fiber.Ctx) error {
	u := actor(c)
	if !u.Has("MAHASISWA", "DOSEN", "KOORDINATOR_TA", "TU", "ADMIN") {
		return fiber.ErrForbidden
	}
	projectFilter := c.Query("project_id")
	if projectFilter != "" && !domain.ValidID(projectFilter) {
		return fiber.ErrBadRequest
	}
	global := u.Has("ADMIN", "KOORDINATOR_TA", "TU")
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	query := `
		SELECT p.id
		FROM capstone_service.projects p
		WHERE p.status='FINALIZED'
		  AND ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND (m.student_id=$2 OR m.supervisor_id=$2)))
		  AND ($3='' OR p.id::text=$3)
		ORDER BY p.updated_at DESC, p.title`
	if hasSchema {
		query = `
		SELECT p.id
		FROM capstone_service.projects p
		WHERE p.status='FINALIZED'
		  AND ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND (m.student_id=$2 OR m.supervisor_id=$2)) OR EXISTS(SELECT 1 FROM capstone_service.proposal_seminars s WHERE s.project_id=p.id AND (s.examiner_1_id=$2 OR s.examiner_2_id=$2 OR s.examiner_3_id=$2)))
		  AND ($3='' OR p.id::text=$3)
		ORDER BY p.updated_at DESC, p.title`
	}
	rows, e := s.DB.Query(c.Context(), query, global, u.ID, projectFilter)
	if e != nil {
		return e
	}
	defer rows.Close()
	var projectIDs []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return e
		}
		projectIDs = append(projectIDs, id)
	}
	if e = rows.Err(); e != nil {
		return e
	}

	items := make([]semproItem, 0, len(projectIDs))
	for _, pid := range projectIDs {
		p, e := loadProject(c.Context(), s.DB, pid, false)
		if e != nil {
			return e
		}
		r, e := computeSemproReadiness(c.Context(), s.DB, p, hasSchema)
		if e != nil {
			return e
		}
		var sem *domain.ProposalSeminar
		if hasSchema {
			sem, e = loadSeminarForProject(c.Context(), s.DB, pid, p.Students)
			if e != nil {
				return e
			}
		}
		items = append(items, semproItem{
			ProjectID:    p.ID,
			Title:        p.Title,
			InterestName: p.InterestName,
			Students:     p.Students,
			Readiness:    r,
			Seminar:      sem,
		})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) scheduleSempro(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return fiber.NewError(503, "Seminar proposal belum tersedia. Hubungi administrator.")
	}
	var in domain.SemproScheduleInput
	if e := c.Bind().Body(&in); e != nil {
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
	readiness, e := computeSemproReadiness(c.Context(), tx, p, true)
	if e != nil {
		return e
	}
	if !readiness.Finalized {
		return fiber.NewError(422, "Judul capstone harus berstatus Ditetapkan.")
	}
	if readiness.GuidanceApprovedCount < 1 {
		return fiber.NewError(422, "Minimal 1 log bimbingan disetujui pembimbing sebelum menjadwalkan seminar proposal.")
	}
	if !readiness.ProposalReady {
		return fiber.NewError(422, "Berkas persyaratan UP (SIMAK dan surat persetujuan pembimbing) harus valid.")
	}
	if readiness.B100Status != "APPROVED" {
		return fiber.NewError(422, "Dokumen B-100 wajib diunggah dan disetujui pembimbing terlebih dahulu.")
	}

	supervisors := map[string]bool{}
	for _, r := range p.Students {
		if r.SupervisorID != nil {
			supervisors[*r.SupervisorID] = true
		}
	}
	for _, exID := range []string{in.Examiner1ID, in.Examiner2ID, in.Examiner3ID} {
		if supervisors[exID] {
			return fiber.NewError(422, "Dosen penguji tidak boleh merangkap sebagai pembimbing pada kelompok ini.")
		}
	}

	conflictTitle, e := scheduleConflict(c.Context(), tx, "proposal_seminars", p.ID, scheduledAt, in.Location, []string{ex1.ID, ex2.ID, ex3.ID})
	if e != nil {
		return e
	}
	if conflictTitle != "" {
		return fiber.NewError(409, fmt.Sprintf("Terjadi benturan jadwal ruang atau penguji dengan kelompok %q.", conflictTitle))
	}

	var id, status string
	var version int
	e = tx.QueryRow(c.Context(), `SELECT id,status,version FROM capstone_service.proposal_seminars WHERE project_id=$1 FOR UPDATE`, p.ID).Scan(&id, &status, &version)
	if errors.Is(e, pgx.ErrNoRows) {
		id = uuid.NewString()
		_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.proposal_seminars(id,project_id,scheduled_at,location,examiner_1_id,examiner_1_name,examiner_2_id,examiner_2_name,examiner_3_id,examiner_3_name,scheduled_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			id, p.ID, scheduledAt, in.Location, ex1.ID, ex1.FullName, ex2.ID, ex2.FullName, ex3.ID, ex3.FullName, actor(c).ID)
	} else if e == nil {
		if status == "APPROVED" {
			return fiber.NewError(409, "Seminar proposal telah disahkan dan tidak dapat dijadwalkan ulang.")
		}
		if e = stale(version, in.Version); e != nil {
			return e
		}
		nextStatus := status
		if status == "REJECTED" {
			nextStatus = "SCHEDULED"
		}
		full, _ := hasFullSOPSchema(c.Context(), tx)
		if full {
			histEntry, _ := json.Marshal([]map[string]any{{"changed_at": time.Now(), "actor_id": actor(c).ID, "scheduled_at": scheduledAt, "location": in.Location}})
			_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_seminars SET scheduled_at=$2,location=$3,examiner_1_id=$4,examiner_1_name=$5,examiner_2_id=$6,examiner_2_name=$7,examiner_3_id=$8,examiner_3_name=$9,status=$10,schedule_history=coalesce(schedule_history,'[]'::jsonb)||$11::jsonb,version=version+1,updated_at=now() WHERE id=$1`,
				id, scheduledAt, in.Location, ex1.ID, ex1.FullName, ex2.ID, ex2.FullName, ex3.ID, ex3.FullName, nextStatus, string(histEntry))
		} else {
			_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_seminars SET scheduled_at=$2,location=$3,examiner_1_id=$4,examiner_1_name=$5,examiner_2_id=$6,examiner_2_name=$7,examiner_3_id=$8,examiner_3_name=$9,status=$10,version=version+1,updated_at=now() WHERE id=$1`,
				id, scheduledAt, in.Location, ex1.ID, ex1.FullName, ex2.ID, ex2.FullName, ex3.ID, ex3.FullName, nextStatus)
		}
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), p.ID, "SEMPRO_SCHEDULED", map[string]any{"seminar_id": id, "scheduled_at": scheduledAt, "location": in.Location}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}

func (s *Server) evaluateSempro(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return fiber.NewError(503, "Seminar proposal belum tersedia. Hubungi administrator.")
	}
	seminarID := c.Params("id")
	if !domain.ValidID(seminarID) {
		return fiber.ErrBadRequest
	}
	var in domain.SemproEvaluateInput
	if e := c.Bind().Body(&in); e != nil {
		return fiber.ErrBadRequest
	}
	totalScore, gradeLabel, e := in.Validate()
	if e != nil {
		return fiber.NewError(422, e.Error())
	}

	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())

	var projectID, ex1, ex2, ex3, status string
	if e = tx.QueryRow(c.Context(), `SELECT project_id,examiner_1_id,examiner_2_id,examiner_3_id,status FROM capstone_service.proposal_seminars WHERE id=$1 FOR UPDATE`, seminarID).Scan(&projectID, &ex1, &ex2, &ex3, &status); e != nil {
		return e
	}
	if status == "APPROVED" {
		return fiber.NewError(409, "Seminar proposal telah disahkan sehingga penilaian dikunci.")
	}

	evaluatorRole := ""
	switch actor(c).ID {
	case ex1:
		evaluatorRole = "EXAMINER_1"
	case ex2:
		evaluatorRole = "EXAMINER_2"
	case ex3:
		evaluatorRole = "EXAMINER_3"
	default:
		var isSupervisor bool
		if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND supervisor_id=$2)`, projectID, actor(c).ID).Scan(&isSupervisor); e != nil {
			return e
		}
		if !isSupervisor {
			return fiber.ErrForbidden
		}
		evaluatorRole = "SUPERVISOR"
	}

	var studentName, nim string
	if e = tx.QueryRow(c.Context(), `SELECT student_name,nim FROM capstone_service.project_students WHERE project_id=$1 AND student_id=$2 LIMIT 1`, projectID, in.StudentID).Scan(&studentName, &nim); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return fiber.NewError(422, "Mahasiswa bukan anggota proyek ini.")
		}
		return e
	}

	scoreBytes, e := json.Marshal(in.Scores)
	if e != nil {
		return e
	}

	_, e = tx.Exec(c.Context(), `
		INSERT INTO capstone_service.seminar_evaluations(id,seminar_id,project_id,student_id,student_name,nim,evaluator_id,evaluator_name,evaluator_role,scores,total_score,grade_label,recommendation,notes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (seminar_id,student_id,evaluator_id)
		DO UPDATE SET evaluator_role=EXCLUDED.evaluator_role,evaluator_name=EXCLUDED.evaluator_name,scores=EXCLUDED.scores,total_score=EXCLUDED.total_score,grade_label=EXCLUDED.grade_label,recommendation=EXCLUDED.recommendation,notes=EXCLUDED.notes,version=capstone_service.seminar_evaluations.version+1,updated_at=now()`,
		uuid.NewString(), seminarID, projectID, in.StudentID, studentName, nim, actor(c).ID, actor(c).FullName, evaluatorRole, scoreBytes, totalScore, gradeLabel, in.Recommendation, in.Notes)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "SEMPRO_EVALUATED", map[string]any{"seminar_id": seminarID, "student_id": in.StudentID, "role": evaluatorRole, "total_score": totalScore}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"total_score": totalScore, "grade_label": gradeLabel})
}

func (s *Server) decideSempro(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return fiber.NewError(503, "Seminar proposal belum tersedia. Hubungi administrator.")
	}
	seminarID := c.Params("id")
	if !domain.ValidID(seminarID) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Status  string `json:"status"`
		Note    string `json:"note"`
		Version int    `json:"version"`
	}
	if e := c.Bind().Body(&in); e != nil {
		return fiber.ErrBadRequest
	}
	in.Note = strings.TrimSpace(in.Note)
	if in.Status != "APPROVED" && in.Status != "REVISION" && in.Status != "REJECTED" {
		return fiber.ErrUnprocessableEntity
	}
	if in.Status != "APPROVED" && !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Catatan hasil seminar wajib diisi minimal 5 karakter.")
	}
	if len(in.Note) > 2000 {
		return fiber.NewError(422, "Catatan maksimal 2000 karakter.")
	}

	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())

	var projectID string
	var version int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.proposal_seminars WHERE id=$1 FOR UPDATE`, seminarID).Scan(&projectID, &version); e != nil {
		return e
	}
	if e = stale(version, in.Version); e != nil {
		return e
	}
	if in.Status == "APPROVED" {
		var incomplete bool
		if e = tx.QueryRow(c.Context(), `
			SELECT EXISTS(
				SELECT 1 FROM (SELECT DISTINCT student_id FROM capstone_service.project_students WHERE project_id=$1) st
				WHERE (SELECT count(*) FROM capstone_service.seminar_evaluations ev WHERE ev.seminar_id=$2 AND ev.student_id=st.student_id AND ev.evaluator_role IN ('EXAMINER_1','EXAMINER_2','EXAMINER_3')) < 3
			)`, projectID, seminarID).Scan(&incomplete); e != nil {
			return e
		}
		if incomplete {
			return fiber.NewError(422, "Penilaian individu oleh ketiga penguji untuk seluruh anggota kelompok wajib lengkap sebelum pengesahan.")
		}
	}

	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_seminars SET status=$2,outcome_note=$3,decided_by=$4,decided_at=now(),version=version+1,updated_at=now() WHERE id=$1`, seminarID, in.Status, in.Note, actor(c).ID)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "SEMPRO_DECIDED", map[string]any{"seminar_id": seminarID, "status": in.Status, "note": in.Note}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) submitSemproRevision(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return fiber.NewError(503, "Seminar proposal belum tersedia. Hubungi administrator.")
	}
	seminarID := c.Params("id")
	if !domain.ValidID(seminarID) {
		return fiber.ErrBadRequest
	}
	var in struct {
		RevisionFileID string `json:"revision_file_id"`
		RevisionNote   string `json:"revision_note"`
		Version        int    `json:"version"`
	}
	if e := c.Bind().Body(&in); e != nil {
		return fiber.ErrBadRequest
	}
	in.RevisionNote = strings.TrimSpace(in.RevisionNote)
	if !domain.ValidID(in.RevisionFileID) || !domain.Text(in.RevisionNote, 5, 2000) {
		return fiber.NewError(422, "Berkas revisi PDF dan ringkasan perbaikan minimal 5 karakter wajib diisi.")
	}

	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())

	var projectID, status string
	var version int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,status,version FROM capstone_service.proposal_seminars WHERE id=$1 FOR UPDATE`, seminarID).Scan(&projectID, &status, &version); e != nil {
		return e
	}
	var member bool
	if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND student_id=$2)`, projectID, actor(c).ID).Scan(&member); e != nil {
		return e
	}
	if !member {
		return fiber.ErrForbidden
	}
	if status != "REVISION" {
		return fiber.NewError(409, "Pengumpulan revisi hanya tersedia saat seminar berstatus Perlu revisi.")
	}
	if e = stale(version, in.Version); e != nil {
		return e
	}
	if e = s.ownedFile(c.Context(), tx, in.RevisionFileID, actor(c).ID, "application/pdf"); e != nil {
		return e
	}
	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_seminars SET revision_file_id=$2,revision_note=$3,revision_submitted_at=now(),version=version+1,updated_at=now() WHERE id=$1`, seminarID, in.RevisionFileID, in.RevisionNote)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "SEMPRO_REVISION_SUBMITTED", map[string]any{"seminar_id": seminarID, "file_id": in.RevisionFileID}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) semproReport(c fiber.Ctx) error {
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return fiber.NewError(503, "Seminar proposal belum tersedia. Hubungi administrator.")
	}
	seminarID := c.Params("id")
	if !domain.ValidID(seminarID) {
		return fiber.ErrBadRequest
	}
	var projectID string
	if e := s.DB.QueryRow(c.Context(), `SELECT project_id FROM capstone_service.proposal_seminars WHERE id=$1`, seminarID).Scan(&projectID); e != nil {
		return e
	}
	p, e := loadProject(c.Context(), s.DB, projectID, false)
	if e != nil {
		return e
	}
	sem, e := loadSeminarForProject(c.Context(), s.DB, projectID, p.Students)
	if e != nil || sem == nil {
		return fiber.ErrNotFound
	}
	u := actor(c)
	allowed := u.Has("ADMIN", "KOORDINATOR_TA", "TU") || sem.Examiner1ID == u.ID || sem.Examiner2ID == u.ID || sem.Examiner3ID == u.ID
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

	wita := time.FixedZone("WITA", 8*60*60)
	paras := []string{
		"Judul Capstone: " + p.Title,
		"Bidang Minat: " + p.InterestName,
		"Jadwal Seminar: " + sem.ScheduledAt.In(wita).Format("02-01-2006 15:04 MST") + " | Tempat/Tautan: " + sem.Location,
		fmt.Sprintf("Tim Penguji: 1. %s | 2. %s | 3. %s", sem.Examiner1Name, sem.Examiner2Name, sem.Examiner3Name),
		"Status Hasil Seminar: " + sem.Status,
	}
	if sem.OutcomeNote != "" {
		paras = append(paras, "Catatan Hasil: "+sem.OutcomeNote)
	}
	for _, st := range sem.StudentSummaries {
		paras = append(paras, fmt.Sprintf("Mahasiswa: %s (%s) — Rata-rata Penguji: %.2f (%s) [%d/3 penguji]", st.StudentName, st.NIM, st.AverageScore, st.GradeLabel, st.ExaminerCount))
	}
	b, e := reports.PDF("Berita Acara Seminar Proposal Capstone", paras)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="Berita_Acara_Sempro.pdf"`)
	return c.Send(b)
}
