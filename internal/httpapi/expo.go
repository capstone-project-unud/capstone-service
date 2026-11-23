package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/storage"
	"errors"
	"mime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func categoryDisplayLabel(cat string) string {
	switch cat {
	case "SOFTWARE":
		return "Software & Aplikasi"
	case "HARDWARE":
		return "Hardware & Produk Alat"
	case "ALGORITMA":
		return "Algoritma & Pemodelan"
	case "DOKUMEN_STRATEGIS":
		return "Dokumen Strategis & Simulasi"
	default:
		return cat
	}
}

func (s *Server) publicExpo(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil || !ok {
		return c.JSON(fiber.Map{"items": []fiber.Map{}})
	}
	rows, e := s.DB.Query(c.Context(), `
		SELECT ex.id,ex.project_id,coalesce(cg.code,'CAPSTONE'),coalesce(ap.name,'Periode Aktif'),
		       ex.title,ex.category,i.name,ex.abstract_id,ex.abstract_en,p.summary,
		       ex.poster_file_id,ex.video_url,ex.repo_url,ex.demo_url,ex.journal_title,ex.journal_class,ex.published_at
		FROM capstone_service.expo_submissions ex
		JOIN capstone_service.projects p ON p.id=ex.project_id
		JOIN capstone_service.interest_fields i ON i.id=p.interest_id
		LEFT JOIN capstone_service.capstone_groups cg ON cg.id=p.group_id
		LEFT JOIN capstone_service.academic_periods ap ON ap.id=coalesce(p.academic_period_id, cg.academic_period_id)
		WHERE ex.status='PUBLISHED'
		ORDER BY ex.published_at DESC NULLS LAST, ex.updated_at DESC LIMIT 200`)
	if e != nil {
		return e
	}
	defer rows.Close()
	type member struct {
		Name           string `json:"name"`
		NIM            string `json:"nim"`
		IsLeader       bool   `json:"is_leader"`
		Responsibility string `json:"responsibility"`
	}
	type item struct {
		ID                 string         `json:"id"`
		ProjectID          string         `json:"project_id"`
		GroupCode          string         `json:"group_code"`
		AcademicPeriodName string         `json:"academic_period_name"`
		Title              string         `json:"title"`
		Category           string         `json:"category"`
		CategoryLabel      string         `json:"category_label"`
		InterestName       string         `json:"interest_name"`
		AbstractID         string         `json:"abstract_id"`
		AbstractEN         string         `json:"abstract_en"`
		Supervisors        [2]string      `json:"supervisors"`
		Members            []member       `json:"members"`
		Poster             map[string]any `json:"poster"`
		PosterFileID       *string        `json:"poster_file_id"`
		VideoURL           string         `json:"video_url"`
		RepoURL            string         `json:"repo_url"`
		DemoURL            string         `json:"demo_url"`
		JournalTitle       string         `json:"journal_title"`
		JournalClass       string         `json:"journal_class"`
		PublishedAt        string         `json:"published_at"`
		IsMockPreview      bool           `json:"is_mock_preview"`
	}
	items := []item{}
	for rows.Next() {
		var it item
		var projSummary string
		var pubAt *time.Time
		if e = rows.Scan(&it.ID, &it.ProjectID, &it.GroupCode, &it.AcademicPeriodName, &it.Title, &it.Category, &it.InterestName, &it.AbstractID, &it.AbstractEN, &projSummary, &it.PosterFileID, &it.VideoURL, &it.RepoURL, &it.DemoURL, &it.JournalTitle, &it.JournalClass, &pubAt); e != nil {
			return e
		}
		it.CategoryLabel = categoryDisplayLabel(it.Category)
		if pubAt != nil {
			it.PublishedAt = pubAt.Format("2006-01-02")
		}
		it.Members = []member{}
		it.Poster = map[string]any{
			"problem_summary":      projSummary,
			"architecture_nodes":   []string{"Input & Sensor", "Proses Sistem", "Luaran Layanan", "Validasi"},
			"architecture_summary": it.AbstractID,
			"test_highlights": []map[string]any{
				{"label": "Kelulusan Ujian Akhir", "value": "Lulus & Terverifikasi", "score_pct": 100},
			},
			"conclusion": "Karya Capstone Project terpublikasi resmi pada Galeri PSTI Universitas Udayana.",
		}
		items = append(items, it)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	for i := range items {
		pRows, err := s.DB.Query(c.Context(), `SELECT student_name,nim,is_leader,responsibility,slot,coalesce(supervisor_name,''),is_external_supervisor,external_supervisor_name,external_supervisor_institution FROM capstone_service.project_students WHERE project_id=$1 ORDER BY is_leader DESC, nim, slot`, items[i].ProjectID)
		if err != nil {
			return err
		}
		seenMem := map[string]bool{}
		for pRows.Next() {
			var sName, nim, resp, supName, extName, extInst string
			var isLeader, extFlag bool
			var slot int
			if err = pRows.Scan(&sName, &nim, &isLeader, &resp, &slot, &supName, &extFlag, &extName, &extInst); err != nil {
				pRows.Close()
				return err
			}
			if !seenMem[nim+":"+sName] {
				seenMem[nim+":"+sName] = true
				items[i].Members = append(items[i].Members, member{Name: sName, NIM: nim, IsLeader: isLeader, Responsibility: resp})
			}
			if slot == 1 && supName != "" && items[i].Supervisors[0] == "" {
				items[i].Supervisors[0] = supName
			}
			if slot == 2 {
				if supName != "" && items[i].Supervisors[1] == "" {
					items[i].Supervisors[1] = supName
				} else if extFlag && extName != "" && items[i].Supervisors[1] == "" {
					items[i].Supervisors[1] = extName
					if extInst != "" {
						items[i].Supervisors[1] += " (" + extInst + ")"
					}
				}
			}
		}
		pRows.Close()
		if items[i].Supervisors[0] == "" {
			items[i].Supervisors[0] = "Dosen Pembimbing 1 PSTI"
		}
		if items[i].Supervisors[1] == "" {
			items[i].Supervisors[1] = "Dosen Pembimbing 2 PSTI"
		}
		if items[i].JournalClass == "" {
			items[i].JournalClass = "Luaran Publikasi"
		}
		if items[i].JournalTitle == "" {
			items[i].JournalTitle = "Draft Artikel Ilmiah Capstone PSTI Unud"
		}
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) publicExpoPoster(c fiber.Ctx) error {
	id := c.Params("id")
	if !domain.ValidID(id) {
		return fiber.ErrBadRequest
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil || !ok {
		return fiber.ErrNotFound
	}
	var fileID string
	if e = s.DB.QueryRow(c.Context(), `SELECT poster_file_id FROM capstone_service.expo_submissions WHERE id=$1 AND status='PUBLISHED' AND poster_file_id IS NOT NULL`, id).Scan(&fileID); e != nil {
		return fiber.ErrNotFound
	}
	var name, m, key string
	if e = s.DB.QueryRow(c.Context(), `SELECT name,mime,storage_key FROM capstone_service.files WHERE id=$1`, fileID).Scan(&name, &m, &key); e != nil {
		return e
	}
	b, e := s.Files.Get(c.Context(), key)
	if e != nil {
		return fiber.NewError(503, "Poster belum dapat diunduh.")
	}
	if len(b) > storage.MaxFile {
		return fiber.ErrRequestEntityTooLarge
	}
	c.Set("Content-Type", m)
	c.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": name}))
	return c.Send(b)
}

func (s *Server) listExpo(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.ExpoSubmission{}})
	}
	u := actor(c)
	onlyPublished := c.Query("published") == "true"
	rows, e := s.DB.Query(c.Context(), `
		SELECT ex.id,ex.project_id,p.title,i.name,ex.title,ex.category,ex.abstract_id,ex.abstract_en,ex.poster_file_id,ex.video_url,ex.repo_url,ex.demo_url,
		       ex.journal_title,ex.journal_class,ex.journal_url,ex.publication_consent,ex.status,ex.review_note,ex.reviewed_at,ex.published_at,ex.version,ex.updated_at
		FROM capstone_service.expo_submissions ex
		JOIN capstone_service.projects p ON p.id=ex.project_id
		JOIN capstone_service.interest_fields i ON i.id=p.interest_id
		WHERE (CASE WHEN $1 THEN ex.status='PUBLISHED' ELSE ($2 OR ex.status='PUBLISHED' OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=ex.project_id AND (m.student_id=$3 OR m.supervisor_id=$3))) END)
		ORDER BY ex.published_at DESC NULLS LAST, ex.updated_at DESC LIMIT 200`, onlyPublished, u.Has("ADMIN", "KOORDINATOR_TA", "TU"), u.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []domain.ExpoSubmission{}
	for rows.Next() {
		var it domain.ExpoSubmission
		if e = rows.Scan(&it.ID, &it.ProjectID, &it.ProjectTitle, &it.InterestName, &it.Title, &it.Category, &it.AbstractID, &it.AbstractEN, &it.PosterFileID, &it.VideoURL, &it.RepoURL, &it.DemoURL, &it.JournalTitle, &it.JournalClass, &it.JournalURL, &it.PublicationConsent, &it.Status, &it.ReviewNote, &it.ReviewedAt, &it.PublishedAt, &it.Version, &it.UpdatedAt); e != nil {
			return e
		}
		items = append(items, it)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) saveExpo(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Galeri Expo belum tersedia. Hubungi administrator.")
	}
	var in domain.ExpoInput
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrUnprocessableEntity
	}
	if e := in.Validate(); e != nil {
		return fiber.NewError(422, e.Error())
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
	if in.PosterFileID != nil {
		if e = s.ownedFile(c.Context(), tx, *in.PosterFileID, actor(c).ID, "application/pdf"); e != nil {
			return e
		}
	}
	if in.Submit {
		var passed bool
		_ = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.final_exams WHERE project_id=$1 AND status='PASSED')`, p.ID).Scan(&passed)
		if !passed {
			return fiber.NewError(422, "Pengajuan publikasi Capstone Expo memerlukan status kelulusan Ujian Akhir.")
		}
		var openRevs int
		_ = tx.QueryRow(c.Context(), `SELECT count(*) FROM capstone_service.seminar_revision_items WHERE project_id=$1 AND stage='FINAL_EXAM' AND status <> 'APPROVED'`, p.ID).Scan(&openRevs)
		if openRevs > 0 {
			return fiber.NewError(422, "Selesaikan seluruh revisi Ujian Akhir sebelum mengajukan publikasi Expo.")
		}
	}
	nextStatus := "DRAFT"
	if in.Submit {
		nextStatus = "SUBMITTED"
	}
	var id string
	var ver int
	err := tx.QueryRow(c.Context(), `SELECT id,version FROM capstone_service.expo_submissions WHERE project_id=$1 FOR UPDATE`, p.ID).Scan(&id, &ver)
	if errors.Is(err, pgx.ErrNoRows) {
		id = uuid.NewString()
		_, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.expo_submissions(id,project_id,title,category,abstract_id,abstract_en,poster_file_id,video_url,repo_url,demo_url,journal_title,journal_class,journal_url,publication_consent,status)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			id, p.ID, in.Title, in.Category, in.AbstractID, in.AbstractEN, in.PosterFileID, in.VideoURL, in.RepoURL, in.DemoURL, strings.TrimSpace(in.JournalTitle), strings.TrimSpace(in.JournalClass), in.JournalURL, in.PublicationConsent, nextStatus)
	} else if err == nil {
		if e = stale(ver, in.Version); e != nil {
			return e
		}
		_, e = tx.Exec(c.Context(), `
			UPDATE capstone_service.expo_submissions
			SET title=$2,category=$3,abstract_id=$4,abstract_en=$5,poster_file_id=$6,video_url=$7,repo_url=$8,demo_url=$9,
			    journal_title=$10,journal_class=$11,journal_url=$12,publication_consent=$13,status=$14,version=version+1,updated_at=now()
			WHERE id=$1`,
			id, in.Title, in.Category, in.AbstractID, in.AbstractEN, in.PosterFileID, in.VideoURL, in.RepoURL, in.DemoURL, strings.TrimSpace(in.JournalTitle), strings.TrimSpace(in.JournalClass), in.JournalURL, in.PublicationConsent, nextStatus)
	} else {
		return err
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), p.ID, "EXPO_SAVED", map[string]any{"id": id, "status": nextStatus}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}

func (s *Server) reviewExpo(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Status  string `json:"status"`
		Note    string `json:"note"`
		Version int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || (in.Status != "APPROVED" && in.Status != "PUBLISHED" && in.Status != "REVISION") {
		return fiber.ErrUnprocessableEntity
	}
	if in.Status == "REVISION" && !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Catatan perbaikan materi Expo minimal 5 karakter.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var projectID string
	var ver int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.expo_submissions WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&projectID, &ver); e != nil {
		return e
	}
	if e = stale(ver, in.Version); e != nil {
		return e
	}
	if in.Status == "PUBLISHED" {
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.expo_submissions SET status='PUBLISHED',review_note=$2,reviewed_by=$3,reviewed_at=now(),published_at=now(),version=version+1,updated_at=now() WHERE id=$1`,
			c.Params("id"), strings.TrimSpace(in.Note), actor(c).ID)
	} else {
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.expo_submissions SET status=$2,review_note=$3,reviewed_by=$4,reviewed_at=now(),version=version+1,updated_at=now() WHERE id=$1`,
			c.Params("id"), in.Status, strings.TrimSpace(in.Note), actor(c).ID)
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "EXPO_REVIEWED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
