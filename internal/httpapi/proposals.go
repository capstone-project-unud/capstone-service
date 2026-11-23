package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/reports"
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type ProposalDocument struct {
	Kind       string     `json:"kind"`
	FileID     string     `json:"file_id"`
	Status     string     `json:"status"`
	Note       string     `json:"note"`
	Reviewer   *string    `json:"reviewer"`
	ReviewedAt *time.Time `json:"reviewed_at"`
}
type Proposal struct {
	ID                    string             `json:"id"`
	ProjectID             string             `json:"project_id"`
	Title                 string             `json:"title"`
	InterestID            string             `json:"interest_id"`
	InterestName          string             `json:"interest_name"`
	StudentID             string             `json:"student_id"`
	StudentName           string             `json:"student_name"`
	NIM                   string             `json:"nim"`
	Supervisor1           string             `json:"supervisor_1_name"`
	Supervisor2           string             `json:"supervisor_2_name"`
	GuidanceApprovedCount int                `json:"guidance_approved_count"`
	SupervisorCount       int                `json:"supervisor_count"`
	SimakFileID           string             `json:"simak_file_id"`
	ApprovalFileID        string             `json:"approval_file_id"`
	SimakStatus           string             `json:"simak_status"`
	ApprovalStatus        string             `json:"approval_status"`
	Documents             []ProposalDocument `json:"documents"`
	Version               int                `json:"version"`
	SubmittedAt           time.Time          `json:"submitted_at"`
	Ready                 bool               `json:"ready"`
}

func revisionSchema(ctx context.Context, db querier) error {
	var ok bool
	if e := db.QueryRow(ctx, `SELECT to_regclass('capstone_service.proposal_exams') IS NOT NULL`).Scan(&ok); e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Pendaftaran dan ujian proposal belum tersedia. Hubungi administrator.")
	}
	return nil
}
func loadProposals(ctx context.Context, db querier, u domain.User, interest, validity string) ([]Proposal, error) {
	if interest != "" && !domain.ValidID(interest) {
		return nil, fiber.ErrBadRequest
	}
	if validity != "" && validity != "ready" && validity != "pending" && validity != "revision" {
		return nil, fiber.ErrBadRequest
	}
	rows, e := db.Query(ctx, `SELECT a.id,a.project_id,p.title,p.interest_id,i.name,a.student_id,m.student_name,m.nim,
 coalesce(m.supervisor_name,''),coalesce(nullif(m2.supervisor_name,''), CASE WHEN m2.is_external_supervisor AND m2.external_supervisor_name<>'' THEN m2.external_supervisor_name ELSE '' END,''),
 coalesce((SELECT count(*) FROM capstone_service.guidance_logs gl WHERE gl.project_id=a.project_id AND gl.student_id=a.student_id AND gl.status='APPROVED'),0),
 coalesce((SELECT count(*) FROM capstone_service.project_students ps WHERE ps.project_id=a.project_id AND ps.student_id=a.student_id AND (ps.supervisor_id IS NOT NULL OR (ps.slot=2 AND ps.is_external_supervisor AND ps.external_supervisor_name<>''))),0),
 a.simak_file_id,a.approval_file_id,a.simak_status,a.approval_status,a.version,a.submitted_at,
 coalesce((SELECT jsonb_agg(jsonb_build_object('kind',d.kind,'file_id',d.file_id,'status',d.status,'note',d.note,'reviewer',d.reviewer,'reviewed_at',d.reviewed_at) ORDER BY d.kind) FROM capstone_service.proposal_documents d WHERE d.application_id=a.id),'[]'::jsonb)
 FROM capstone_service.proposal_applications a JOIN capstone_service.projects p ON p.id=a.project_id JOIN capstone_service.interest_fields i ON i.id=p.interest_id
 JOIN capstone_service.project_students m ON m.project_id=a.project_id AND m.student_id=a.student_id AND m.slot=1
 LEFT JOIN capstone_service.project_students m2 ON m2.project_id=a.project_id AND m2.student_id=a.student_id AND m2.slot=2
 WHERE ($1 OR a.student_id=$2) AND ($3='' OR p.interest_id::text=$3) ORDER BY a.submitted_at DESC LIMIT 10001`, u.Staff(), u.ID, interest)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []Proposal{}
	count := 0
	for rows.Next() {
		count++
		var a Proposal
		var raw []byte
		if e = rows.Scan(&a.ID, &a.ProjectID, &a.Title, &a.InterestID, &a.InterestName, &a.StudentID, &a.StudentName, &a.NIM, &a.Supervisor1, &a.Supervisor2, &a.GuidanceApprovedCount, &a.SupervisorCount, &a.SimakFileID, &a.ApprovalFileID, &a.SimakStatus, &a.ApprovalStatus, &a.Version, &a.SubmittedAt, &raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &a.Documents); e != nil {
			return nil, e
		}
		a.Ready = len(a.Documents) == 6
		revision := false
		for _, d := range a.Documents {
			a.Ready = a.Ready && d.Status == "VALID"
			revision = revision || d.Status == "REVISION"
		}
		if validity == "ready" && !a.Ready || validity == "pending" && a.Ready || validity == "revision" && !revision {
			continue
		}
		items = append(items, a)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if count > 10000 {
		return nil, fiber.NewError(422, "Persempit bidang minat untuk mengunduh maksimal 10000 pendaftaran.")
	}
	return items, nil
}
func (s *Server) listProposals(c fiber.Ctx) error {
	if !actor(c).Has("MAHASISWA", "DOSEN", "TU", "ADMIN", "KOORDINATOR_TA", "KOPRODI") {
		return fiber.ErrForbidden
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	items, e := loadProposals(c.Context(), s.DB, actor(c), c.Query("interest_id"), c.Query("validity"))
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items})
}
func (s *Server) exportProposals(c fiber.Ctx) error {
	if !actor(c).Staff() {
		return fiber.ErrForbidden
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	items, e := loadProposals(c.Context(), s.DB, actor(c), c.Query("interest_id"), c.Query("validity"))
	if e != nil {
		return e
	}
	rows := [][]string{{"NIM", "Nama", "Judul", "P1", "P2", "Bidang minat", "Status kelengkapan", "SIMAK", "Persetujuan pembimbing", "Bahasa Inggris", "Proposal", "B100", "B200"}}
	for _, a := range items {
		status := "Belum valid"
		if a.Ready {
			status = "Valid"
		}
		row := []string{a.NIM, a.StudentName, a.Title, a.Supervisor1, a.Supervisor2, a.InterestName, status}
		for _, k := range domain.ProposalKinds {
			v := "MISSING"
			for _, d := range a.Documents {
				if d.Kind == k {
					v = d.Status
				}
			}
			row = append(row, v)
		}
		rows = append(rows, row)
	}
	b, e := reports.XLSX(rows)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", `attachment; filename="pendaftaran-proposal.xlsx"`)
	return c.Send(b)
}
func (s *Server) saveProposal(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA"); e != nil {
		return e
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	var in struct {
		ProjectID string `json:"project_id"`
		Simak     string `json:"simak_file_id"`
		Approval  string `json:"approval_file_id"`
		English   string `json:"english_file_id"`
		Proposal  string `json:"proposal_file_id"`
		B100      string `json:"b100_file_id"`
		B200      string `json:"b200_file_id"`
		Version   int    `json:"version"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidID(in.ProjectID) {
		return fiber.ErrUnprocessableEntity
	}
	ids := []string{in.Simak, in.Approval, in.English, in.Proposal, in.B100, in.B200}
	seen := map[string]bool{}
	for _, id := range ids {
		if !domain.ValidID(id) || seen[id] {
			return fiber.NewError(422, "Enam berkas persyaratan yang berbeda wajib diunggah.")
		}
		seen[id] = true
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
	supervisors := 0
	for _, m := range p.Students {
		if m.StudentID == actor(c).ID && m.SupervisorID != nil {
			supervisors++
		}
	}
	if p.Status != "FINALIZED" || supervisors != 2 {
		return fiber.NewError(422, "Judul dan dua pembimbing Anda harus ditetapkan sebelum mendaftar.")
	}
	for i, id := range ids {
		mime := "application/pdf"
		if i == 0 || i == 2 {
			mime = ""
		}
		if i >= 3 {
			e = s.projectOrOwnedFile(c.Context(), tx, id, actor(c).ID, p.ID, mime)
		} else {
			e = s.ownedFile(c.Context(), tx, id, actor(c).ID, mime)
		}
		if e != nil {
			return e
		}
	}
	id := uuid.NewString()
	version := 0
	e = tx.QueryRow(c.Context(), `SELECT id,version FROM capstone_service.proposal_applications WHERE project_id=$1 AND student_id=$2 FOR UPDATE`, p.ID, actor(c).ID).Scan(&id, &version)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	exists := e == nil
	if exists {
		if e = stale(version, in.Version); e != nil {
			return e
		}
		if e = unrecordedProposal(c.Context(), tx, id); e != nil {
			return e
		}
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_applications SET simak_file_id=$2,approval_file_id=$3,version=version+1,submitted_at=now() WHERE id=$1`, id, in.Simak, in.Approval)
	} else {
		if in.Version != 0 {
			return fiber.ErrConflict
		}
		_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.proposal_applications(id,project_id,student_id,simak_file_id,approval_file_id) VALUES($1,$2,$3,$4,$5)`, id, p.ID, actor(c).ID, in.Simak, in.Approval)
	}
	if e != nil {
		return e
	}
	changed := int64(0)
	for i, k := range domain.ProposalKinds {
		tag, err := tx.Exec(c.Context(), `INSERT INTO capstone_service.proposal_documents(application_id,kind,file_id) VALUES($1,$2,$3)
 ON CONFLICT(application_id,kind) DO UPDATE SET file_id=excluded.file_id,status='PENDING',note='',reviewer=NULL,reviewed_at=NULL WHERE proposal_documents.file_id<>excluded.file_id`, id, k, ids[i])
		if err != nil {
			return err
		}
		changed += tag.RowsAffected()
	}
	if changed == 0 {
		return fiber.NewError(422, "Unggah berkas perbaikan sebelum mengajukan ulang.")
	}
	for _, k := range []string{"simak", "approval"} {
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_applications a SET `+k+`_status=d.status,`+k+`_note=d.note,`+k+`_reviewer=d.reviewer,`+k+`_reviewed_at=d.reviewed_at FROM capstone_service.proposal_documents d WHERE a.id=$1 AND d.application_id=a.id AND d.kind=$2`, id, k)
		if e != nil {
			return e
		}
	}
	if hasFull, err := hasFullSOPSchema(c.Context(), tx); err == nil && hasFull {
		syncDocs := []struct{ code, fileID string }{
			{"PROPOSAL_USULAN", in.Proposal},
			{"B-100", in.B100},
			{"B-200", in.B200},
		}
		u := actor(c)
		const syncSummary = "Disinkronkan dari pendaftaran ujian proposal"
		for _, sd := range syncDocs {
			var docID, curFile string
			var curVerNo int
			err := tx.QueryRow(c.Context(), `SELECT id,file_id::text,current_version_no FROM capstone_service.project_documents WHERE project_id=$1 AND doc_type_code=$2 AND student_id IS NULL FOR UPDATE`, p.ID, sd.code).Scan(&docID, &curFile, &curVerNo)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			var fileName string
			_ = tx.QueryRow(c.Context(), `SELECT name FROM capstone_service.files WHERE id=$1`, sd.fileID).Scan(&fileName)
			if errors.Is(err, pgx.ErrNoRows) {
				docID = uuid.NewString()
				if _, err = tx.Exec(c.Context(), `INSERT INTO capstone_service.project_documents(id,project_id,doc_type_code,student_id,series,file_id,uploaded_by,uploaded_by_name,summary,revision_no,current_version_no) VALUES($1,$2,$3,NULL,$3,$4,$5,$6,$7,1,1)`,
					docID, p.ID, sd.code, sd.fileID, u.ID, u.FullName, syncSummary); err != nil {
					return err
				}
				if _, err = tx.Exec(c.Context(), `INSERT INTO capstone_service.project_document_versions(id,document_id,version_no,file_id,file_name,summary,uploaded_by,uploaded_by_name,review_status) VALUES($1,$2,1,$3,$4,$5,$6,$7,'PENDING')`,
					uuid.NewString(), docID, sd.fileID, fileName, syncSummary, u.ID, u.FullName); err != nil {
					return err
				}
			} else if curFile != sd.fileID {
				curVerNo++
				if _, err = tx.Exec(c.Context(), `UPDATE capstone_service.project_documents SET file_id=$2,uploaded_by=$3,uploaded_by_name=$4,summary=$5,revision_no=$6,current_version_no=$6,status='PENDING',review_note='',reviewer_id=NULL,reviewer_name='',reviewed_at=NULL,version=version+1,updated_at=now() WHERE id=$1`,
					docID, sd.fileID, u.ID, u.FullName, syncSummary, curVerNo); err != nil {
					return err
				}
				if _, err = tx.Exec(c.Context(), `INSERT INTO capstone_service.project_document_versions(id,document_id,version_no,file_id,file_name,summary,uploaded_by,uploaded_by_name,review_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'PENDING')`,
					uuid.NewString(), docID, curVerNo, sd.fileID, fileName, syncSummary, u.ID, u.FullName); err != nil {
					return err
				}
			}
		}
	}
	if e = audit(c.Context(), tx, actor(c), p.ID, "PROPOSAL_SUBMITTED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}
func unrecordedProposal(ctx context.Context, db querier, id string) error {
	var used bool
	if e := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM capstone_service.proposal_exams WHERE application_id=$1)`, id).Scan(&used); e != nil {
		return e
	}
	if used {
		return fiber.NewError(409, "Berkas terkunci karena informasi ujian telah dicatat.")
	}
	return nil
}
func (s *Server) reviewProposal(c fiber.Ctx) error {
	if e := require(c, "TU"); e != nil {
		return e
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
		Status  string `json:"status"`
		Note    string `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ProposalKind(in.Kind) || (in.Status != "VALID" && in.Status != "REVISION") || !domain.Text(in.Note, 0, 2000) || (in.Status == "REVISION" && !domain.Text(in.Note, 5, 2000)) {
		return fiber.NewError(422, "Keputusan dan catatan pemeriksaan tidak valid.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var project string
	var version int
	if e = tx.QueryRow(c.Context(), `SELECT project_id,version FROM capstone_service.proposal_applications WHERE id=$1 FOR UPDATE`, c.Params("id")).Scan(&project, &version); e != nil {
		return e
	}
	if e = stale(version, in.Version); e != nil {
		return e
	}
	if e = unrecordedProposal(c.Context(), tx, c.Params("id")); e != nil {
		return e
	}
	tag, e := tx.Exec(c.Context(), `UPDATE capstone_service.proposal_documents SET status=$3,note=$4,reviewer=$5,reviewed_at=now() WHERE application_id=$1 AND kind=$2`, c.Params("id"), in.Kind, in.Status, in.Note, actor(c).ID)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return fiber.NewError(422, "Berkas belum diunggah.")
	}
	_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_applications SET version=version+1 WHERE id=$1`, c.Params("id"))
	if e != nil {
		return e
	}
	if in.Kind == "simak" || in.Kind == "approval" {
		k := in.Kind
		_, e = tx.Exec(c.Context(), `UPDATE capstone_service.proposal_applications SET `+k+`_status=$2,`+k+`_note=$3,`+k+`_reviewer=$4,`+k+`_reviewed_at=now() WHERE id=$1`, c.Params("id"), in.Status, in.Note, actor(c).ID)
		if e != nil {
			return e
		}
	}
	if e = audit(c.Context(), tx, actor(c), project, "PROPOSAL_FILE_REVIEWED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
