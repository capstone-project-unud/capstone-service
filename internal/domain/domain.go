package domain

import (
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode/utf8"
)

type User struct {
	ID       string   `json:"id"`
	FullName string   `json:"full_name"`
	NIM      string   `json:"nim"`
	Roles    []string `json:"roles"`
	IsActive bool     `json:"is_active"`
}

func (u User) Has(roles ...string) bool {
	for _, r := range roles {
		for _, v := range u.Roles {
			if v == r {
				return true
			}
		}
	}
	return false
}
func (u User) Staff() bool { return u.Has("ADMIN", "DOSEN", "KOORDINATOR_TA", "TU") }

type Project struct {
	ID                string        `json:"id"`
	OwnerID           string        `json:"owner_id"`
	Title             string        `json:"title"`
	Summary           string        `json:"summary"`
	InterestID        string        `json:"interest_id"`
	InterestName      string        `json:"interest_name"`
	DescriptionFileID *string       `json:"description_file_id"`
	SignedDescription bool          `json:"signed_description"`
	Status            string        `json:"status"`
	RevisionNote      string        `json:"revision_note"`
	Version           int           `json:"version"`
	FinalizedAt       *time.Time    `json:"finalized_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	Students          []Participant `json:"students"`
}
type Participant struct {
	StudentID          string     `json:"student_id"`
	StudentName        string     `json:"student_name"`
	NIM                string     `json:"nim"`
	Slot               int        `json:"slot"`
	CandidateID        *string    `json:"candidate_id"`
	CandidateName      string     `json:"candidate_name"`
	SupervisorID       *string    `json:"supervisor_id"`
	SupervisorName     string     `json:"supervisor_name"`
	TitleStatus        string     `json:"title_status"`
	ConfirmationStatus string     `json:"confirmation_status"`
	Note               string     `json:"note"`
	TitleActor         *string    `json:"title_actor"`
	TitleAt            *time.Time `json:"title_at"`
	ConfirmationActor  *string    `json:"confirmation_actor"`
	ConfirmationAt     *time.Time `json:"confirmation_at"`
}
type MemberInput struct {
	StudentID  string  `json:"student_id"`
	Candidate1 *string `json:"candidate_1"`
	Candidate2 *string `json:"candidate_2"`
}
type ProjectInput struct {
	Title             string        `json:"title"`
	Summary           string        `json:"summary"`
	InterestID        string        `json:"interest_id"`
	DescriptionFileID *string       `json:"description_file_id"`
	SignedDescription bool          `json:"signed_description"`
	Members           []MemberInput `json:"members"`
	Version           int           `json:"version"`
}

func ValidID(v string) bool        { _, e := uuid.Parse(v); return e == nil }
func ValidOptional(v *string) bool { return v == nil || ValidID(*v) }
func Text(v string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(v))
	return n >= min && n <= max
}
func (in *ProjectInput) Validate(owner string) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Summary = strings.TrimSpace(in.Summary)
	if !Text(in.Title, 5, 500) || !Text(in.Summary, 10, 10000) || !ValidID(in.InterestID) || !ValidOptional(in.DescriptionFileID) {
		return fmt.Errorf("Judul 5–500 karakter, deskripsi 10–10000 karakter, dan bidang minat wajib diisi.")
	}
	if len(in.Members) < 2 || len(in.Members) > 5 {
		return fmt.Errorf("Kelompok harus berisi 2–5 mahasiswa.")
	}
	seen := map[string]bool{}
	for _, m := range in.Members {
		if !ValidID(m.StudentID) || seen[m.StudentID] || !ValidOptional(m.Candidate1) || !ValidOptional(m.Candidate2) {
			return fmt.Errorf("Anggota atau calon pembimbing tidak valid atau berulang.")
		}
		seen[m.StudentID] = true
		if m.Candidate1 != nil && m.Candidate2 != nil && *m.Candidate1 == *m.Candidate2 {
			return fmt.Errorf("Dua calon pembimbing mahasiswa harus berbeda.")
		}
	}
	if !seen[owner] {
		return fmt.Errorf("Pengusul harus menjadi anggota kelompok.")
	}
	return nil
}
func ReadyToFinalize(p Project) error {
	if p.Status != "SUBMITTED" {
		return fmt.Errorf("Hanya judul yang diajukan dapat difinalisasi.")
	}
	if !p.SignedDescription || p.DescriptionFileID == nil {
		return fmt.Errorf("Deskripsi dengan pengesahan calon pembimbing wajib tersedia.")
	}
	count := map[string]int{}
	names := map[string]string{}
	assigned := map[string]map[string]bool{}
	for _, s := range p.Students {
		names[s.StudentID] = s.StudentName
		if s.SupervisorID == nil {
			continue
		}
		if s.TitleStatus != "APPROVED" || s.ConfirmationStatus != "APPROVED" {
			return fmt.Errorf("Persetujuan judul dan kesediaan pembimbing %s belum lengkap.", s.StudentName)
		}
		if assigned[s.StudentID] == nil {
			assigned[s.StudentID] = map[string]bool{}
		}
		if assigned[s.StudentID][*s.SupervisorID] {
			return fmt.Errorf("Pembimbing 1 dan 2 harus berbeda.")
		}
		assigned[s.StudentID][*s.SupervisorID] = true
		count[s.StudentID]++
	}
	for id, name := range names {
		if count[id] == 0 {
			return fmt.Errorf("Tetapkan sedikitnya satu pembimbing untuk %s.", name)
		}
	}
	if len(names) < 2 {
		return fmt.Errorf("Kelompok belum lengkap.")
	}
	return nil
}
