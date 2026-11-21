package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type DocumentSeries string

const (
	SeriesB100 DocumentSeries = "B-100"
	SeriesB200 DocumentSeries = "B-200"
	SeriesB300 DocumentSeries = "B-300"
	SeriesB400 DocumentSeries = "B-400"
)

var cpmkGroups = [][]string{
	{"IK-1a", "IK-1b", "IK-1c"},
	{"IK-2a", "IK-2b", "IK-2c"},
	{"IK-3a", "IK-3b"},
	{"IK-4a", "IK-4b", "IK-4c"},
	{"IK-5a", "IK-5b"},
	{"IK-6a", "IK-6b", "IK-6c"},
}

type DocumentVersion struct {
	ID             string     `json:"id"`
	DocumentID     string     `json:"document_id"`
	VersionNo      int        `json:"version_no"`
	FileID         string     `json:"file_id"`
	FileName       string     `json:"file_name"`
	Summary        string     `json:"summary"`
	UploadedBy     string     `json:"uploaded_by"`
	UploadedByName string     `json:"uploaded_by_name"`
	ReviewStatus   string     `json:"review_status"`
	ReviewNote     string     `json:"review_note"`
	ReviewerName   string     `json:"reviewer_name"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type ProjectDocument struct {
	ID             string            `json:"id"`
	ProjectID      string            `json:"project_id"`
	ProjectTitle   string            `json:"project_title"`
	DocTypeCode    string            `json:"doc_type_code"`
	DocTypeName    string            `json:"doc_type_name"`
	Category       string            `json:"category"`
	Stage          string            `json:"stage"`
	Scope          string            `json:"scope"`
	StudentID      *string           `json:"student_id"`
	StudentName    string            `json:"student_name"`
	Series         string            `json:"series"`
	FileID         string            `json:"file_id"`
	FileName       string            `json:"file_name"`
	UploadedBy     string            `json:"uploaded_by"`
	UploadedByName string            `json:"uploaded_by_name"`
	Summary        string            `json:"summary"`
	RevisionNo     int               `json:"revision_no"`
	Status         string            `json:"status"`
	ReviewNote     string            `json:"review_note"`
	ReviewerName   string            `json:"reviewer_name"`
	ReviewedAt     *time.Time        `json:"reviewed_at"`
	Version        int               `json:"version"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Versions       []DocumentVersion `json:"versions"`
}

type DocumentInput struct {
	ProjectID   string  `json:"project_id"`
	DocTypeCode string  `json:"doc_type_code"`
	Series      string  `json:"series"`
	StudentID   *string `json:"student_id"`
	FileID      string  `json:"file_id"`
	Summary     string  `json:"summary"`
	Version     int     `json:"version"`
}

func ValidDocumentSeries(s string) bool {
	return s == string(SeriesB100) || s == string(SeriesB200) || s == string(SeriesB300) || s == string(SeriesB400)
}

func ValidDocTypeCode(code string) bool {
	if len(code) < 2 || len(code) > 50 {
		return false
	}
	for _, r := range code {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (in *DocumentInput) Validate() error {
	in.Series = strings.TrimSpace(in.Series)
	in.DocTypeCode = strings.TrimSpace(strings.ToUpper(in.DocTypeCode))
	if in.DocTypeCode == "" {
		if !ValidDocumentSeries(in.Series) {
			return fmt.Errorf("Seri dokumen tidak valid (pilih B-100 s.d. B-400 atau tentukan kode jenis dokumen).")
		}
		in.DocTypeCode = in.Series
	}
	if in.Series == "" {
		in.Series = in.DocTypeCode
	}
	in.Summary = strings.TrimSpace(in.Summary)
	if !ValidID(in.ProjectID) || !ValidDocTypeCode(in.DocTypeCode) || !ValidOptional(in.StudentID) || !ValidID(in.FileID) {
		return fmt.Errorf("Proyek, jenis dokumen, dan berkas PDF wajib valid.")
	}
	if in.Summary != "" && !Text(in.Summary, 1, 2000) {
		return fmt.Errorf("Ringkasan perubahan maksimal 2000 karakter.")
	}
	return nil
}

type SemproReadiness struct {
	Finalized             bool   `json:"finalized"`
	GuidanceApprovedCount int    `json:"guidance_approved_count"`
	ProposalReady         bool   `json:"proposal_ready"`
	B100Status            string `json:"b100_status"`
	ProposalUsulanStatus  string `json:"proposal_usulan_status"`
	AdvisorApprovalCount  int    `json:"advisor_approval_count"`
	Ready                 bool   `json:"ready"`
}

type SemproAdvisorApproval struct {
	SupervisorID   string     `json:"supervisor_id"`
	SupervisorName string     `json:"supervisor_name"`
	Status         string     `json:"status"`
	Note           string     `json:"note"`
	SignedAt       *time.Time `json:"signed_at"`
}

type ProposalSeminar struct {
	ID                     string                  `json:"id"`
	ProjectID              string                  `json:"project_id"`
	ScheduledAt            time.Time               `json:"scheduled_at"`
	Location               string                  `json:"location"`
	Examiner1ID            string                  `json:"examiner_1_id"`
	Examiner1Name          string                  `json:"examiner_1_name"`
	Examiner2ID            string                  `json:"examiner_2_id"`
	Examiner2Name          string                  `json:"examiner_2_name"`
	Examiner3ID            string                  `json:"examiner_3_id"`
	Examiner3Name          string                  `json:"examiner_3_name"`
	Status                 string                  `json:"status"`
	OutcomeNote            string                  `json:"outcome_note"`
	RevisionFileID         *string                 `json:"revision_file_id"`
	RevisionNote           string                  `json:"revision_note"`
	RevisionSubmittedAt    *time.Time              `json:"revision_submitted_at"`
	MinutesFileID          *string                 `json:"minutes_file_id"`
	RevisionApprovedFileID *string                 `json:"revision_approved_file_id"`
	ScheduleHistory        []map[string]any        `json:"schedule_history"`
	DecidedAt              *time.Time              `json:"decided_at"`
	Version                int                     `json:"version"`
	Evaluations            []SemproEvaluation      `json:"evaluations"`
	StudentSummaries       []StudentScoreBrief     `json:"student_summaries"`
	AdvisorApprovals       []SemproAdvisorApproval `json:"advisor_approvals"`
	RevisionItems          []RevisionItem          `json:"revision_items"`
}

type SemproEvaluation struct {
	ID             string             `json:"id"`
	SeminarID      string             `json:"seminar_id"`
	StudentID      string             `json:"student_id"`
	StudentName    string             `json:"student_name"`
	NIM            string             `json:"nim"`
	EvaluatorID    string             `json:"evaluator_id"`
	EvaluatorName  string             `json:"evaluator_name"`
	EvaluatorRole  string             `json:"evaluator_role"`
	Scores         map[string]float64 `json:"scores"`
	TotalScore     float64            `json:"total_score"`
	GradeLabel     string             `json:"grade_label"`
	Recommendation string             `json:"recommendation"`
	Notes          string             `json:"notes"`
	Version        int                `json:"version"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type StudentScoreBrief struct {
	StudentID       string  `json:"student_id"`
	StudentName     string  `json:"student_name"`
	NIM             string  `json:"nim"`
	ExaminerCount   int     `json:"examiner_count"`
	SupervisorCount int     `json:"supervisor_count"`
	AverageScore    float64 `json:"average_score"`
	GradeLabel      string  `json:"grade_label"`
}

type SemproScheduleInput struct {
	ProjectID   string `json:"project_id"`
	ScheduledAt string `json:"scheduled_at"`
	Location    string `json:"location"`
	Examiner1ID string `json:"examiner_1_id"`
	Examiner2ID string `json:"examiner_2_id"`
	Examiner3ID string `json:"examiner_3_id"`
	Version     int    `json:"version"`
}

func (in *SemproScheduleInput) Validate() (time.Time, error) {
	in.Location = strings.TrimSpace(in.Location)
	if !ValidID(in.ProjectID) {
		return time.Time{}, fmt.Errorf("Judul capstone tidak valid.")
	}
	if !Text(in.Location, 3, 250) {
		return time.Time{}, fmt.Errorf("Lokasi atau tautan seminar wajib diisi (3–250 karakter).")
	}
	t, e := time.Parse(time.RFC3339, strings.TrimSpace(in.ScheduledAt))
	if e != nil || t.Year() < 2020 || t.Year() > 2100 {
		return time.Time{}, fmt.Errorf("Waktu seminar tidak valid.")
	}
	if !ValidID(in.Examiner1ID) || !ValidID(in.Examiner2ID) || !ValidID(in.Examiner3ID) {
		return time.Time{}, fmt.Errorf("Tiga dosen penguji wajib dipilih.")
	}
	if in.Examiner1ID == in.Examiner2ID || in.Examiner1ID == in.Examiner3ID || in.Examiner2ID == in.Examiner3ID {
		return time.Time{}, fmt.Errorf("Ketiga dosen penguji harus berbeda.")
	}
	return t, nil
}

type SemproEvaluateInput struct {
	StudentID      string             `json:"student_id"`
	Scores         map[string]float64 `json:"scores"`
	Recommendation string             `json:"recommendation"`
	Notes          string             `json:"notes"`
}

func (in *SemproEvaluateInput) Validate() (float64, string, error) {
	in.Notes = strings.TrimSpace(in.Notes)
	if !ValidID(in.StudentID) {
		return 0, "", fmt.Errorf("Identitas mahasiswa tidak valid.")
	}
	if in.Recommendation != "APPROVED" && in.Recommendation != "REVISION" && in.Recommendation != "REJECTED" {
		return 0, "", fmt.Errorf("Rekomendasi hasil seminar tidak valid.")
	}
	if in.Recommendation != "APPROVED" && !Text(in.Notes, 5, 2000) {
		return 0, "", fmt.Errorf("Catatan penilaian wajib diisi minimal 5 karakter untuk revisi atau penolakan.")
	}
	if len(in.Notes) > 2000 {
		return 0, "", fmt.Errorf("Catatan maksimal 2000 karakter.")
	}
	return EvaluateScores(in.Scores)
}

func EvaluateScores(scores map[string]float64) (float64, string, error) {
	expected := 0
	var cpmkSum float64
	for _, group := range cpmkGroups {
		var groupSum float64
		for _, ik := range group {
			expected++
			v, ok := scores[ik]
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
				return 0, "", fmt.Errorf("Nilai indikator %s harus antara 0 sampai 100.", ik)
			}
			groupSum += v
		}
		cpmkSum += groupSum / float64(len(group))
	}
	if len(scores) != expected {
		return 0, "", fmt.Errorf("Penilaian harus memuat tepat 16 indikator kinerja resmi.")
	}
	total := math.Round((cpmkSum/float64(len(cpmkGroups)))*100) / 100
	return total, GradeCategory(total), nil
}

func GradeCategory(score float64) string {
	switch {
	case score >= 80:
		return "ISTIMEWA"
	case score >= 71:
		return "SANGAT BAIK"
	case score >= 65:
		return "BAIK"
	case score >= 60:
		return "CUKUP BAIK"
	case score >= 55:
		return "CUKUP"
	case score >= 50:
		return "KURANG CUKUP"
	case score >= 40:
		return "KURANG"
	default:
		return "SANGAT KURANG"
	}
}
