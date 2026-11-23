package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// SOP-004: Data Master Akademik
type AcademicPeriod struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Semester  string `json:"semester"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	GroupOpen bool   `json:"group_open"`
	TopicOpen bool   `json:"topic_open"`
	SemproOpen bool  `json:"sempro_open"`
	ExamOpen  bool   `json:"exam_open"`
	IsActive  bool   `json:"is_active"`
}

type PeriodInput struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Semester   string `json:"semester"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	GroupOpen  bool   `json:"group_open"`
	TopicOpen  bool   `json:"topic_open"`
	SemproOpen bool   `json:"sempro_open"`
	ExamOpen   bool   `json:"exam_open"`
	IsActive   bool   `json:"is_active"`
}

func (in *PeriodInput) Validate() error {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.Semester = strings.ToUpper(strings.TrimSpace(in.Semester))
	if !Text(in.Code, 3, 40) || !Text(in.Name, 3, 150) {
		return fmt.Errorf("Kode dan nama periode wajib diisi.")
	}
	if in.Semester != "GANJIL" && in.Semester != "GENAP" && in.Semester != "ANTARA" {
		return fmt.Errorf("Semester harus GANJIL, GENAP, atau ANTARA.")
	}
	s, e1 := time.Parse("2006-01-02", strings.TrimSpace(in.StartDate))
	en, e2 := time.Parse("2006-01-02", strings.TrimSpace(in.EndDate))
	if e1 != nil || e2 != nil || en.Before(s) {
		return fmt.Errorf("Rentang tanggal mulai dan selesai periode tidak valid.")
	}
	in.StartDate, in.EndDate = s.Format("2006-01-02"), en.Format("2006-01-02")
	return nil
}

type DocumentType struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Stage        string `json:"stage"`
	Scope        string `json:"scope"`
	Description  string `json:"description"`
	AcceptedMime string `json:"accepted_mime"`
	IsRequired   bool   `json:"is_required"`
	IsActive     bool   `json:"is_active"`
	SortOrder    int    `json:"sort_order"`
}

type DocumentTypeInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Stage       string `json:"stage"`
	Scope       string `json:"scope"`
	Description string `json:"description"`
	IsRequired  bool   `json:"is_required"`
	IsActive    bool   `json:"is_active"`
	SortOrder   int    `json:"sort_order"`
}

func (in *DocumentTypeInput) Validate() error {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if !ValidDocTypeCode(in.Code) || !Text(in.Name, 3, 200) {
		return fmt.Errorf("Kode dokumen (huruf kapital/angka/-/_) dan nama jenis dokumen wajib diisi.")
	}
	if in.Category != "B_SERIES" && in.Category != "PROPOSAL_REPORT" && in.Category != "APPROVAL_ADMIN" && in.Category != "FINAL_REQUIREMENT" && in.Category != "EXPO_OUTPUT" {
		return fmt.Errorf("Kategori jenis dokumen tidak valid.")
	}
	if in.Stage != "TOPIK" && in.Stage != "SEMPRO" && in.Stage != "BIMBINGAN" && in.Stage != "UJIAN_AKHIR" && in.Stage != "EXPO" {
		return fmt.Errorf("Tahap dokumen tidak valid.")
	}
	if in.Scope != "GROUP" && in.Scope != "INDIVIDUAL" {
		return fmt.Errorf("Lingkup dokumen harus GROUP atau INDIVIDUAL.")
	}
	if len(in.Description) > 1000 {
		return fmt.Errorf("Deskripsi jenis dokumen maksimal 1000 karakter.")
	}
	if in.SortOrder < 1 {
		in.SortOrder = 100
	}
	return nil
}

type StageRequirement struct {
	Stage                 string   `json:"stage"`
	Label                 string   `json:"label"`
	MinGroupMembers       int      `json:"min_group_members"`
	MaxGroupMembers       int      `json:"max_group_members"`
	MinSKS                int      `json:"min_sks"`
	MinGrade              string   `json:"min_grade"`
	MinGuidanceLogs       int      `json:"min_guidance_logs"`
	MaxPlagiarismPct      int      `json:"max_plagiarism_pct"`
	MaxAssignmentAgeDays  int      `json:"max_assignment_age_days"`
	RequiredDocCodes      []string `json:"required_doc_codes"`
	Notes                 string   `json:"notes"`
}

type AssessmentRubricItem struct {
	ID            string  `json:"id"`
	Stage         string  `json:"stage"`
	CPMKCode      string  `json:"cpmk_code"`
	IKCode        string  `json:"ik_code"`
	CPMKTitle     string  `json:"cpmk_title"`
	IKDescription string  `json:"ik_description"`
	Weight        float64 `json:"weight"`
	MaxScore      float64 `json:"max_score"`
	IsActive      bool    `json:"is_active"`
	SortOrder     int     `json:"sort_order"`
}

// SOP-005: Pembentukan Kelompok Capstone Project terlebih dahulu
type GroupMember struct {
	StudentID      string    `json:"student_id"`
	StudentName    string    `json:"student_name"`
	NIM            string    `json:"nim"`
	IsLeader       bool      `json:"is_leader"`
	Responsibility string    `json:"responsibility"`
	JoinedAt       time.Time `json:"joined_at"`
}

type GroupAdvisorBrief struct {
	StudentID                     string  `json:"student_id"`
	StudentName                   string  `json:"student_name"`
	Slot                          int     `json:"slot"`
	CandidateID                   *string `json:"candidate_id"`
	CandidateName                 string  `json:"candidate_name"`
	SupervisorID                  *string `json:"supervisor_id"`
	SupervisorName                string  `json:"supervisor_name"`
	IsExternalSupervisor          bool    `json:"is_external_supervisor"`
	ExternalSupervisorName        string  `json:"external_supervisor_name"`
	ExternalSupervisorInstitution string  `json:"external_supervisor_institution"`
	TitleStatus                   string  `json:"title_status"`
	ConfirmationStatus            string  `json:"confirmation_status"`
}

type CapstoneGroup struct {
	ID                 string              `json:"id"`
	Code               string              `json:"code"`
	AcademicPeriodID   string              `json:"academic_period_id"`
	AcademicPeriodName string              `json:"academic_period_name"`
	InterestID         string              `json:"interest_id"`
	InterestName       string              `json:"interest_name"`
	WorkingTitle       string              `json:"working_title"`
	InitialDescription string              `json:"initial_description"`
	LeaderID           string              `json:"leader_id"`
	LeaderName         string              `json:"leader_name"`
	Status             string              `json:"status"`
	ProjectID          *string             `json:"project_id"`
	ProjectTitle       string              `json:"project_title"`
	ProjectStatus      string              `json:"project_status"`
	Version            int                 `json:"version"`
	UpdatedAt          time.Time           `json:"updated_at"`
	Members            []GroupMember       `json:"members"`
	Advisors           []GroupAdvisorBrief `json:"advisors"`
}

type GroupMemberInput struct {
	StudentID      string `json:"student_id"`
	IsLeader       bool   `json:"is_leader"`
	Responsibility string `json:"responsibility"`
}

type GroupInput struct {
	AcademicPeriodID   string             `json:"academic_period_id"`
	InterestID         string             `json:"interest_id"`
	WorkingTitle       string             `json:"working_title"`
	InitialDescription string             `json:"initial_description"`
	Finalize           bool               `json:"finalize"`
	Members            []GroupMemberInput `json:"members"`
	Version            int                `json:"version"`
}

func (in *GroupInput) Validate(actorID string) (string, error) {
	in.WorkingTitle = strings.TrimSpace(in.WorkingTitle)
	in.InitialDescription = strings.TrimSpace(in.InitialDescription)
	if !ValidID(in.AcademicPeriodID) || !ValidID(in.InterestID) {
		return "", fmt.Errorf("Periode akademik dan bidang minat kelompok wajib dipilih.")
	}
	if !Text(in.WorkingTitle, 5, 500) || !Text(in.InitialDescription, 10, 10000) {
		return "", fmt.Errorf("Judul kerja 5–500 karakter dan deskripsi awal 10–10000 karakter wajib diisi.")
	}
	if len(in.Members) < 2 || len(in.Members) > 5 {
		return "", fmt.Errorf("Kelompok harus beranggotakan 2–5 mahasiswa.")
	}
	seen := map[string]bool{}
	leaderID := ""
	leaderCount := 0
	for i := range in.Members {
		m := &in.Members[i]
		m.Responsibility = strings.TrimSpace(m.Responsibility)
		if !ValidID(m.StudentID) || seen[m.StudentID] {
			return "", fmt.Errorf("Daftar anggota kelompok tidak valid atau duplikat.")
		}
		seen[m.StudentID] = true
		if m.IsLeader {
			leaderID = m.StudentID
			leaderCount++
		}
		if len(m.Responsibility) > 500 {
			return "", fmt.Errorf("Uraian tanggung jawab anggota maksimal 500 karakter.")
		}
		if in.Finalize && !Text(m.Responsibility, 3, 500) {
			return "", fmt.Errorf("Tanggung jawab setiap anggota wajib diisi (minimal 3 karakter) sebelum mengunci kesiapan kelompok.")
		}
	}
	if leaderCount != 1 {
		return "", fmt.Errorf("Kelompok wajib memiliki tepat satu Ketua Kelompok.")
	}
	if !seen[actorID] {
		return "", fmt.Errorf("Pembuat kelompok harus terdaftar sebagai salah satu anggota.")
	}
	return leaderID, nil
}

// SOP-008: Seminar Kelayakan Topik
type TopicSeminar struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	ProjectTitle     string     `json:"project_title"`
	InterestName     string     `json:"interest_name"`
	ScheduledAt      *time.Time `json:"scheduled_at"`
	Location         string     `json:"location"`
	Panelist1ID      *string    `json:"panelist_1_id"`
	Panelist1Name    string     `json:"panelist_1_name"`
	Panelist2ID      *string    `json:"panelist_2_id"`
	Panelist2Name    string     `json:"panelist_2_name"`
	Status           string     `json:"status"`
	FeasibilityNotes string     `json:"feasibility_notes"`
	DecisionNote     string     `json:"decision_note"`
	DecidedAt        *time.Time `json:"decided_at"`
	Version          int        `json:"version"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// SOP-014 & SOP-017: Item Revisi Penguji (Sempro & Ujian Akhir)
type RevisionItem struct {
	ID              string     `json:"id"`
	Stage           string     `json:"stage"`
	SeminarID       string     `json:"seminar_id"`
	ProjectID       string     `json:"project_id"`
	StudentID       *string    `json:"student_id"`
	ExaminerID      string     `json:"examiner_id"`
	ExaminerName    string     `json:"examiner_name"`
	Note            string     `json:"note"`
	StudentResponse string     `json:"student_response"`
	ResponseFileID  *string    `json:"response_file_id"`
	Status          string     `json:"status"`
	ReviewerNote    string     `json:"reviewer_note"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	Version         int        `json:"version"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// SOP-015..017: Ujian Akhir
type FinalExamReadiness struct {
	SemproPassed       bool   `json:"sempro_passed"`
	SemproRevApproved  bool   `json:"sempro_rev_approved"`
	BSeriesComplete    bool   `json:"b_series_complete"`
	FinalReportReady   bool   `json:"final_report_ready"`
	PlagiarismReady    bool   `json:"plagiarism_ready"`
	AssignmentValid    bool   `json:"assignment_valid"`
	GuidanceCount      int    `json:"guidance_count"`
	StudentsVerified   int    `json:"students_verified"`
	TotalStudents      int    `json:"total_students"`
	Ready              bool   `json:"ready"`
}

type FinalExamApplication struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"project_id"`
	ProjectTitle      string     `json:"project_title"`
	StudentID         string     `json:"student_id"`
	StudentName       string     `json:"student_name"`
	NIM               string     `json:"nim"`
	SKSCompleted      int        `json:"sks_completed"`
	MinGradeOK        bool       `json:"min_grade_ok"`
	TranscriptFileID  *string    `json:"transcript_file_id"`
	SPPFileID         *string    `json:"spp_file_id"`
	BiodataSummary    string     `json:"biodata_summary"`
	IjazahFileID      *string    `json:"ijazah_file_id"`
	PhotoFileID       *string    `json:"photo_file_id"`
	PALetterFileID    *string    `json:"pa_letter_file_id"`
	PlagiarismPct     float64    `json:"plagiarism_pct"`
	PlagiarismFileID  *string    `json:"plagiarism_file_id"`
	Advisor1Approved  bool       `json:"advisor_1_approved"`
	Advisor2Approved  bool       `json:"advisor_2_approved"`
	AdvisorNote       string     `json:"advisor_note"`
	AdvisorApprovedAt *time.Time `json:"advisor_approved_at"`
	TUStatus          string     `json:"tu_status"`
	TUNote            string     `json:"tu_note"`
	TUReviewedAt      *time.Time `json:"tu_reviewed_at"`
	CoordinatorStatus string     `json:"coordinator_status"`
	CoordinatorNote   string     `json:"coordinator_note"`
	Version           int        `json:"version"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type FinalExamAttendance struct {
	LecturerID  string    `json:"lecturer_id"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

type FinalExam struct {
	ID                     string                `json:"id"`
	ProjectID              string                `json:"project_id"`
	ScheduledAt            time.Time             `json:"scheduled_at"`
	Location               string                `json:"location"`
	Examiner1ID            string                `json:"examiner_1_id"`
	Examiner1Name          string                `json:"examiner_1_name"`
	Examiner2ID            string                `json:"examiner_2_id"`
	Examiner2Name          string                `json:"examiner_2_name"`
	Examiner3ID            string                `json:"examiner_3_id"`
	Examiner3Name          string                `json:"examiner_3_name"`
	Status                 string                `json:"status"`
	OutcomeNote            string                `json:"outcome_note"`
	MinutesFileID          *string               `json:"minutes_file_id"`
	RevisionApprovedFileID *string               `json:"revision_approved_file_id"`
	ScheduleHistory        []map[string]any      `json:"schedule_history"`
	DecidedAt              *time.Time            `json:"decided_at"`
	Version                int                   `json:"version"`
	Evaluations            []SemproEvaluation    `json:"evaluations"`
	Attendance             []FinalExamAttendance `json:"attendance"`
	StudentSummaries       []StudentScoreBrief   `json:"student_summaries"`
	RevisionItems          []RevisionItem        `json:"revision_items"`
}

type FinalExamEvaluateInput struct {
	StudentID      string             `json:"student_id"`
	Scores         map[string]float64 `json:"scores"`
	Recommendation string             `json:"recommendation"`
	Notes          string             `json:"notes"`
}

func (in *FinalExamEvaluateInput) Validate() (float64, string, error) {
	in.Notes = strings.TrimSpace(in.Notes)
	if !ValidID(in.StudentID) {
		return 0, "", fmt.Errorf("Identitas mahasiswa tidak valid.")
	}
	if in.Recommendation != "PASSED" && in.Recommendation != "REVISION" && in.Recommendation != "FAILED" {
		return 0, "", fmt.Errorf("Rekomendasi hasil ujian akhir tidak valid.")
	}
	if in.Recommendation != "PASSED" && !Text(in.Notes, 5, 2000) {
		return 0, "", fmt.Errorf("Catatan perbaikan atau ketidaklulusan minimal 5 karakter.")
	}
	return EvaluateScores(in.Scores)
}

// SOP-018: Capstone Expo & Publikasi Karya
type ExpoSubmission struct {
	ID                 string     `json:"id"`
	ProjectID          string     `json:"project_id"`
	ProjectTitle       string     `json:"project_title"`
	InterestName       string     `json:"interest_name"`
	Title              string     `json:"title"`
	Category           string     `json:"category"`
	AbstractID         string     `json:"abstract_id"`
	AbstractEN         string     `json:"abstract_en"`
	PosterFileID       *string    `json:"poster_file_id"`
	VideoURL           string     `json:"video_url"`
	RepoURL            string     `json:"repo_url"`
	DemoURL            string     `json:"demo_url"`
	JournalTitle       string     `json:"journal_title"`
	JournalClass       string     `json:"journal_class"`
	JournalURL         string     `json:"journal_url"`
	PublicationConsent bool       `json:"publication_consent"`
	Status             string     `json:"status"`
	ReviewNote         string     `json:"review_note"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	PublishedAt        *time.Time `json:"published_at"`
	Version            int        `json:"version"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type ExpoInput struct {
	ProjectID          string  `json:"project_id"`
	Title              string  `json:"title"`
	Category           string  `json:"category"`
	AbstractID         string  `json:"abstract_id"`
	AbstractEN         string  `json:"abstract_en"`
	PosterFileID       *string `json:"poster_file_id"`
	VideoURL           string  `json:"video_url"`
	RepoURL            string  `json:"repo_url"`
	DemoURL            string  `json:"demo_url"`
	JournalTitle       string  `json:"journal_title"`
	JournalClass       string  `json:"journal_class"`
	JournalURL         string  `json:"journal_url"`
	PublicationConsent bool    `json:"publication_consent"`
	Submit             bool    `json:"submit"`
	Version            int     `json:"version"`
}

func ValidHTTPURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (in *ExpoInput) Validate() error {
	in.Title = strings.TrimSpace(in.Title)
	in.AbstractID = strings.TrimSpace(in.AbstractID)
	in.AbstractEN = strings.TrimSpace(in.AbstractEN)
	in.VideoURL = strings.TrimSpace(in.VideoURL)
	in.RepoURL = strings.TrimSpace(in.RepoURL)
	in.DemoURL = strings.TrimSpace(in.DemoURL)
	in.JournalURL = strings.TrimSpace(in.JournalURL)
	if !ValidID(in.ProjectID) || !Text(in.Title, 5, 500) || !Text(in.AbstractID, 20, 10000) || !ValidOptional(in.PosterFileID) {
		return fmt.Errorf("Judul karya, kategori, dan abstrak (minimal 20 karakter) wajib diisi.")
	}
	if in.Category != "SOFTWARE" && in.Category != "HARDWARE" && in.Category != "ALGORITMA" && in.Category != "DOKUMEN_STRATEGIS" {
		return fmt.Errorf("Kategori karya tidak valid.")
	}
	if !ValidHTTPURL(in.VideoURL) || !ValidHTTPURL(in.RepoURL) || !ValidHTTPURL(in.DemoURL) || !ValidHTTPURL(in.JournalURL) {
		return fmt.Errorf("Tautan video, repositori, demo, dan jurnal harus berupa URL http/https yang valid.")
	}
	if in.Submit {
		if in.PosterFileID == nil || in.RepoURL == "" || in.VideoURL == "" || !in.PublicationConsent {
			return fmt.Errorf("Pengajuan publikasi Expo mewajibkan Poster PDF, tautan GitHub, tautan Video Demo, dan persetujuan izin publikasi.")
		}
	}
	return nil
}

// SOP-019: Asisten Pedoman (RAG)
type KnowledgeSource struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`
	Title         string     `json:"title"`
	VersionLabel  string     `json:"version_label"`
	EffectiveDate string     `json:"effective_date"`
	ApprovalProof string     `json:"approval_proof"`
	RawText       string     `json:"raw_text,omitempty"`
	Status        string     `json:"status"`
	ChunkCount    int        `json:"chunk_count"`
	LastTestStatus string    `json:"last_test_status"`
	ActivatedAt   *time.Time `json:"activated_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type ChatCitation struct {
	SourceTitle  string `json:"source_title"`
	VersionLabel string `json:"version_label"`
	SectionTitle string `json:"section_title"`
	PageRef      string `json:"page_ref"`
	Excerpt      string `json:"excerpt"`
}

type ChatMessage struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Status    string         `json:"status"`
	Citations []ChatCitation `json:"citations"`
	CreatedAt time.Time      `json:"created_at"`
}

type ChatSession struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	UpdatedAt time.Time     `json:"updated_at"`
	Messages  []ChatMessage `json:"messages"`
}

func SplitKnowledgeSections(raw string) []struct{ Title, Body string } {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	blocks := strings.Split(raw, "\n\n")
	out := []struct{ Title, Body string }{}
	for idx, block := range blocks {
		b := strings.TrimSpace(block)
		if len(b) < 20 {
			continue
		}
		lines := strings.SplitN(b, "\n", 2)
		title := fmt.Sprintf("Bagian %d", idx+1)
		body := b
		if len(lines) == 2 && len([]rune(lines[0])) <= 180 {
			title = strings.TrimSpace(strings.TrimLeft(lines[0], "# "))
			body = strings.TrimSpace(lines[1])
		}
		if body == "" {
			body = title
		}
		out = append(out, struct{ Title, Body string }{Title: title, Body: body})
	}
	return out
}
