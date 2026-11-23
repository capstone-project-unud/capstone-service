package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/storage"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

func (s *Server) ownedFile(ctx context.Context, db querier, id, owner, requiredMime string) error {
	if !domain.ValidID(id) {
		return fiber.ErrUnprocessableEntity
	}
	var uid, m string
	if e := db.QueryRow(ctx, "SELECT owner_id,mime FROM capstone_service.files WHERE id=$1", id).Scan(&uid, &m); e != nil {
		return e
	}
	if uid != owner {
		return fiber.ErrForbidden
	}
	if requiredMime != "" && m != requiredMime {
		return fiber.NewError(422, "Berkas harus berupa PDF.")
	}
	return nil
}

func (s *Server) projectOrOwnedFile(ctx context.Context, db querier, id, owner, projectID, requiredMime string) error {
	if !domain.ValidID(id) {
		return fiber.ErrUnprocessableEntity
	}
	var uid, m string
	if e := db.QueryRow(ctx, "SELECT owner_id,mime FROM capstone_service.files WHERE id=$1", id).Scan(&uid, &m); e != nil {
		return e
	}
	if uid != owner {
		linked := false
		if projectID != "" {
			if hasSchema, err := hasSemproSchema(ctx, db); err == nil && hasSchema {
				_ = db.QueryRow(ctx, `
					SELECT EXISTS(SELECT 1 FROM capstone_service.project_documents d WHERE d.project_id=$1 AND d.file_id=$2)
					    OR EXISTS(SELECT 1 FROM capstone_service.project_document_versions v JOIN capstone_service.project_documents d ON d.id=v.document_id WHERE d.project_id=$1 AND v.file_id=$2)`,
					projectID, id).Scan(&linked)
			}
		}
		if !linked {
			return fiber.ErrForbidden
		}
	}
	if requiredMime != "" && m != requiredMime {
		return fiber.NewError(422, "Berkas harus berupa PDF.")
	}
	return nil
}
func (s *Server) upload(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA", "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	f, e := c.FormFile("file")
	if e != nil {
		return fiber.NewError(422, "Pilih berkas PDF, PNG, atau JPEG.")
	}
	if f.Size < 1 || f.Size > storage.MaxFile {
		return fiber.NewError(422, "Ukuran berkas maksimal 10 MB.")
	}
	stream, e := f.Open()
	if e != nil {
		return e
	}
	defer stream.Close()
	b, e := io.ReadAll(io.LimitReader(stream, storage.MaxFile+1))
	if e != nil {
		return e
	}
	if len(b) > storage.MaxFile {
		return fiber.ErrRequestEntityTooLarge
	}
	contentType := http.DetectContentType(b)
	ext := strings.ToLower(filepath.Ext(f.Filename))
	valid := (contentType == "application/pdf" && ext == ".pdf") || (contentType == "image/png" && ext == ".png") || (contentType == "image/jpeg" && (ext == ".jpg" || ext == ".jpeg"))
	if !valid {
		return fiber.NewError(422, "Isi dan ekstensi berkas harus sesuai: PDF, PNG, atau JPEG.")
	}
	name := filepath.Base(strings.ReplaceAll(f.Filename, "\\", "/"))
	if !domain.Text(name, 1, 200) || strings.ContainsAny(name, "\r\n\x00") {
		return fiber.ErrUnprocessableEntity
	}
	id := uuid.NewString()
	key := id + ext
	if e = s.Files.Put(c.Context(), key, b, contentType); e != nil {
		return fiber.NewError(503, "Berkas belum tersimpan. Silakan coba kembali.")
	}
	_, e = s.DB.Exec(c.Context(), "INSERT INTO capstone_service.files(id,owner_id,name,mime,size,storage_key) VALUES($1,$2,$3,$4,$5,$6)", id, actor(c).ID, name, contentType, len(b), key)
	if e != nil {
		_ = s.Files.Delete(c.Context(), key)
		return e
	}
	return c.Status(201).JSON(fiber.Map{"file": fiber.Map{"id": id, "name": name, "mime": contentType, "size": len(b)}})
}
func (s *Server) download(c fiber.Ctx) error {
	id := c.Params("id")
	if !domain.ValidID(id) {
		return fiber.ErrBadRequest
	}
	var owner, name, m, key string
	if e := s.DB.QueryRow(c.Context(), "SELECT owner_id,name,mime,storage_key FROM capstone_service.files WHERE id=$1", id).Scan(&owner, &name, &m, &key); e != nil {
		return e
	}
	u := actor(c)
	allowed := owner == u.ID
	if !allowed {
		hasSchema, e := hasSemproSchema(c.Context(), s.DB)
		if e != nil {
			return e
		}
		hasFull, e := hasFullSOPSchema(c.Context(), s.DB)
		if e != nil {
			return e
		}
		var related bool
		fileQuery := `
			SELECT EXISTS(SELECT 1 FROM capstone_service.projects p WHERE p.description_file_id=$1 AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students d WHERE d.project_id=p.id AND d.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.proposal_applications a WHERE (a.simak_file_id=$1 OR a.approval_file_id=$1) AND (a.student_id=$3 OR $4 OR EXISTS(SELECT 1 FROM capstone_service.project_students d WHERE d.project_id=a.project_id AND d.student_id=a.student_id AND d.supervisor_id=$3)))`
		if hasSchema {
			fileQuery += `
			    OR EXISTS(SELECT 1 FROM capstone_service.project_documents d WHERE d.file_id=$1 AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=d.project_id AND m.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.proposal_seminars s WHERE s.revision_file_id=$1 AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=s.project_id AND m.student_id=$3)))`
		}
		if hasFull {
			fileQuery += `
			    OR EXISTS(SELECT 1 FROM capstone_service.project_document_versions v JOIN capstone_service.project_documents d ON d.id=v.document_id WHERE v.file_id=$1 AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=d.project_id AND m.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.projects p WHERE p.assignment_letter_file_id=$1 AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=p.id AND m.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.proposal_seminars s WHERE (s.minutes_file_id=$1 OR s.revision_approved_file_id=$1) AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=s.project_id AND m.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.seminar_revision_items r WHERE r.response_file_id=$1 AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=r.project_id AND m.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.final_exam_applications fa WHERE ($1 IN (fa.transcript_file_id, fa.spp_file_id, fa.ijazah_file_id, fa.photo_file_id, fa.pa_letter_file_id, fa.plagiarism_file_id)) AND (fa.student_id=$3 OR $4 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=fa.project_id AND m.student_id=fa.student_id AND m.supervisor_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.final_exams fe WHERE ($1 IN (fe.minutes_file_id, fe.revision_approved_file_id)) AND ($2 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=fe.project_id AND m.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.expo_submissions ex WHERE ex.poster_file_id=$1 AND (ex.status='PUBLISHED' OR $4 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=ex.project_id AND m.student_id=$3)))
			    OR EXISTS(SELECT 1 FROM capstone_service.knowledge_sources ks WHERE ks.source_file_id=$1 AND (ks.status='ACTIVE' OR $4))`
		}
		if revisionSchema(c.Context(), s.DB) == nil {
			fileQuery += ` OR EXISTS(SELECT 1 FROM capstone_service.proposal_documents pd JOIN capstone_service.proposal_applications a ON a.id=pd.application_id WHERE pd.file_id=$1 AND (a.student_id=$3 OR $2))`
		}
		e = s.DB.QueryRow(c.Context(), fileQuery, id, u.Staff(), u.ID, u.Has("TU", "KOORDINATOR_TA", "ADMIN")).Scan(&related)
		if e != nil {
			return e
		}
		allowed = related
	}
	if !allowed {
		return fiber.ErrForbidden
	}
	b, e := s.Files.Get(c.Context(), key)
	if e != nil {
		return fiber.NewError(503, "Berkas belum dapat diunduh.")
	}
	if len(b) > storage.MaxFile {
		return fiber.ErrRequestEntityTooLarge
	}
	c.Set("Content-Type", m)
	c.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	return c.Send(b)
}
func (s *Server) interests(c fiber.Ctx) error {
	rows, e := s.DB.Query(c.Context(), "SELECT id,name,is_active FROM capstone_service.interest_fields ORDER BY name")
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []fiber.Map{}
	for rows.Next() {
		var id, name string
		var active bool
		if e = rows.Scan(&id, &name, &active); e != nil {
			return e
		}
		items = append(items, fiber.Map{"id": id, "name": name, "is_active": active})
	}
	if e = rows.Err(); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items})
}
func (s *Server) saveInterest(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	var in struct {
		Name     string `json:"name"`
		IsActive bool   `json:"is_active"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Name, 2, 150) {
		return fiber.NewError(422, "Nama bidang minat 2–150 karakter wajib diisi.")
	}
	id := c.Params("id")
	var e error
	if id == "" {
		id = uuid.NewString()
		_, e = s.DB.Exec(c.Context(), "INSERT INTO capstone_service.interest_fields(id,name,is_active) VALUES($1,$2,$3)", id, strings.TrimSpace(in.Name), in.IsActive)
	} else {
		if !domain.ValidID(id) {
			return fiber.ErrBadRequest
		}
		tag, err := s.DB.Exec(c.Context(), "UPDATE capstone_service.interest_fields SET name=$2,is_active=$3 WHERE id=$1", id, strings.TrimSpace(in.Name), in.IsActive)
		e = err
		if e == nil && tag.RowsAffected() == 0 {
			return fiber.ErrNotFound
		}
	}
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id, "message": fmt.Sprintf("Bidang minat %s disimpan.", in.Name)})
}
