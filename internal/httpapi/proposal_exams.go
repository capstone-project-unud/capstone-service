package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type examPanel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}
type examEvaluation struct {
	EvaluatorID    string             `json:"evaluator_id"`
	Scores         map[string]float64 `json:"scores"`
	Total          float64            `json:"total"`
	Recommendation string             `json:"recommendation"`
	Notes          string             `json:"notes"`
	Version        int                `json:"version"`
	UpdatedAt      time.Time          `json:"updated_at"`
}
type examAttendance struct {
	LecturerID  string    `json:"lecturer_id"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}
type proposalExam struct {
	ID                     string                 `json:"id"`
	ApplicationID          string                 `json:"application_id"`
	ProjectID              string                 `json:"project_id"`
	StudentID              string                 `json:"student_id"`
	StudentName            string                 `json:"student_name"`
	NIM                    string                 `json:"nim"`
	Title                  string                 `json:"title"`
	ScheduledAt            time.Time              `json:"scheduled_at"`
	Location               string                 `json:"location"`
	Panel                  []examPanel            `json:"panel"`
	PolicyID               *string                `json:"policy_id"`
	Policy                 *domain.ProposalPolicy `json:"policy"`
	RevisionApprovedFileID *string                `json:"revision_approved_file_id"`
	RevisionItems          []domain.RevisionItem  `json:"revision_items"`
	Version                int                    `json:"version"`
	Evaluations            []examEvaluation       `json:"evaluations"`
	Attendance             []examAttendance       `json:"attendance"`
	FinalScore             *float64               `json:"final_score"`
	Letter                 string                 `json:"letter"`
	Category               string                 `json:"category"`
	Outcome                string                 `json:"outcome"`
}

func (ex proposalExam) panelMember(id string) *examPanel {
	for _, p := range ex.Panel {
		if p.ID == id {
			return &p
		}
	}
	return nil
}
func (ex proposalExam) canRead(u domain.User) bool {
	return ex.StudentID == u.ID || u.Has("ADMIN", "TU", "KOORDINATOR_TA", "KOPRODI") || ex.panelMember(u.ID) != nil
}

const proposalExamSelect = `SELECT to_jsonb(e)||jsonb_build_object(
 'project_id',a.project_id,'student_id',a.student_id,'student_name',m.student_name,'nim',m.nim,'title',p.title,'policy',gp.rules,
 'evaluations',coalesce((SELECT jsonb_agg(to_jsonb(v)-'signature_key' ORDER BY v.evaluator_id) FROM capstone_service.proposal_exam_evaluations v WHERE v.exam_id=e.id),'[]'::jsonb),
 'attendance',coalesce((SELECT jsonb_agg(to_jsonb(t)-'signature_key' ORDER BY t.lecturer_id) FROM capstone_service.proposal_exam_attendance t WHERE t.exam_id=e.id),'[]'::jsonb),
 'revision_items',coalesce((SELECT jsonb_agg(to_jsonb(r) ORDER BY r.created_at ASC) FROM capstone_service.seminar_revision_items r WHERE r.stage='SEMPRO' AND r.seminar_id=e.id),'[]'::jsonb))
 FROM capstone_service.proposal_exams e JOIN capstone_service.proposal_applications a ON a.id=e.application_id
 JOIN capstone_service.projects p ON p.id=a.project_id JOIN capstone_service.project_students m ON m.project_id=a.project_id AND m.student_id=a.student_id AND m.slot=1
 LEFT JOIN capstone_service.proposal_grading_policies gp ON gp.id=e.policy_id `

func decodeProposalExam(raw []byte) (proposalExam, error) {
	var ex proposalExam
	if e := json.Unmarshal(raw, &ex); e != nil {
		return ex, e
	}
	if ex.Policy != nil {
		scores := map[string]float64{}
		outcome := "APPROVED"
		for _, ev := range ex.Evaluations {
			if p := ex.panelMember(ev.EvaluatorID); p != nil {
				scores[p.Role] = ev.Total
			}
			if ev.Recommendation == "REJECTED" {
				outcome = "REJECTED"
			} else if ev.Recommendation == "REVISION" && outcome != "REJECTED" {
				outcome = "REVISION"
			}
		}
		if total, letter, ok := ex.Policy.Final(scores); ok {
			ex.FinalScore = &total
			ex.Letter = letter
			ex.Category = domain.GradeCategory(total)
			ex.Outcome = outcome
		}
	}
	return ex, nil
}
func loadProposalExam(ctx context.Context, db querier, id string, lock bool) (proposalExam, error) {
	if lock {
		var lockedID string
		if e := db.QueryRow(ctx, `SELECT id FROM capstone_service.proposal_exams WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID); e != nil {
			return proposalExam{}, e
		}
	}
	var raw []byte
	if e := db.QueryRow(ctx, proposalExamSelect+" WHERE e.id=$1", id).Scan(&raw); e != nil {
		return proposalExam{}, e
	}
	return decodeProposalExam(raw)
}
func (s *Server) listProposalExams(c fiber.Ctx) error {
	u := actor(c)
	if !u.Staff() && !u.Has("MAHASISWA") {
		return fiber.ErrForbidden
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	member, _ := json.Marshal([]map[string]string{{"id": u.ID}})
	rows, e := s.DB.Query(c.Context(), proposalExamSelect+" WHERE ($1 OR a.student_id=$2 OR e.panel @> $3::jsonb) ORDER BY e.scheduled_at DESC,e.id LIMIT 1001", u.Has("ADMIN", "TU", "KOORDINATOR_TA", "KOPRODI"), u.ID, member)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []proposalExam{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return e
		}
		ex, err := decodeProposalExam(raw)
		if err != nil {
			return err
		}
		items = append(items, ex)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	if len(items) > 1000 {
		return fiber.NewError(422, "Daftar ujian melebihi 1000 data.")
	}
	return c.JSON(fiber.Map{"items": items})
}
func (s *Server) recordProposalExam(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	var in struct {
		ApplicationID string `json:"application_id"`
		ScheduledAt   string `json:"scheduled_at"`
		Location      string `json:"location"`
		Examiner1     string `json:"examiner_1_id"`
		Examiner2     string `json:"examiner_2_id"`
		Examiner3     string `json:"examiner_3_id"`
		Version       int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrBadRequest
	}
	check := domain.SemproScheduleInput{ProjectID: in.ApplicationID, ScheduledAt: in.ScheduledAt, Location: in.Location, Examiner1ID: in.Examiner1, Examiner2ID: in.Examiner2, Examiner3ID: in.Examiner3}
	date, e := check.Validate()
	if e != nil {
		return fiber.NewError(422, e.Error())
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var project string
	e = tx.QueryRow(c.Context(), `SELECT project_id FROM capstone_service.proposal_applications WHERE id=$1 AND student_id=$2 FOR UPDATE`, in.ApplicationID, actor(c).ID).Scan(&project)
	if e != nil {
		return e
	}
	var valid int
	if e = tx.QueryRow(c.Context(), `SELECT count(*) FROM capstone_service.proposal_documents WHERE application_id=$1 AND status='VALID'`, in.ApplicationID).Scan(&valid); e != nil {
		return e
	}
	if valid != 6 {
		return fiber.NewError(422, "Keenam berkas harus divalidasi TU sebelum mencatat informasi ujian.")
	}
	p, e := loadProject(c.Context(), tx, project, false)
	if e != nil {
		return e
	}
	if p.Status != "FINALIZED" {
		return fiber.ErrConflict
	}
	panel := []examPanel{}
	used := map[string]bool{}
	for _, role := range []int{1, 2} {
		for _, m := range p.Students {
			if m.StudentID == actor(c).ID && m.Slot == role && m.SupervisorID != nil {
				panel = append(panel, examPanel{*m.SupervisorID, m.SupervisorName, fmt.Sprintf("SUPERVISOR_%d", role)})
				used[*m.SupervisorID] = true
			}
		}
	}
	if len(panel) != 2 || len(used) != 2 {
		return fiber.NewError(422, "Dua pembimbing berbeda harus ditetapkan koordinator.")
	}
	for i, id := range []string{in.Examiner1, in.Examiner2, in.Examiner3} {
		if used[id] {
			return fiber.NewError(422, "Penguji harus berbeda dari pembimbing dan penguji lainnya.")
		}
		d, err := s.identity(c.Context(), id, "DOSEN")
		if err != nil {
			return err
		}
		panel = append(panel, examPanel{id, d.FullName, fmt.Sprintf("EXAMINER_%d", i+1)})
		used[id] = true
	}
	var existingID string
	var version int
	e = tx.QueryRow(c.Context(), `SELECT id,version FROM capstone_service.proposal_exams WHERE application_id=$1 FOR UPDATE`, in.ApplicationID).Scan(&existingID, &version)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	raw, _ := json.Marshal(panel)
	id := existingID
	if e == nil {
		if e = stale(version, in.Version); e != nil {
			return e
		}
		var frozen bool
		if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.proposal_exam_evaluations WHERE exam_id=$1) OR EXISTS(SELECT 1 FROM capstone_service.proposal_exam_attendance WHERE exam_id=$1)`, id).Scan(&frozen); e != nil {
			return e
		}
		if frozen {
			return fiber.NewError(409, "Informasi ujian terkunci setelah presensi atau penilaian diisi.")
		}
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_exams SET scheduled_at=$2,location=$3,panel=$4,version=version+1,updated_at=now() WHERE id=$1`, id, date, check.Location, raw)
	} else {
		if in.Version != 0 {
			return fiber.ErrConflict
		}
		id = uuid.NewString()
		_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.proposal_exams(id,application_id,scheduled_at,location,panel) VALUES($1,$2,$3,$4,$5)`, id, in.ApplicationID, date, check.Location, raw)
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), project, "PROPOSAL_EXAM_RECORDED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}
func (s *Server) proposalPolicy(c fiber.Ctx) error {
	if !actor(c).Staff() && !actor(c).Has("MAHASISWA") {
		return fiber.ErrForbidden
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	var raw []byte
	e := s.DB.QueryRow(c.Context(), `SELECT to_jsonb(p) FROM capstone_service.proposal_grading_policies p ORDER BY created_at DESC,id DESC LIMIT 1`).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return c.JSON(fiber.Map{"policy": nil, "indicators": domain.ProposalIndicators(), "panel_roles": domain.PanelRoles})
	}
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"policy": json.RawMessage(raw), "indicators": domain.ProposalIndicators(), "panel_roles": domain.PanelRoles})
}
func (s *Server) saveProposalPolicy(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	var in domain.ProposalPolicy
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrBadRequest
	}
	if e := in.Validate(); e != nil {
		return fiber.NewError(422, e.Error())
	}
	raw, _ := json.Marshal(in)
	id := uuid.NewString()
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.proposal_grading_policies(id,name,rules,created_by) VALUES($1,$2,$3,$4)`, id, in.Name, raw, actor(c).ID)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), "", "PROPOSAL_POLICY_PUBLISHED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}
func (s *Server) evaluateProposalExam(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	var in struct {
		Scores         map[string]float64 `json:"scores"`
		Recommendation string             `json:"recommendation"`
		Notes          string             `json:"notes"`
		Version        int                `json:"version"`
		Confirm        bool               `json:"confirm_signature"`
	}
	if c.Bind().JSON(&in) != nil || !in.Confirm {
		return fiber.NewError(422, "Konfirmasikan penggunaan tanda tangan pada penilaian ini.")
	}
	check := domain.SemproEvaluateInput{StudentID: actor(c).ID, Scores: in.Scores, Recommendation: in.Recommendation, Notes: in.Notes}
	if _, _, e := check.Validate(); e != nil {
		return fiber.NewError(422, e.Error())
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	ex, e := loadProposalExam(c.Context(), tx, c.Params("id"), true)
	if e != nil {
		return e
	}
	if ex.panelMember(actor(c).ID) == nil {
		return fiber.ErrForbidden
	}
	if time.Now().Before(ex.ScheduledAt) {
		return fiber.NewError(422, "Penilaian dapat diisi setelah waktu ujian.")
	}
	if ex.Policy == nil {
		var raw []byte
		var id string
		e = tx.QueryRow(c.Context(), `SELECT id,rules FROM capstone_service.proposal_grading_policies ORDER BY created_at DESC,id DESC LIMIT 1`).Scan(&id, &raw)
		if errors.Is(e, pgx.ErrNoRows) {
			return fiber.NewError(422, "Koordinator atau admin perlu menetapkan bobot dan konversi nilai terlebih dahulu.")
		}
		if e != nil {
			return e
		}
		ex.Policy = &domain.ProposalPolicy{}
		if e = json.Unmarshal(raw, ex.Policy); e != nil {
			return e
		}
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_exams SET policy_id=$2 WHERE id=$1`, ex.ID, id)
		if e != nil {
			return e
		}
	}
	total, e := ex.Policy.Score(in.Scores)
	if e != nil {
		return fiber.NewError(422, e.Error())
	}
	key, e := signatureKey(c.Context(), tx, actor(c).ID)
	if e != nil {
		return e
	}
	current := 0
	for _, ev := range ex.Evaluations {
		if ev.EvaluatorID == actor(c).ID {
			current = ev.Version
		}
	}
	if e = stale(current, in.Version); e != nil {
		return e
	}
	scores, _ := json.Marshal(in.Scores)
	_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.proposal_exam_evaluations(exam_id,evaluator_id,scores,total,recommendation,notes,signature_key) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(exam_id,evaluator_id) DO UPDATE SET scores=excluded.scores,total=excluded.total,recommendation=excluded.recommendation,notes=excluded.notes,signature_key=excluded.signature_key,version=proposal_exam_evaluations.version+1,updated_at=now()`, ex.ID, actor(c).ID, scores, total, in.Recommendation, check.Notes, key)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), ex.ProjectID, "PROPOSAL_EXAM_EVALUATED", fiber.Map{"exam_id": ex.ID, "evaluator_id": actor(c).ID}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
func (s *Server) attendProposalExam(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	var in struct {
		Confirm bool `json:"confirm_signature"`
	}
	if c.Bind().JSON(&in) != nil || !in.Confirm {
		return fiber.ErrUnprocessableEntity
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	ex, e := loadProposalExam(c.Context(), tx, c.Params("id"), true)
	if e != nil {
		return e
	}
	if ex.panelMember(actor(c).ID) == nil {
		return fiber.ErrForbidden
	}
	if time.Now().Before(ex.ScheduledAt) {
		return fiber.NewError(422, "Presensi dapat dikonfirmasi setelah waktu ujian.")
	}
	key, e := signatureKey(c.Context(), tx, actor(c).ID)
	if e != nil {
		return e
	}
	_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.proposal_exam_attendance(exam_id,lecturer_id,signature_key) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, ex.ID, actor(c).ID, key)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), ex.ProjectID, "PROPOSAL_EXAM_ATTENDED", fiber.Map{"exam_id": ex.ID}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
