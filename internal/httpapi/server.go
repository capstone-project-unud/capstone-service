package httpapi

import (
	"bytes"
	"capstone/capstone-service/internal/accesslog"
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/storage"
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Server struct {
	DB                                     *pgxpool.Pool
	Files                                  storage.Store
	Client                                 *http.Client
	AuthURL, UserURL, AuthToken, UserToken string
}

func New(db *pgxpool.Pool, files storage.Store) *fiber.App {
	transport := &http.Transport{MaxIdleConns: 100, MaxIdleConnsPerHost: 32, IdleConnTimeout: 90 * time.Second}
	s := &Server{db, files, &http.Client{Timeout: 10 * time.Second, Transport: transport}, strings.TrimRight(os.Getenv("AUTH_SERVICE_URL"), "/"), strings.TrimRight(os.Getenv("USER_SERVICE_URL"), "/"), os.Getenv("AUTH_INTERNAL_TOKEN"), os.Getenv("USER_INTERNAL_TOKEN")}
	app := fiber.New(fiber.Config{AppName: "capstone-service", BodyLimit: storage.MaxFile + 65536, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, ErrorHandler: func(c fiber.Ctx, e error) error {
		code, msg := 500, "Layanan capstone mengalami gangguan."
		var fe *fiber.Error
		var pe *pgconn.PgError
		if errors.As(e, &fe) {
			code, msg = fe.Code, fe.Message
		} else if errors.Is(e, pgx.ErrNoRows) {
			code, msg = 404, "Data tidak ditemukan."
		} else if errors.As(e, &pe) && pe.Code == "23505" {
			code, msg = 409, "Data sudah digunakan. Mahasiswa hanya dapat terdaftar pada satu kelompok."
		} else {
			log.Printf("capstone request failed: %T", e)
		}
		return c.Status(code).JSON(fiber.Map{"error": fiber.Map{"message": msg}})
	}})
	app.Use(accesslog.New("capstone-service"))
	app.Use(recover.New())
	app.Use(func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 50*time.Second)
		defer cancel()
		c.SetContext(ctx)
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		return c.Next()
	})
	app.Get("/health", func(c fiber.Ctx) error {
		if db.Ping(c.Context()) != nil {
			return fiber.ErrServiceUnavailable
		}
		return c.JSON(fiber.Map{"status": "ok", "service": "capstone-service"})
	})
	app.Get("/api/v1/capstone/public/expo", s.publicExpo)
	app.Get("/api/v1/capstone/public/expo/:id/poster", s.publicExpoPoster)
	api := app.Group("/api/v1/capstone", s.authenticated)
	api.Get("/dashboard", s.dashboard)
	api.Get("/directory", s.directory)
	api.Get("/interests", s.interests)
	api.Post("/interests", s.saveInterest)
	api.Put("/interests/:id", s.saveInterest)
	api.Post("/files", s.upload)
	api.Get("/files/:id", s.download)
	api.Get("/projects/export", s.exportProjects)
	api.Get("/projects", s.listProjects)
	api.Post("/projects", s.createProject)
	api.Get("/projects/:id", s.getProject)
	api.Put("/projects/:id", s.updateProject)
	api.Post("/projects/:id/submit", s.submitProject)
	api.Post("/projects/:id/revision", s.requestRevision)
	api.Put("/projects/:id/candidates", s.updateCandidates)
	api.Put("/projects/:id/decision", s.decision)
	api.Put("/projects/:id/assignment", s.assignment)
	api.Post("/projects/:id/finalize", s.finalize)
	api.Post("/projects/:id/reopen", s.reopen)
	api.Get("/projects/:id/letter", s.letter)
	api.Get("/projects/:id/history", s.history)
	api.Get("/guidance/export", s.exportGuidance)
	api.Get("/guidance", s.listGuidance)
	api.Post("/guidance", s.saveGuidance)
	api.Put("/guidance/:id", s.saveGuidance)
	api.Put("/guidance/:id/review", s.reviewGuidance)
	api.Get("/proposals/export", s.exportProposals)
	api.Get("/proposals", s.listProposals)
	api.Post("/proposals", s.saveProposal)
	api.Put("/proposals/:id/review", s.reviewProposal)
	api.Get("/proposal-exams", s.listProposalExams)
	api.Post("/proposal-exams", s.recordProposalExam)
	api.Put("/proposal-exams/:id/evaluation", s.evaluateProposalExam)
	api.Post("/proposal-exams/:id/attendance", s.attendProposalExam)
	api.Post("/proposal-exams/:id/revision-approval", s.attachProposalExamRevisionApproval)
	api.Get("/proposal-exams/:id/report", s.proposalExamReport)
	api.Get("/proposal-grading-policy", s.proposalPolicy)
	api.Post("/proposal-grading-policy", s.saveProposalPolicy)
	api.Get("/signature", s.getSignature)
	api.Get("/signature/preview", s.previewSignature)
	api.Post("/signature", s.saveSignature)
	api.Get("/documents", s.listDocuments)
	api.Post("/documents", s.saveDocument)
	api.Put("/documents/:id/review", s.reviewDocument)
	api.Get("/sempro", s.listSempro)
	api.Post("/sempro", func(c fiber.Ctx) error {
		return fiber.NewError(410, "Jadwal ditentukan di luar sistem. Mahasiswa mencatat informasi ujian melalui /proposal-exams.")
	})
	api.Put("/sempro/:id/evaluate", s.evaluateSempro)
	api.Post("/sempro/:id/decision", s.decideSempro)
	api.Post("/sempro/:id/revision", s.submitSemproRevision)
	api.Post("/sempro/:id/revision-approval", s.attachSemproRevisionApproval)
	api.Get("/sempro/:id/report", s.semproReport)
	api.Post("/projects/:id/sempro-approval", s.saveSemproApproval)
	api.Post("/projects/:id/verify-topic", s.verifyTopicAdmin)
	api.Put("/projects/:id/assignment-letter", s.saveAssignmentLetter)

	// SOP-004 Master Data Akademik
	api.Get("/master/periods", s.listPeriods)
	api.Post("/master/periods", s.savePeriod)
	api.Put("/master/periods/:id", s.savePeriod)
	api.Get("/master/document-types", s.listDocumentTypes)
	api.Post("/master/document-types", s.saveDocumentType)
	api.Put("/master/document-types/:code", s.saveDocumentType)
	api.Get("/master/stage-requirements", s.listStageRequirements)
	api.Put("/master/stage-requirements/:stage", s.updateStageRequirement)
	api.Get("/master/rubrics", s.listRubrics)
	api.Put("/master/rubrics/:id", s.updateRubricItem)

	// SOP-005 Kelompok Capstone Project
	api.Get("/groups", s.listGroups)
	api.Post("/groups", s.saveGroup)
	api.Put("/groups/:id", s.saveGroup)
	api.Post("/groups/:id/topic", s.createTopicFromGroup)

	// SOP-008 Seminar Kelayakan Topik
	api.Get("/topic-seminars", s.listTopicSeminars)
	api.Post("/topic-seminars", s.registerTopicSeminar)
	api.Put("/topic-seminars/:id/schedule", s.scheduleTopicSeminar)
	api.Put("/topic-seminars/:id/evaluate", s.evaluateTopicSeminar)
	api.Post("/topic-seminars/:id/decision", s.decideTopicSeminar)

	// SOP-014 & SOP-017 Revisi Penguji Terstruktur
	api.Post("/revisions", s.createRevisionItem)
	api.Put("/revisions/:id/respond", s.respondRevisionItem)
	api.Put("/revisions/:id/review", s.reviewRevisionItem)

	// SOP-015..017 Ujian Akhir
	api.Get("/final-exam", s.listFinalExam)
	api.Post("/final-exam/applications", s.saveFinalExamApp)
	api.Put("/final-exam/applications/:id/tu-review", s.reviewFinalExamAppTU)
	api.Put("/final-exam/applications/:id/advisor-review", s.reviewFinalExamAppAdvisor)
	api.Put("/final-exam/applications/:id/coordinator-review", s.reviewFinalExamAppCoordinator)
	api.Post("/final-exam/schedule", s.scheduleFinalExam)
	api.Put("/final-exam/:id/evaluate", s.evaluateFinalExam)
	api.Post("/final-exam/:id/attendance", s.attendFinalExam)
	api.Post("/final-exam/:id/decision", s.decideFinalExam)
	api.Post("/final-exam/:id/revision-approval", s.attachFinalExamRevisionApproval)
	api.Get("/final-exam/:id/report", s.finalExamReport)

	// SOP-018 Capstone Expo
	api.Get("/expo", s.listExpo)
	api.Post("/expo", s.saveExpo)
	api.Put("/expo/:id/review", s.reviewExpo)

	// SOP-019 Asisten Pedoman RAG
	api.Get("/assistant/sources", s.listKnowledgeSources)
	api.Post("/assistant/sources", s.saveKnowledgeSource)
	api.Post("/assistant/sources/:id/index", s.indexKnowledgeSource)
	api.Post("/assistant/sources/:id/test", s.testKnowledgeSource)
	api.Post("/assistant/sources/:id/activate", s.activateKnowledgeSource)
	api.Get("/assistant/sessions", s.listChatSessions)
	api.Post("/assistant/chat", s.askAssistant)

	// SOP-020 Monitoring & Pelaporan
	api.Get("/monitoring/report", s.monitoringReport)
	api.Get("/monitoring/export", s.exportMonitoring)
	return app
}
func (s *Server) remote(ctx context.Context, method, base, path, token string, data any, out any) error {
	var body io.Reader
	if data != nil {
		b, e := json.Marshal(data)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, base+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("X-Service-Token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, e := s.Client.Do(req)
	if e != nil {
		return fiber.NewError(503, "Layanan akun belum tersedia.")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return fiber.ErrUnauthorized
	}
	if resp.StatusCode == 404 {
		return fiber.ErrNotFound
	}
	if resp.StatusCode != 200 {
		return fiber.ErrServiceUnavailable
	}
	if out != nil {
		if e = json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(out); e != nil {
			return fiber.ErrServiceUnavailable
		}
	}
	return nil
}
func (s *Server) identity(ctx context.Context, id string, role string) (domain.User, error) {
	if !domain.ValidID(id) {
		return domain.User{}, fiber.NewError(422, "Identitas tidak valid.")
	}
	var u domain.User
	err := s.remote(ctx, "GET", s.UserURL, "/internal/users/"+id, s.UserToken, nil, &u)
	if err != nil {
		return u, err
	}
	if !u.IsActive || (role != "" && !u.Has(role)) {
		return u, fiber.NewError(422, "Pengguna tidak aktif atau perannya tidak sesuai.")
	}
	return u, nil
}
func (s *Server) authenticated(c fiber.Ctx) error {
	token := c.Cookies("capstone_session")
	if len(token) != 43 {
		return fiber.ErrUnauthorized
	}
	var result struct {
		UserID string `json:"user_id"`
	}
	if e := s.remote(c.Context(), "POST", s.AuthURL, "/internal/sessions/verify", s.AuthToken, map[string]string{"token": token}, &result); e != nil {
		return e
	}
	u, e := s.identity(c.Context(), result.UserID, "")
	if e != nil {
		return e
	}
	c.Locals("user", u)
	return c.Next()
}
func actor(c fiber.Ctx) domain.User { return c.Locals("user").(domain.User) }
func require(c fiber.Ctx, roles ...string) error {
	if !actor(c).Has(roles...) {
		return fiber.NewError(403, "Peran Anda tidak diizinkan melakukan tindakan ini.")
	}
	return nil
}
func (s *Server) directory(c fiber.Ctx) error {
	role := c.Query("role")
	if role != "MAHASISWA" && role != "DOSEN" {
		return fiber.ErrBadRequest
	}
	var out any
	e := s.remote(c.Context(), "GET", s.UserURL, "/internal/directory?"+string(c.Request().URI().QueryString()), s.UserToken, nil, &out)
	if e != nil {
		return e
	}
	return c.JSON(out)
}
func audit(ctx context.Context, tx pgx.Tx, u domain.User, projectID, action string, detail any) error {
	b, e := json.Marshal(detail)
	if e != nil {
		return e
	}
	var pid *string
	if projectID != "" {
		pid = &projectID
	}
	_, e = tx.Exec(ctx, "INSERT INTO capstone_service.audit_logs(actor_id,project_id,action,detail) VALUES($1,$2,$3,$4)", u.ID, pid, action, b)
	return e
}
func stale(actual, want int) error {
	if actual != want {
		return fiber.NewError(409, "Data telah berubah. Muat ulang sebelum menyimpan kembali.")
	}
	return nil
}
