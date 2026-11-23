package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

type Guidance struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	Title          string     `json:"title"`
	StudentID      string     `json:"student_id"`
	StudentName    string     `json:"student_name"`
	NIM            string     `json:"nim"`
	InterestName   string     `json:"interest_name"`
	SupervisorID   string     `json:"supervisor_id"`
	SupervisorName string     `json:"supervisor_name"`
	MeetingDate    time.Time  `json:"meeting_date"`
	Topic          string     `json:"topic"`
	Progress       string     `json:"progress"`
	Obstacles      string     `json:"obstacles"`
	NextPlan       string     `json:"next_plan"`
	Status         string     `json:"status"`
	ReviewNote     string     `json:"review_note"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	Version        int        `json:"version"`
}

func (s *Server) guidanceList(c fiber.Ctx) ([]Guidance, error) {
	u := actor(c)
	project, student, id := c.Query("project_id"), c.Query("student_id"), c.Query("id")
	for _, v := range []string{project, student, id} {
		if v != "" && !domain.ValidID(v) {
			return nil, fiber.ErrBadRequest
		}
	}
	rows, e := s.DB.Query(c.Context(), `SELECT l.id,l.project_id,p.title,l.student_id,l.student_name,l.supervisor_id,l.supervisor_name,l.meeting_date,l.topic,l.progress,l.obstacles,l.next_plan,l.status,l.review_note,l.reviewed_at,l.version,m.nim,i.name FROM capstone_service.guidance_logs l JOIN capstone_service.projects p ON p.id=l.project_id JOIN capstone_service.project_students m ON m.project_id=l.project_id AND m.student_id=l.student_id AND m.slot=1 JOIN capstone_service.interest_fields i ON i.id=p.interest_id WHERE (l.student_id=$1 OR $2 OR EXISTS(SELECT 1 FROM capstone_service.project_students r WHERE r.project_id=l.project_id AND r.student_id=l.student_id AND r.supervisor_id=$1)) AND ($3='' OR l.project_id::text=$3) AND ($4='' OR l.student_id::text=$4) AND ($5='' OR l.id::text=$5) ORDER BY l.meeting_date DESC,l.created_at DESC LIMIT 1001`, u.ID, u.Has("ADMIN", "KOORDINATOR_TA", "KOPRODI"), project, student, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []Guidance{}
	for rows.Next() {
		var l Guidance
		if e = rows.Scan(&l.ID, &l.ProjectID, &l.Title, &l.StudentID, &l.StudentName, &l.SupervisorID, &l.SupervisorName, &l.MeetingDate, &l.Topic, &l.Progress, &l.Obstacles, &l.NextPlan, &l.Status, &l.ReviewNote, &l.ReviewedAt, &l.Version, &l.NIM, &l.InterestName); e != nil {
			return nil, e
		}
		items = append(items, l)
	}
	if len(items) > 1000 {
		return nil, fiber.NewError(422, "Lebih dari 1000 log. Saring per mahasiswa atau proyek.")
	}
	return items, rows.Err()
}
func (s *Server) listGuidance(c fiber.Ctx) error {
	items, e := s.guidanceList(c)
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items})
}
func (s *Server) saveGuidance(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	var in struct {
		ProjectID    string `json:"project_id"`
		SupervisorID string `json:"supervisor_id"`
		MeetingDate  string `json:"meeting_date"`
		Topic        string `json:"topic"`
		Progress     string `json:"progress"`
		Obstacles    string `json:"obstacles"`
		NextPlan     string `json:"next_plan"`
		Version      int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.ProjectID) || !domain.ValidID(in.SupervisorID) || !domain.Text(in.Topic, 3, 300) || !domain.Text(in.Progress, 5, 10000) || !domain.Text(in.Obstacles, 0, 5000) || !domain.Text(in.NextPlan, 0, 5000) {
		return fiber.NewError(422, "Tanggal, pembimbing, topik, dan kemajuan bimbingan wajib diisi.")
	}
	date, e := time.Parse("2006-01-02", in.MeetingDate)
	if e != nil || date.After(time.Now().Add(24*time.Hour)) || date.Year() < 2000 {
		return fiber.NewError(422, "Tanggal bimbingan tidak valid.")
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
	if p.Status != "FINALIZED" {
		return fiber.NewError(422, "Judul dan pembimbing harus ditetapkan terlebih dahulu.")
	}
	assigned := false
	name := ""
	for _, r := range p.Students {
		if r.StudentID == actor(c).ID && r.SupervisorID != nil && *r.SupervisorID == in.SupervisorID {
			assigned = true
			name = r.SupervisorName
		}
	}
	if !assigned {
		return fiber.ErrForbidden
	}
	id := c.Params("id")
	if id == "" {
		id = uuid.NewString()
		_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.guidance_logs(id,project_id,student_id,student_name,supervisor_id,supervisor_name,meeting_date,topic,progress,obstacles,next_plan) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, p.ID, actor(c).ID, actor(c).FullName, in.SupervisorID, name, date, in.Topic, in.Progress, in.Obstacles, in.NextPlan)
	} else {
		if !domain.ValidID(id) {
			return fiber.ErrBadRequest
		}
		var owner, status, project string
		var version int
		if e = tx.QueryRow(c.Context(), "SELECT student_id,status,version,project_id FROM capstone_service.guidance_logs WHERE id=$1 FOR UPDATE", id).Scan(&owner, &status, &version, &project); e != nil {
			return e
		}
		if owner != actor(c).ID || project != p.ID {
			return fiber.ErrForbidden
		}
		if status == "APPROVED" {
			return fiber.NewError(409, "Log yang disetujui tidak dapat diubah.")
		}
		if e = stale(version, in.Version); e != nil {
			return e
		}
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.guidance_logs SET supervisor_id=$2,supervisor_name=$3,meeting_date=$4,topic=$5,progress=$6,obstacles=$7,next_plan=$8,status='PENDING',review_note='',reviewed_at=NULL,version=version+1,updated_at=now() WHERE id=$1`, id, in.SupervisorID, name, date, in.Topic, in.Progress, in.Obstacles, in.NextPlan)
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), p.ID, "GUIDANCE_SUBMITTED", map[string]any{"id": id, "input": in}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}
func (s *Server) reviewGuidance(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Version int    `json:"version"`
		Status  string `json:"status"`
		Note    string `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || (in.Status != "APPROVED" && in.Status != "REVISION") || !domain.Text(in.Note, 0, 2000) || (in.Status == "REVISION" && !domain.Text(in.Note, 5, 2000)) {
		return fiber.NewError(422, "Keputusan atau catatan revisi tidak valid.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var supervisor, project, status string
	var version int
	e = tx.QueryRow(c.Context(), "SELECT supervisor_id,project_id,status,version FROM capstone_service.guidance_logs WHERE id=$1 FOR UPDATE", c.Params("id")).Scan(&supervisor, &project, &status, &version)
	if e != nil {
		return e
	}
	if supervisor != actor(c).ID {
		return fiber.ErrForbidden
	}
	if status != "PENDING" {
		return fiber.ErrConflict
	}
	if e = stale(version, in.Version); e != nil {
		return e
	}
	_, e = tx.Exec(c.Context(), "UPDATE capstone_service.guidance_logs SET status=$2,review_note=$3,reviewed_at=now(),version=version+1 WHERE id=$1", c.Params("id"), in.Status, in.Note)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), project, "GUIDANCE_REVIEWED", map[string]any{"id": c.Params("id"), "input": in}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
