package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"github.com/gofiber/fiber/v3"
	"time"
)

type dashboardStats struct {
	Projects            int `json:"projects"`
	Draft               int `json:"draft"`
	Submitted           int `json:"submitted"`
	Revision            int `json:"revision"`
	Finalized           int `json:"finalized"`
	Students            int `json:"students"`
	SupervisedStudents  int `json:"supervised_students"`
	TitlePending        int `json:"title_pending"`
	ConfirmationPending int `json:"confirmation_pending"`
	Guidance            int `json:"guidance"`
	GuidancePending     int `json:"guidance_pending"`
	Proposals           int `json:"proposals"`
	ProposalReady       int `json:"proposal_ready"`
	SimakPending        int `json:"simak_pending"`
	ApprovalPending     int `json:"approval_pending"`
}
type dashboardProject struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`
	InterestName string    `json:"interest_name"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type dashboardInterest struct {
	Name      string `json:"name"`
	Total     int    `json:"total"`
	Finalized int    `json:"finalized"`
}

func dashboardScope(u domain.User) string {
	for _, entry := range []struct{ role, scope string }{{"ADMIN", "admin"}, {"KOORDINATOR_TA", "coordinator"}, {"TU", "tu"}, {"MAHASISWA", "student"}, {"KOPRODI", "koprodi"}, {"DOSEN", "lecturer"}} {
		if u.Has(entry.role) {
			return entry.scope
		}
	}
	return ""
}
func (s *Server) dashboard(c fiber.Ctx) error {
	u := actor(c)
	scope := dashboardScope(u)
	if scope == "" {
		return fiber.ErrForbidden
	}
	global := u.Has("ADMIN", "KOORDINATOR_TA", "TU", "KOPRODI")
	var stats dashboardStats
	where := ` FROM capstone_service.projects p WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND (m.student_id=$2 OR m.candidate_id=$2 OR m.supervisor_id=$2)))`
	err := s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE p.status='DRAFT'),count(*) FILTER(WHERE p.status='SUBMITTED'),count(*) FILTER(WHERE p.status='REVISION'),count(*) FILTER(WHERE p.status='FINALIZED')`+where, global, u.ID).Scan(&stats.Projects, &stats.Draft, &stats.Submitted, &stats.Revision, &stats.Finalized)
	if err != nil {
		return err
	}
	err = s.DB.QueryRow(c.Context(), `SELECT count(DISTINCT m.student_id),count(DISTINCT m.student_id) FILTER(WHERE m.supervisor_id=$2),count(*) FILTER(WHERE p.status='SUBMITTED' AND m.title_status='PENDING' AND coalesce(m.supervisor_id,m.candidate_id) IS NOT NULL),count(*) FILTER(WHERE p.status='SUBMITTED' AND m.confirmation_status='PENDING' AND coalesce(m.supervisor_id,m.candidate_id) IS NOT NULL) FROM capstone_service.project_students m JOIN capstone_service.projects p ON p.id=m.project_id WHERE ($1 OR m.student_id=$2 OR coalesce(m.supervisor_id,m.candidate_id)=$2)`, global, u.ID).Scan(&stats.Students, &stats.SupervisedStudents, &stats.TitlePending, &stats.ConfirmationPending)
	if err != nil {
		return err
	}
	if scope != "tu" {
		err = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE l.status='PENDING' AND ($2 OR l.student_id=$1 OR l.supervisor_id=$1)) FROM capstone_service.guidance_logs l WHERE ($2 OR l.student_id=$1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=l.project_id AND m.student_id=l.student_id AND m.supervisor_id=$1))`, u.ID, u.Has("ADMIN", "KOORDINATOR_TA", "KOPRODI")).Scan(&stats.Guidance, &stats.GuidancePending)
		if err != nil {
			return err
		}
	}
	if scope != "lecturer" {
		err = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE simak_status='VALID' AND approval_status='VALID'),count(*) FILTER(WHERE simak_status='PENDING'),count(*) FILTER(WHERE approval_status='PENDING') FROM capstone_service.proposal_applications WHERE ($1 OR student_id=$2)`, global, u.ID).Scan(&stats.Proposals, &stats.ProposalReady, &stats.SimakPending, &stats.ApprovalPending)
		if err != nil {
			return err
		}
	}
	return s.dashboardWithRecent(c, scope, stats, global)
}
func (s *Server) dashboardWithRecent(c fiber.Ctx, scope string, stats dashboardStats, global bool) error {
	rows, err := s.DB.Query(c.Context(), `SELECT p.id,p.title,p.status,i.name,p.updated_at FROM capstone_service.projects p JOIN capstone_service.interest_fields i ON i.id=p.interest_id WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND (m.student_id=$2 OR m.candidate_id=$2 OR m.supervisor_id=$2))) ORDER BY p.updated_at DESC,p.id LIMIT 5`, global, actor(c).ID)
	if err != nil {
		return err
	}
	recent := []dashboardProject{}
	for rows.Next() {
		var p dashboardProject
		if err = rows.Scan(&p.ID, &p.Title, &p.Status, &p.InterestName, &p.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		recent = append(recent, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	interests := []dashboardInterest{}
	if global {
		rows, err = s.DB.Query(c.Context(), `SELECT i.name,count(p.id),count(p.id) FILTER(WHERE p.status='FINALIZED') FROM capstone_service.interest_fields i LEFT JOIN capstone_service.projects p ON p.interest_id=i.id GROUP BY i.id,i.name ORDER BY i.name`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var i dashboardInterest
			if err = rows.Scan(&i.Name, &i.Total, &i.Finalized); err != nil {
				rows.Close()
				return err
			}
			interests = append(interests, i)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return c.JSON(fiber.Map{"scope": scope, "stats": stats, "recent_projects": recent, "interest_summary": interests})
}
