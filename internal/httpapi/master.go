package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var masterSOPReference = regexp.MustCompile(`\s*\([^)]*\bSOP-\d[^)]*\)`)

func (s *Server) listPeriods(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.AcademicPeriod{}})
	}
	rows, e := s.DB.Query(c.Context(), `
		SELECT id,code,name,semester,start_date::text,end_date::text,group_open,topic_open,sempro_open,exam_open,is_active
		FROM capstone_service.academic_periods
		ORDER BY is_active DESC, start_date DESC`)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []domain.AcademicPeriod{}
	for rows.Next() {
		var it domain.AcademicPeriod
		if e = rows.Scan(&it.ID, &it.Code, &it.Name, &it.Semester, &it.StartDate, &it.EndDate, &it.GroupOpen, &it.TopicOpen, &it.SemproOpen, &it.ExamOpen, &it.IsActive); e != nil {
			return e
		}
		items = append(items, it)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) savePeriod(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Data akademik belum tersedia. Hubungi administrator.")
	}
	var in domain.PeriodInput
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrBadRequest
	}
	if e := in.Validate(); e != nil {
		return fiber.NewError(422, e.Error())
	}
	id := c.Params("id")
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if in.IsActive {
		if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.academic_periods SET is_active=false WHERE id::text <> $1`, id); e != nil {
			return e
		}
	}
	if id == "" {
		id = uuid.NewString()
		_, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.academic_periods(id,code,name,semester,start_date,end_date,group_open,topic_open,sempro_open,exam_open,is_active)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			id, in.Code, in.Name, in.Semester, in.StartDate, in.EndDate, in.GroupOpen, in.TopicOpen, in.SemproOpen, in.ExamOpen, in.IsActive)
	} else {
		if !domain.ValidID(id) {
			return fiber.ErrBadRequest
		}
		tag, err := tx.Exec(c.Context(), `
			UPDATE capstone_service.academic_periods
			SET code=$2,name=$3,semester=$4,start_date=$5,end_date=$6,group_open=$7,topic_open=$8,sempro_open=$9,exam_open=$10,is_active=$11,updated_at=now()
			WHERE id=$1`,
			id, in.Code, in.Name, in.Semester, in.StartDate, in.EndDate, in.GroupOpen, in.TopicOpen, in.SemproOpen, in.ExamOpen, in.IsActive)
		e = err
		if e == nil && tag.RowsAffected() == 0 {
			return fiber.ErrNotFound
		}
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), "", "MASTER_PERIOD_SAVED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"id": id})
}

func (s *Server) listDocumentTypes(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.DocumentType{
			{Code: "B-100", Name: "B-100 · Dokumen Identifikasi Masalah", Category: "B_SERIES", Stage: "SEMPRO", Scope: "GROUP", IsRequired: true, IsActive: true, SortOrder: 20},
			{Code: "B-200", Name: "B-200 · Dokumen Spesifikasi dan Perancangan Desain", Category: "B_SERIES", Stage: "BIMBINGAN", Scope: "GROUP", IsRequired: true, IsActive: true, SortOrder: 30},
			{Code: "B-300", Name: "B-300 · Dokumen Spesifikasi dan Implementasi Produk", Category: "B_SERIES", Stage: "UJIAN_AKHIR", Scope: "GROUP", IsRequired: true, IsActive: true, SortOrder: 40},
			{Code: "B-400", Name: "B-400 · Dokumen Pengujian dan Validasi Desain", Category: "B_SERIES", Stage: "UJIAN_AKHIR", Scope: "GROUP", IsRequired: true, IsActive: true, SortOrder: 50},
		}})
	}
	rows, e := s.DB.Query(c.Context(), `
		SELECT code,name,category,stage,scope,description,accepted_mime,is_required,is_active,sort_order
		FROM capstone_service.document_types
		ORDER BY sort_order ASC, code ASC`)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []domain.DocumentType{}
	for rows.Next() {
		var it domain.DocumentType
		if e = rows.Scan(&it.Code, &it.Name, &it.Category, &it.Stage, &it.Scope, &it.Description, &it.AcceptedMime, &it.IsRequired, &it.IsActive, &it.SortOrder); e != nil {
			return e
		}
		it.Name = masterSOPReference.ReplaceAllString(it.Name, "")
		it.Description = masterSOPReference.ReplaceAllString(it.Description, "")
		items = append(items, it)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) saveDocumentType(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Jenis dokumen belum tersedia. Hubungi administrator.")
	}
	var in domain.DocumentTypeInput
	if c.Bind().JSON(&in) != nil {
		return fiber.ErrBadRequest
	}
	if param := strings.TrimSpace(c.Params("code")); param != "" {
		in.Code = param
	}
	if e := in.Validate(); e != nil {
		return fiber.NewError(422, e.Error())
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if c.Params("code") == "" {
		_, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.document_types(code,name,category,stage,scope,description,is_required,is_active,sort_order)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			in.Code, in.Name, in.Category, in.Stage, in.Scope, in.Description, in.IsRequired, in.IsActive, in.SortOrder)
	} else {
		tag, err := tx.Exec(c.Context(), `
			UPDATE capstone_service.document_types
			SET name=$2,category=$3,stage=$4,scope=$5,description=$6,is_required=$7,is_active=$8,sort_order=$9,updated_at=now()
			WHERE code=$1`,
			in.Code, in.Name, in.Category, in.Stage, in.Scope, in.Description, in.IsRequired, in.IsActive, in.SortOrder)
		e = err
		if e == nil && tag.RowsAffected() == 0 {
			return fiber.ErrNotFound
		}
	}
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), "", "MASTER_DOCTYPE_SAVED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"code": in.Code})
}

func (s *Server) listStageRequirements(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.StageRequirement{}})
	}
	rows, e := s.DB.Query(c.Context(), `
		SELECT stage,label,min_group_members,max_group_members,min_sks,min_grade,min_guidance_logs,max_plagiarism_pct,max_assignment_age_days,required_doc_codes,notes
		FROM capstone_service.stage_requirements
		ORDER BY CASE stage WHEN 'KELOMPOK' THEN 1 WHEN 'SEMINAR_TOPIK' THEN 2 WHEN 'SEMINAR_PROPOSAL' THEN 3 WHEN 'UJIAN_AKHIR' THEN 4 ELSE 5 END`)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []domain.StageRequirement{}
	for rows.Next() {
		var it domain.StageRequirement
		var rawDocs []byte
		if e = rows.Scan(&it.Stage, &it.Label, &it.MinGroupMembers, &it.MaxGroupMembers, &it.MinSKS, &it.MinGrade, &it.MinGuidanceLogs, &it.MaxPlagiarismPct, &it.MaxAssignmentAgeDays, &rawDocs, &it.Notes); e != nil {
			return e
		}
		_ = json.Unmarshal(rawDocs, &it.RequiredDocCodes)
		it.Label = masterSOPReference.ReplaceAllString(it.Label, "")
		items = append(items, it)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) updateStageRequirement(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Persyaratan belum tersedia. Hubungi administrator.")
	}
	stage := strings.ToUpper(strings.TrimSpace(c.Params("stage")))
	var in domain.StageRequirement
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Label, 3, 120) || in.MinGroupMembers < 1 || in.MaxGroupMembers < in.MinGroupMembers || in.MinSKS < 0 || in.MaxPlagiarismPct < 0 || in.MaxPlagiarismPct > 100 {
		return fiber.NewError(422, "Data persyaratan tahap tidak valid.")
	}
	rawDocs, _ := json.Marshal(in.RequiredDocCodes)
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	tag, e := tx.Exec(c.Context(), `
		UPDATE capstone_service.stage_requirements
		SET label=$2,min_group_members=$3,max_group_members=$4,min_sks=$5,min_grade=$6,min_guidance_logs=$7,max_plagiarism_pct=$8,max_assignment_age_days=$9,required_doc_codes=$10,notes=$11,updated_at=now()
		WHERE stage=$1`,
		stage, strings.TrimSpace(in.Label), in.MinGroupMembers, in.MaxGroupMembers, in.MinSKS, strings.TrimSpace(in.MinGrade), in.MinGuidanceLogs, in.MaxPlagiarismPct, in.MaxAssignmentAgeDays, rawDocs, strings.TrimSpace(in.Notes))
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return fiber.ErrNotFound
	}
	if e = audit(c.Context(), tx, actor(c), "", "MASTER_STAGE_UPDATED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) listRubrics(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.AssessmentRubricItem{}})
	}
	stage := strings.TrimSpace(c.Query("stage"))
	rows, e := s.DB.Query(c.Context(), `
		SELECT id,stage,cpmk_code,ik_code,cpmk_title,ik_description,weight,max_score,is_active,sort_order
		FROM capstone_service.assessment_rubrics
		WHERE ($1='' OR stage=$1)
		ORDER BY stage ASC, sort_order ASC`, stage)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []domain.AssessmentRubricItem{}
	for rows.Next() {
		var it domain.AssessmentRubricItem
		if e = rows.Scan(&it.ID, &it.Stage, &it.CPMKCode, &it.IKCode, &it.CPMKTitle, &it.IKDescription, &it.Weight, &it.MaxScore, &it.IsActive, &it.SortOrder); e != nil {
			return e
		}
		items = append(items, it)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) updateRubricItem(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA"); e != nil {
		return e
	}
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Rubrik penilaian belum tersedia. Hubungi administrator.")
	}
	var in struct {
		CPMKTitle     string  `json:"cpmk_title"`
		IKDescription string  `json:"ik_description"`
		Weight        float64 `json:"weight"`
		IsActive      bool    `json:"is_active"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.CPMKTitle, 3, 250) || !domain.Text(in.IKDescription, 5, 1000) || in.Weight <= 0 || in.Weight > 100 {
		return fiber.NewError(422, "Rincian CPMK/IK rubrik tidak valid.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	tag, e := tx.Exec(c.Context(), `UPDATE capstone_service.assessment_rubrics SET cpmk_title=$2,ik_description=$3,weight=$4,is_active=$5,updated_at=now() WHERE id=$1`,
		c.Params("id"), strings.TrimSpace(in.CPMKTitle), strings.TrimSpace(in.IKDescription), in.Weight, in.IsActive)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return fiber.ErrNotFound
	}
	if e = audit(c.Context(), tx, actor(c), "", "MASTER_RUBRIC_UPDATED", in); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}
