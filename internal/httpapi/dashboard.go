package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"time"
)

type dashboardStats struct {
	Groups              int `json:"groups"`
	GroupsReady         int `json:"groups_ready"`
	Projects            int `json:"projects"`
	Draft               int `json:"draft"`
	Submitted           int `json:"submitted"`
	Revision            int `json:"revision"`
	Finalized           int `json:"finalized"`
	Students            int `json:"students"`
	SupervisedStudents  int `json:"supervised_students"`
	TitlePending        int `json:"title_pending"`
	ConfirmationPending int `json:"confirmation_pending"`
	TopicSeminars       int `json:"topic_seminars"`
	TopicSeminarsLayak  int `json:"topic_seminars_layak"`
	Guidance            int `json:"guidance"`
	GuidanceApproved    int `json:"guidance_approved"`
	GuidancePending     int `json:"guidance_pending"`
	GuidanceRevision    int `json:"guidance_revision"`
	Proposals           int `json:"proposals"`
	ProposalReady       int `json:"proposal_ready"`
	SimakPending        int `json:"simak_pending"`
	SimakValid          int `json:"simak_valid"`
	SimakRevision       int `json:"simak_revision"`
	ApprovalPending     int `json:"approval_pending"`
	ApprovalValid       int `json:"approval_valid"`
	ApprovalRevision    int `json:"approval_revision"`
	DocumentsUploaded   int `json:"documents_uploaded"`
	DocumentsPending    int `json:"documents_pending"`
	DocumentsApproved   int `json:"documents_approved"`
	DocumentsRevision   int `json:"documents_revision"`
	SemproScheduled     int `json:"sempro_scheduled"`
	SemproRevision      int `json:"sempro_revision"`
	SemproApproved      int `json:"sempro_approved"`
	SemproRejected      int `json:"sempro_rejected"`
	FinalExamApps       int `json:"final_exam_apps"`
	FinalExamScheduled  int `json:"final_exam_scheduled"`
	FinalExamPassed     int `json:"final_exam_passed"`
	ExpoSubmitted       int `json:"expo_submitted"`
	ExpoPublished       int `json:"expo_published"`
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
	for _, entry := range []struct{ role, scope string }{{"ADMIN", "admin"}, {"KOORDINATOR_TA", "coordinator"}, {"TU", "tu"}, {"KOPRODI", "koprodi"}, {"MAHASISWA", "student"}, {"DOSEN", "lecturer"}} {
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
	var hasSemproTables bool
	if err := s.DB.QueryRow(c.Context(), `SELECT to_regclass('capstone_service.project_documents') IS NOT NULL AND to_regclass('capstone_service.proposal_seminars') IS NOT NULL`).Scan(&hasSemproTables); err != nil {
		return err
	}
	var stats dashboardStats
	projectFilter := `($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND (m.student_id=$2 OR m.candidate_id=$2 OR m.supervisor_id=$2)))`
	if hasSemproTables {
		projectFilter = `($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND (m.student_id=$2 OR m.candidate_id=$2 OR m.supervisor_id=$2)) OR EXISTS(SELECT 1 FROM capstone_service.proposal_seminars s WHERE s.project_id=p.id AND (s.examiner_1_id=$2 OR s.examiner_2_id=$2 OR s.examiner_3_id=$2)))`
	}
	if revisionSchema(c.Context(), s.DB) == nil {
		projectFilter = "(" + projectFilter + ` OR EXISTS(SELECT 1 FROM capstone_service.proposal_exams e JOIN capstone_service.proposal_applications a ON a.id=e.application_id WHERE a.project_id=p.id AND e.panel @> jsonb_build_array(jsonb_build_object('id',$2::text))))`
	}
	where := ` FROM capstone_service.projects p WHERE ` + projectFilter
	err := s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE p.status='DRAFT'),count(*) FILTER(WHERE p.status='SUBMITTED'),count(*) FILTER(WHERE p.status='REVISION'),count(*) FILTER(WHERE p.status='FINALIZED')`+where, global, u.ID).Scan(&stats.Projects, &stats.Draft, &stats.Submitted, &stats.Revision, &stats.Finalized)
	if err != nil {
		return err
	}
	err = s.DB.QueryRow(c.Context(), `SELECT count(DISTINCT m.student_id),count(DISTINCT m.student_id) FILTER(WHERE m.supervisor_id=$2),count(*) FILTER(WHERE p.status='SUBMITTED' AND m.title_status='PENDING' AND coalesce(m.supervisor_id,m.candidate_id) IS NOT NULL),count(*) FILTER(WHERE p.status='SUBMITTED' AND m.confirmation_status='PENDING' AND coalesce(m.supervisor_id,m.candidate_id) IS NOT NULL) FROM capstone_service.project_students m JOIN capstone_service.projects p ON p.id=m.project_id WHERE ($1 OR m.student_id=$2 OR coalesce(m.supervisor_id,m.candidate_id)=$2)`, global, u.ID).Scan(&stats.Students, &stats.SupervisedStudents, &stats.TitlePending, &stats.ConfirmationPending)
	if err != nil {
		return err
	}
	if scope != "tu" {
		err = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE l.status='APPROVED'),count(*) FILTER(WHERE l.status='PENDING' AND ($2 OR l.student_id=$1 OR l.supervisor_id=$1)),count(*) FILTER(WHERE l.status='REVISION') FROM capstone_service.guidance_logs l WHERE ($2 OR l.student_id=$1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=l.project_id AND m.student_id=l.student_id AND m.supervisor_id=$1))`, u.ID, u.Has("ADMIN", "KOORDINATOR_TA")).Scan(&stats.Guidance, &stats.GuidanceApproved, &stats.GuidancePending, &stats.GuidanceRevision)
		if err != nil {
			return err
		}
	}
	if scope != "lecturer" {
		err = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE simak_status='VALID' AND approval_status='VALID'),count(*) FILTER(WHERE simak_status='PENDING'),count(*) FILTER(WHERE simak_status='VALID'),count(*) FILTER(WHERE simak_status='REVISION'),count(*) FILTER(WHERE approval_status='PENDING'),count(*) FILTER(WHERE approval_status='VALID'),count(*) FILTER(WHERE approval_status='REVISION') FROM capstone_service.proposal_applications WHERE ($1 OR student_id=$2)`, global, u.ID).Scan(&stats.Proposals, &stats.ProposalReady, &stats.SimakPending, &stats.SimakValid, &stats.SimakRevision, &stats.ApprovalPending, &stats.ApprovalValid, &stats.ApprovalRevision)
		if err != nil {
			return err
		}
	}
	if hasSemproTables {
		err = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE status='PENDING'),count(*) FILTER(WHERE status='APPROVED'),count(*) FILTER(WHERE status='REVISION') FROM capstone_service.project_documents d WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=d.project_id AND (m.student_id=$2 OR m.supervisor_id=$2)))`, global, u.ID).Scan(&stats.DocumentsUploaded, &stats.DocumentsPending, &stats.DocumentsApproved, &stats.DocumentsRevision)
		if err != nil {
			return err
		}
		err = s.DB.QueryRow(c.Context(), `SELECT count(*) FILTER(WHERE s.status='SCHEDULED'),count(*) FILTER(WHERE s.status='REVISION'),count(*) FILTER(WHERE s.status='APPROVED'),count(*) FILTER(WHERE s.status='REJECTED') FROM capstone_service.proposal_seminars s WHERE ($1 OR s.examiner_1_id=$2 OR s.examiner_2_id=$2 OR s.examiner_3_id=$2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=s.project_id AND (m.student_id=$2 OR m.supervisor_id=$2)))`, global, u.ID).Scan(&stats.SemproScheduled, &stats.SemproRevision, &stats.SemproApproved, &stats.SemproRejected)
		if err != nil {
			return err
		}
	}
	hasFull, err := hasFullSOPSchema(c.Context(), s.DB)
	if err != nil {
		return err
	}
	if hasFull {
		_ = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE status IN ('READY','LOCKED')) FROM capstone_service.capstone_groups g WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.capstone_group_members m WHERE m.group_id=g.id AND m.student_id=$2))`, global, u.ID).Scan(&stats.Groups, &stats.GroupsReady)
		_ = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE status='LAYAK') FROM capstone_service.topic_seminars ts WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=ts.project_id AND (m.student_id=$2 OR m.supervisor_id=$2)) OR ts.panelist_1_id=$2 OR ts.panelist_2_id=$2)`, global, u.ID).Scan(&stats.TopicSeminars, &stats.TopicSeminarsLayak)
		_ = s.DB.QueryRow(c.Context(), `SELECT count(*) FROM capstone_service.final_exam_applications fa WHERE ($1 OR fa.student_id=$2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=fa.project_id AND m.supervisor_id=$2))`, global, u.ID).Scan(&stats.FinalExamApps)
		_ = s.DB.QueryRow(c.Context(), `SELECT count(*) FILTER(WHERE fe.status IN ('SCHEDULED','REVISION')),count(*) FILTER(WHERE fe.status='PASSED') FROM capstone_service.final_exams fe WHERE ($1 OR fe.examiner_1_id=$2 OR fe.examiner_2_id=$2 OR fe.examiner_3_id=$2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=fe.project_id AND (m.student_id=$2 OR m.supervisor_id=$2)))`, global, u.ID).Scan(&stats.FinalExamScheduled, &stats.FinalExamPassed)
		_ = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE status='PUBLISHED') FROM capstone_service.expo_submissions ex WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=ex.project_id AND (m.student_id=$2 OR m.supervisor_id=$2)))`, global, u.ID).Scan(&stats.ExpoSubmitted, &stats.ExpoPublished)
	}
	if revisionSchema(c.Context(), s.DB) == nil {
		if err = s.DB.QueryRow(c.Context(), `SELECT count(*),count(*) FILTER(WHERE (SELECT count(*) FROM capstone_service.proposal_documents d WHERE d.application_id=a.id AND d.status='VALID')=6) FROM capstone_service.proposal_applications a WHERE ($1 OR a.student_id=$2)`, global || u.Has("DOSEN"), u.ID).Scan(&stats.Proposals, &stats.ProposalReady); err != nil {
			return err
		}
		var individual dashboardStats
		member := fmt.Sprintf(`[{"id":%q}]`, u.ID)
		if err = s.DB.QueryRow(c.Context(), `SELECT count(*) FILTER(WHERE n<5),count(*) FILTER(WHERE n=5 AND rejected=0 AND revised>0),count(*) FILTER(WHERE n=5 AND rejected=0 AND revised=0),count(*) FILTER(WHERE n=5 AND rejected>0) FROM (SELECT e.id,count(v.evaluator_id) n,count(*) FILTER(WHERE v.recommendation='REJECTED') rejected,count(*) FILTER(WHERE v.recommendation='REVISION') revised FROM capstone_service.proposal_exams e JOIN capstone_service.proposal_applications a ON a.id=e.application_id LEFT JOIN capstone_service.proposal_exam_evaluations v ON v.exam_id=e.id WHERE ($1 OR a.student_id=$2 OR e.panel @> $3::jsonb) GROUP BY e.id) x`, global, u.ID, member).Scan(&individual.SemproScheduled, &individual.SemproRevision, &individual.SemproApproved, &individual.SemproRejected); err != nil {
			return err
		}
		if individual.SemproScheduled+individual.SemproRevision+individual.SemproApproved+individual.SemproRejected > 0 {
			stats.SemproScheduled = individual.SemproScheduled
			stats.SemproRevision = individual.SemproRevision
			stats.SemproApproved = individual.SemproApproved
			stats.SemproRejected = individual.SemproRejected
		}
	}
	return s.dashboardWithRecent(c, scope, stats, global, projectFilter, hasFull)
}
func (s *Server) dashboardWithRecent(c fiber.Ctx, scope string, stats dashboardStats, global bool, projectFilter string, hasFull bool) error {
	rows, err := s.DB.Query(c.Context(), `SELECT p.id,p.title,p.status,i.name,p.updated_at FROM capstone_service.projects p JOIN capstone_service.interest_fields i ON i.id=p.interest_id WHERE `+projectFilter+` ORDER BY p.updated_at DESC,p.id LIMIT 5`, global, actor(c).ID)
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
	var studentGroup *domain.CapstoneGroup
	if scope == "student" {
		if hasFull {
			var gid string
			if err := s.DB.QueryRow(c.Context(), `SELECT g.id FROM capstone_service.capstone_groups g JOIN capstone_service.capstone_group_members m ON m.group_id=g.id WHERE m.student_id=$1 ORDER BY g.updated_at DESC LIMIT 1`, actor(c).ID).Scan(&gid); err == nil {
				if g, err := loadGroup(c.Context(), s.DB, gid); err == nil {
					studentGroup = &g
				}
			}
		} else if len(recent) > 0 {
			if p, err := loadProject(c.Context(), s.DB, recent[0].ID, false); err == nil {
				g := domain.CapstoneGroup{
					ID:            p.ID,
					Code:          "TIM-CAP",
					InterestID:    p.InterestID,
					InterestName:  p.InterestName,
					WorkingTitle:  p.Title,
					LeaderID:      p.OwnerID,
					Status:        p.Status,
					ProjectID:     &p.ID,
					ProjectTitle:  p.Title,
					ProjectStatus: p.Status,
					Members:       []domain.GroupMember{},
					Advisors:      []domain.GroupAdvisorBrief{},
				}
				seenM := map[string]bool{}
				for _, ps := range p.Students {
					if !seenM[ps.StudentID] {
						seenM[ps.StudentID] = true
						isL := ps.StudentID == p.OwnerID
						if isL {
							g.LeaderName = ps.StudentName
						}
						resp := "Anggota Tim"
						if isL {
							resp = "Ketua Kelompok"
						}
						g.Members = append(g.Members, domain.GroupMember{StudentID: ps.StudentID, StudentName: ps.StudentName, NIM: ps.NIM, IsLeader: isL, Responsibility: resp})
					}
					g.Advisors = append(g.Advisors, domain.GroupAdvisorBrief{
						StudentID: ps.StudentID, StudentName: ps.StudentName, Slot: ps.Slot, CandidateID: ps.CandidateID, CandidateName: ps.CandidateName,
						SupervisorID: ps.SupervisorID, SupervisorName: ps.SupervisorName, TitleStatus: ps.TitleStatus, ConfirmationStatus: ps.ConfirmationStatus,
					})
				}
				studentGroup = &g
			}
		}
	}
	var activePeriod *domain.AcademicPeriod
	if hasFull {
		var ap domain.AcademicPeriod
		if err := s.DB.QueryRow(c.Context(), `SELECT id,code,name,semester,start_date::text,end_date::text,group_open,topic_open,sempro_open,exam_open,is_active FROM capstone_service.academic_periods ORDER BY is_active DESC, start_date DESC LIMIT 1`).Scan(
			&ap.ID, &ap.Code, &ap.Name, &ap.Semester, &ap.StartDate, &ap.EndDate, &ap.GroupOpen, &ap.TopicOpen, &ap.SemproOpen, &ap.ExamOpen, &ap.IsActive); err == nil {
			activePeriod = &ap
		}
	}
	return c.JSON(fiber.Map{"scope": scope, "stats": stats, "recent_projects": recent, "interest_summary": interests, "student_group": studentGroup, "active_period": activePeriod})
}
