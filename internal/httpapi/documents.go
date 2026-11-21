package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func loadDocVersions(ctx context.Context, db querier, docID string) ([]domain.DocumentVersion, error) {
	rows, e := db.Query(ctx, `
		SELECT id,document_id,version_no,file_id,file_name,summary,uploaded_by,uploaded_by_name,review_status,review_note,reviewer_name,reviewed_at,created_at
		FROM capstone_service.project_document_versions
		WHERE document_id=$1
		ORDER BY version_no DESC`, docID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	list := []domain.DocumentVersion{}
	for rows.Next() {
		var v domain.DocumentVersion
		if e = rows.Scan(&v.ID, &v.DocumentID, &v.VersionNo, &v.FileID, &v.FileName, &v.Summary, &v.UploadedBy, &v.UploadedByName, &v.ReviewStatus, &v.ReviewNote, &v.ReviewerName, &v.ReviewedAt, &v.CreatedAt); e != nil {
			return nil, e
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (s *Server) listDocuments(c fiber.Ctx) error {
	u := actor(c)
	if !u.Has("MAHASISWA", "DOSEN", "KOORDINATOR_TA", "TU", "ADMIN") {
		return fiber.ErrForbidden
	}
	projectID := c.Query("project_id")
	codeFilter := strings.TrimSpace(c.Query("doc_type_code"))
	if codeFilter == "" {
		codeFilter = strings.TrimSpace(c.Query("series"))
	}
	category := strings.TrimSpace(c.Query("category"))
	stage := strings.TrimSpace(c.Query("stage"))
	if (projectID != "" && !domain.ValidID(projectID)) || (codeFilter != "" && !domain.ValidDocTypeCode(strings.ToUpper(codeFilter))) {
		return fiber.ErrBadRequest
	}
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return c.JSON(fiber.Map{"items": []domain.ProjectDocument{}})
	}
	hasFull, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	global := u.Has("ADMIN", "KOORDINATOR_TA", "TU")
	items := []domain.ProjectDocument{}
	if !hasFull {
		rows, e := s.DB.Query(c.Context(), `
			SELECT d.id,d.project_id,p.title,d.series,d.file_id,coalesce(f.name,''),d.uploaded_by,d.uploaded_by_name,d.summary,d.revision_no,d.status,d.review_note,d.reviewer_name,d.reviewed_at,d.version,d.updated_at
			FROM capstone_service.project_documents d
			JOIN capstone_service.projects p ON p.id=d.project_id
			LEFT JOIN capstone_service.files f ON f.id=d.file_id
			WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=d.project_id AND (m.student_id=$2 OR m.supervisor_id=$2)) OR EXISTS(SELECT 1 FROM capstone_service.proposal_seminars s WHERE s.project_id=d.project_id AND (s.examiner_1_id=$2 OR s.examiner_2_id=$2 OR s.examiner_3_id=$2)))
			  AND ($3='' OR d.project_id::text=$3)
			  AND ($4='' OR d.series=$4)
			ORDER BY d.updated_at DESC, d.series`, global, u.ID, projectID, codeFilter)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var d domain.ProjectDocument
			if e = rows.Scan(&d.ID, &d.ProjectID, &d.ProjectTitle, &d.Series, &d.FileID, &d.FileName, &d.UploadedBy, &d.UploadedByName, &d.Summary, &d.RevisionNo, &d.Status, &d.ReviewNote, &d.ReviewerName, &d.ReviewedAt, &d.Version, &d.UpdatedAt); e != nil {
				return e
			}
			d.DocTypeCode = d.Series
			d.DocTypeName = d.Series
			d.Category = "B_SERIES"
			d.Scope = "GROUP"
			d.Versions = []domain.DocumentVersion{}
			items = append(items, d)
		}
		return c.JSON(fiber.Map{"items": items})
	}

	rows, e := s.DB.Query(c.Context(), `
		SELECT d.id,d.project_id,p.title,d.doc_type_code,coalesce(dt.name,d.doc_type_code),coalesce(dt.category,'B_SERIES'),coalesce(dt.stage,'SEMPRO'),coalesce(dt.scope,'GROUP'),
		       d.student_id,coalesce((SELECT ps.student_name FROM capstone_service.project_students ps WHERE ps.project_id=d.project_id AND ps.student_id=d.student_id LIMIT 1),''),
		       d.series,d.file_id,coalesce(f.name,''),d.uploaded_by,d.uploaded_by_name,d.summary,d.current_version_no,d.status,d.review_note,d.reviewer_name,d.reviewed_at,d.version,d.updated_at
		FROM capstone_service.project_documents d
		JOIN capstone_service.projects p ON p.id=d.project_id
		LEFT JOIN capstone_service.document_types dt ON dt.code=d.doc_type_code
		LEFT JOIN capstone_service.files f ON f.id=d.file_id
		WHERE ($1 OR EXISTS(SELECT 1 FROM capstone_service.project_students m WHERE m.project_id=d.project_id AND (m.student_id=$2 OR m.candidate_id=$2 OR m.supervisor_id=$2))
		          OR EXISTS(SELECT 1 FROM capstone_service.proposal_seminars s WHERE s.project_id=d.project_id AND (s.examiner_1_id=$2 OR s.examiner_2_id=$2 OR s.examiner_3_id=$2))
		          OR EXISTS(SELECT 1 FROM capstone_service.final_exams fe WHERE fe.project_id=d.project_id AND (fe.examiner_1_id=$2 OR fe.examiner_2_id=$2 OR fe.examiner_3_id=$2)))
		  AND ($3='' OR d.project_id::text=$3)
		  AND ($4='' OR d.doc_type_code=$4 OR d.series=$4)
		  AND ($5='' OR dt.category=$5)
		  AND ($6='' OR dt.stage=$6)
		ORDER BY coalesce(dt.sort_order,100) ASC, d.updated_at DESC LIMIT 400`, global, u.ID, projectID, codeFilter, category, stage)
	if e != nil {
		return e
	}
	for rows.Next() {
		var d domain.ProjectDocument
		if e = rows.Scan(&d.ID, &d.ProjectID, &d.ProjectTitle, &d.DocTypeCode, &d.DocTypeName, &d.Category, &d.Stage, &d.Scope, &d.StudentID, &d.StudentName, &d.Series, &d.FileID, &d.FileName, &d.UploadedBy, &d.UploadedByName, &d.Summary, &d.RevisionNo, &d.Status, &d.ReviewNote, &d.ReviewerName, &d.ReviewedAt, &d.Version, &d.UpdatedAt); e != nil {
			rows.Close()
			return e
		}
		items = append(items, d)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for i := range items {
		versions, e := loadDocVersions(c.Context(), s.DB, items[i].ID)
		if e != nil {
			return e
		}
		items[i].Versions = versions
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) saveDocument(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA", "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return fiber.NewError(503, "Dokumen belum tersedia. Hubungi administrator.")
	}
	hasFull, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	var in domain.DocumentInput
	if e := c.Bind().Body(&in); e != nil {
		return fiber.ErrBadRequest
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
	u := actor(c)
	member := false
	for _, r := range p.Students {
		if r.StudentID == u.ID {
			member = true
			break
		}
	}
	if !u.Has("ADMIN", "KOORDINATOR_TA") && !member {
		return fiber.ErrForbidden
	}
	if !hasFull && p.Status != "FINALIZED" {
		return fiber.NewError(422, "Dokumen Seri B dapat diunggah setelah judul ditetapkan.")
	}
	docScope := "GROUP"
	if hasFull {
		var stage string
		var active bool
		e = tx.QueryRow(c.Context(), `SELECT scope,stage,is_active FROM capstone_service.document_types WHERE code=$1`, in.DocTypeCode).Scan(&docScope, &stage, &active)
		if errors.Is(e, pgx.ErrNoRows) {
			return fiber.NewError(422, "Jenis dokumen tidak terdaftar dalam Data Master.")
		}
		if e != nil {
			return e
		}
		if !active {
			return fiber.NewError(422, "Jenis dokumen sedang dinonaktifkan.")
		}
		if stage != "TOPIK" && p.Status != "FINALIZED" {
			return fiber.NewError(422, "Dokumen tahap pelaksanaan dapat diunggah setelah judul ditetapkan.")
		}
		if docScope == "INDIVIDUAL" {
			if in.StudentID == nil {
				if u.Has("MAHASISWA") {
					idCopy := u.ID
					in.StudentID = &idCopy
				} else {
					return fiber.NewError(422, "Pilih mahasiswa pemilik untuk dokumen lingkup individu.")
				}
			}
			validStudent := false
			for _, r := range p.Students {
				if r.StudentID == *in.StudentID {
					validStudent = true
					break
				}
			}
			if !validStudent {
				return fiber.NewError(422, "Pemilik dokumen harus anggota kelompok proyek.")
			}
		} else {
			in.StudentID = nil
		}
	}

	if e = s.ownedFile(c.Context(), tx, in.FileID, u.ID, "application/pdf"); e != nil {
		return e
	}
	var fileName string
	_ = tx.QueryRow(c.Context(), `SELECT name FROM capstone_service.files WHERE id=$1`, in.FileID).Scan(&fileName)

	var id string
	var existingVersion, currentVersionNo int
	if hasFull {
		e = tx.QueryRow(c.Context(), `
			SELECT id,version,current_version_no
			FROM capstone_service.project_documents
			WHERE project_id=$1 AND doc_type_code=$2 AND coalesce(student_id,'00000000-0000-0000-0000-000000000000'::uuid)=coalesce($3,'00000000-0000-0000-0000-000000000000'::uuid)
			FOR UPDATE`, p.ID, in.DocTypeCode, in.StudentID).Scan(&id, &existingVersion, &currentVersionNo)
	} else {
		e = tx.QueryRow(c.Context(), `SELECT id,version,revision_no FROM capstone_service.project_documents WHERE project_id=$1 AND series=$2 FOR UPDATE`, p.ID, in.Series).Scan(&id, &existingVersion, &currentVersionNo)
	}
	newVersionNo := 1
	if errors.Is(e, pgx.ErrNoRows) {
		id = uuid.NewString()
		if hasFull {
			_, e = tx.Exec(c.Context(), `
				INSERT INTO capstone_service.project_documents(id,project_id,doc_type_code,student_id,series,file_id,uploaded_by,uploaded_by_name,summary,revision_no,current_version_no)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,1)`,
				id, p.ID, in.DocTypeCode, in.StudentID, in.DocTypeCode, in.FileID, u.ID, u.FullName, in.Summary)
		} else {
			_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.project_documents(id,project_id,series,file_id,uploaded_by,uploaded_by_name,summary) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, p.ID, in.Series, in.FileID, u.ID, u.FullName, in.Summary)
		}
	} else if e == nil {
		if e = stale(existingVersion, in.Version); e != nil {
			return e
		}
		newVersionNo = currentVersionNo + 1
		if hasFull {
			_, e = tx.Exec(c.Context(), `
				UPDATE capstone_service.project_documents
				SET file_id=$2,uploaded_by=$3,uploaded_by_name=$4,summary=$5,revision_no=$6,current_version_no=$6,status='PENDING',review_note='',reviewer_id=NULL,reviewer_name='',reviewed_at=NULL,version=version+1,updated_at=now()
				WHERE id=$1`, id, in.FileID, u.ID, u.FullName, in.Summary, newVersionNo)
		} else {
			_, e = tx.Exec(c.Context(), `UPDATE capstone_service.project_documents SET file_id=$2,uploaded_by=$3,uploaded_by_name=$4,summary=$5,revision_no=revision_no+1,status='PENDING',review_note='',reviewer_id=NULL,reviewer_name='',reviewed_at=NULL,version=version+1,updated_at=now() WHERE id=$1`, id, in.FileID, u.ID, u.FullName, in.Summary)
		}
	}
	if e != nil {
		return e
	}
	if hasFull {
		if _, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.project_document_versions(id,document_id,version_no,file_id,file_name,summary,uploaded_by,uploaded_by_name,review_status)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,'PENDING')`,
			uuid.NewString(), id, newVersionNo, in.FileID, fileName, in.Summary, u.ID, u.FullName); e != nil {
			return e
		}
	}
	if e = audit(c.Context(), tx, u, p.ID, "DOCUMENT_UPLOADED", map[string]any{"id": id, "doc_type_code": in.DocTypeCode, "version_no": newVersionNo, "file_id": in.FileID}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id, "version_no": newVersionNo})
}

func (s *Server) reviewDocument(c fiber.Ctx) error {
	if e := require(c, "DOSEN", "KOORDINATOR_TA", "ADMIN", "TU"); e != nil {
		return e
	}
	hasSchema, e := hasSemproSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !hasSchema {
		return fiber.NewError(503, "Dokumen belum tersedia. Hubungi administrator.")
	}
	hasFull, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	id := c.Params("id")
	if !domain.ValidID(id) {
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
	if in.Status != "APPROVED" && in.Status != "REVISION" {
		return fiber.ErrUnprocessableEntity
	}
	if in.Status == "REVISION" && !domain.Text(in.Note, 3, 2000) {
		return fiber.NewError(422, "Catatan perbaikan wajib diisi minimal 3 karakter.")
	}
	if len(in.Note) > 2000 {
		return fiber.NewError(422, "Catatan maksimal 2000 karakter.")
	}

	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())

	var projectID, docCategory string
	var version, verNo int
	if hasFull {
		e = tx.QueryRow(c.Context(), `
			SELECT d.project_id,d.version,d.current_version_no,coalesce(dt.category,'B_SERIES')
			FROM capstone_service.project_documents d
			LEFT JOIN capstone_service.document_types dt ON dt.code=d.doc_type_code
			WHERE d.id=$1 FOR UPDATE OF d`, id).Scan(&projectID, &version, &verNo, &docCategory)
	} else {
		docCategory = "B_SERIES"
		e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.project_documents WHERE id=$1 FOR UPDATE`, id).Scan(&projectID, &version)
	}
	if e != nil {
		return e
	}
	u := actor(c)
	if u.Has("TU") && !u.Has("KOORDINATOR_TA", "ADMIN") && (docCategory == "B_SERIES" || docCategory == "PROPOSAL_REPORT") {
		return fiber.ErrForbidden
	}
	if !u.Has("KOORDINATOR_TA", "ADMIN", "TU") {
		var isSupervisor bool
		if e = tx.QueryRow(c.Context(), `SELECT EXISTS(SELECT 1 FROM capstone_service.project_students WHERE project_id=$1 AND supervisor_id=$2)`, projectID, u.ID).Scan(&isSupervisor); e != nil {
			return e
		}
		if !isSupervisor {
			return fiber.ErrForbidden
		}
	}
	if e = stale(version, in.Version); e != nil {
		return e
	}
	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.project_documents SET status=$2,review_note=$3,reviewer_id=$4,reviewer_name=$5,reviewed_at=now(),version=version+1,updated_at=now() WHERE id=$1`, id, in.Status, in.Note, actor(c).ID, actor(c).FullName)
	if e != nil {
		return e
	}
	if hasFull {
		_, e = tx.Exec(c.Context(), `
			UPDATE capstone_service.project_document_versions
			SET review_status=$3,review_note=$4,reviewer_id=$5,reviewer_name=$6,reviewed_at=now()
			WHERE document_id=$1 AND version_no=$2`, id, verNo, in.Status, in.Note, actor(c).ID, actor(c).FullName)
		if e != nil {
			return e
		}
	}
	if e = audit(c.Context(), tx, actor(c), projectID, "DOCUMENT_REVIEWED", map[string]any{"id": id, "status": in.Status, "note": in.Note}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
