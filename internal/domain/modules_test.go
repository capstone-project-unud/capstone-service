package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestGroupInputValidate(t *testing.T) {
	periodID, interestID := uuid.NewString(), uuid.NewString()
	s1, s2 := uuid.NewString(), uuid.NewString()
	in := GroupInput{
		AcademicPeriodID:   periodID,
		InterestID:         interestID,
		WorkingTitle:       "Sistem Monitoring Sensor IoT Cerdas",
		InitialDescription: "Rancang bangun sistem monitoring kualitas air tambak udang berbasis IoT dan analitik prediktif.",
		Finalize:           true,
		Members: []GroupMemberInput{
			{StudentID: s1, IsLeader: true, Responsibility: "Arsitektur firmware ESP32 & protokol MQTT"},
			{StudentID: s2, IsLeader: false, Responsibility: "Backend API, dashboard analitik, dan pengujian sistem"},
		},
	}
	leader, e := in.Validate(s1)
	if e != nil || leader != s1 {
		t.Fatalf("unexpected error=%v leader=%s", e, leader)
	}
	in.Members[1].IsLeader = true
	if _, e := in.Validate(s1); e == nil {
		t.Fatal("expected error when two leaders exist")
	}
	in.Members[1].IsLeader = false
	in.Members[1].Responsibility = ""
	if _, e := in.Validate(s1); e == nil {
		t.Fatal("expected error when finalized group member has blank responsibility")
	}
}

func TestPeriodAndDocTypeValidation(t *testing.T) {
	p := PeriodInput{Code: "2026-genap", Name: "Semester Genap 2025/2026", Semester: "genap", StartDate: "2026-02-01", EndDate: "2026-07-31", GroupOpen: true, IsActive: true}
	if e := p.Validate(); e != nil {
		t.Fatalf("unexpected period error: %v", e)
	}
	if p.Code != "2026-GENAP" || p.Semester != "GENAP" {
		t.Fatalf("expected uppercase code/semester, got %s/%s", p.Code, p.Semester)
	}
	p.EndDate = "2026-01-01"
	if e := p.Validate(); e == nil {
		t.Fatal("expected end_date before start_date error")
	}
	dt := DocumentTypeInput{Code: "PROPOSAL_USULAN", Name: "Naskah Proposal Usulan", Category: "PROPOSAL_REPORT", Stage: "SEMPRO", Scope: "GROUP", IsRequired: true, IsActive: true, SortOrder: 60}
	if e := dt.Validate(); e != nil {
		t.Fatalf("unexpected doc type error: %v", e)
	}
}

func TestExpoAndFinalExamEvaluateValidation(t *testing.T) {
	proj, poster := uuid.NewString(), uuid.NewString()
	expo := ExpoInput{
		ProjectID:          proj,
		Title:              "Sistem Pintar Deteksi Hama Tanaman Padi",
		Category:           "SOFTWARE",
		AbstractID:         "Abstrak ringkas mengenai solusi computer vision untuk deteksi hama padi.",
		PosterFileID:       &poster,
		VideoURL:           "https://youtu.be/demo12345",
		RepoURL:            "https://github.com/psti-unud/capstone-demo",
		PublicationConsent: true,
		Submit:             true,
	}
	if e := expo.Validate(); e != nil {
		t.Fatalf("unexpected expo validation error: %v", e)
	}
	expo.VideoURL = "javascript:alert(1)"
	if e := expo.Validate(); e == nil {
		t.Fatal("expected javascript url rejection")
	}

	ev := FinalExamEvaluateInput{
		StudentID:      uuid.NewString(),
		Scores:         fullScores(75),
		Recommendation: "PASSED",
	}
	total, grade, e := ev.Validate()
	if e != nil || total != 75 || grade != "SANGAT BAIK" {
		t.Fatalf("unexpected eval result total=%v grade=%s err=%v", total, grade, e)
	}
}

func TestSplitKnowledgeSections(t *testing.T) {
	raw := "# Syarat Sempro\nMahasiswa harus menyelesaikan B-100 dan memiliki pembimbing tetap.\n\n# Syarat Ujian Akhir\nMinimal 140 SKS dengan nilai minimal C dan uji plagiarisme kurang dari 20 persen."
	parts := SplitKnowledgeSections(raw)
	if len(parts) != 2 || parts[0].Title != "Syarat Sempro" {
		t.Fatalf("unexpected split sections: %+v", parts)
	}
}
