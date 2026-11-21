package domain

import (
	"testing"

	"github.com/google/uuid"
)

func fullScores(v float64) map[string]float64 {
	m := map[string]float64{}
	for _, g := range cpmkGroups {
		for _, ik := range g {
			m[ik] = v
		}
	}
	return m
}

func TestDocumentInputValidate(t *testing.T) {
	p, f := uuid.NewString(), uuid.NewString()
	in := DocumentInput{ProjectID: p, Series: "B-100", FileID: f, Summary: "Identifikasi masalah"}
	if e := in.Validate(); e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	bad := DocumentInput{ProjectID: p, Series: "B-999", FileID: f}
	if e := bad.Validate(); e == nil {
		t.Fatal("expected invalid series error")
	}
}

func TestSemproScheduleValidate(t *testing.T) {
	p, e1, e2, e3 := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	in := SemproScheduleInput{
		ProjectID:   p,
		ScheduledAt: "2026-12-01T09:00:00+08:00",
		Location:    "Ruang Sidang PSTI",
		Examiner1ID: e1,
		Examiner2ID: e2,
		Examiner3ID: e3,
	}
	if _, e := in.Validate(); e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	in.Examiner3ID = e1
	if _, e := in.Validate(); e == nil {
		t.Fatal("expected distinct examiners error")
	}
}

func TestEvaluateScoresAndGrades(t *testing.T) {
	// Equal-weighted CPMKs: CPMK-3 (2 IKs) = 90, all other 5 CPMKs = 78 -> (90 + 5*78)/6 = 80.00 ISTIMEWA
	scores := fullScores(78)
	scores["IK-3a"] = 90
	scores["IK-3b"] = 90
	total, grade, e := EvaluateScores(scores)
	if e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
	if total != 80 || grade != "ISTIMEWA" {
		t.Fatalf("got total=%v grade=%s want=80 ISTIMEWA", total, grade)
	}

	delete(scores, "IK-6c")
	if _, _, e := EvaluateScores(scores); e == nil {
		t.Fatal("expected missing indicator error")
	}

	outOfRange := fullScores(80)
	outOfRange["IK-1a"] = 101
	if _, _, e := EvaluateScores(outOfRange); e == nil {
		t.Fatal("expected out of range error")
	}

	for score, want := range map[float64]string{
		85: "ISTIMEWA", 75: "SANGAT BAIK", 67: "BAIK", 62: "CUKUP BAIK",
		57: "CUKUP", 52: "KURANG CUKUP", 45: "KURANG", 30: "SANGAT KURANG",
	} {
		if got := GradeCategory(score); got != want {
			t.Fatalf("score=%v got=%s want=%s", score, got, want)
		}
	}
}
