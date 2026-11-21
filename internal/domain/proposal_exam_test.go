package domain

import (
	"math"
	"testing"
)

func samplePolicy() ProposalPolicy {
	p := ProposalPolicy{Name: "Aturan uji, bukan keputusan akademik", IndicatorWeights: map[string]float64{}, PanelWeights: map[string]float64{}, LetterBands: []LetterBand{{80, "A"}, {65, "B"}, {0, "C"}}}
	for _, k := range ProposalIndicators() {
		p.IndicatorWeights[k] = 6.25
	}
	for _, k := range PanelRoles {
		p.PanelWeights[k] = 20
	}
	return p
}
func TestIndividualProposalGrading(t *testing.T) {
	p := samplePolicy()
	p.PanelWeights["SUPERVISOR_1"] = 30
	p.PanelWeights["EXAMINER_3"] = 10
	if e := p.Validate(); e != nil {
		t.Fatal(e)
	}
	scores := map[string]float64{}
	for _, k := range ProposalIndicators() {
		scores[k] = 80
	}
	scores["IK-1a"] = 0
	total, e := p.Score(scores)
	if e != nil || total != 75 {
		t.Fatalf("weighted indicator score %v %v", total, e)
	}
	panel := map[string]float64{"SUPERVISOR_1": 100, "SUPERVISOR_2": 50, "EXAMINER_1": 80, "EXAMINER_2": 60}
	if _, _, ok := p.Final(panel); ok {
		t.Fatal("partial panel produced a final grade")
	}
	panel["EXAMINER_3"] = 40
	if v, letter, ok := p.Final(panel); !ok || v != 72 || letter != "B" {
		t.Fatalf("all five weighted panel grades: %v %s %v", v, letter, ok)
	}
	scores["IK-1a"] = math.NaN()
	if _, e = p.Score(scores); e == nil {
		t.Fatal("NaN accepted")
	}
	scores["IK-1a"] = 50
	scores["unofficial"] = 90
	if _, e = p.Score(scores); e == nil {
		t.Fatal("extra indicator accepted")
	}
}
func TestProposalPolicyRejectsAmbiguousRules(t *testing.T) {
	for _, change := range []func(*ProposalPolicy){
		func(p *ProposalPolicy) { delete(p.IndicatorWeights, "IK-6c") }, func(p *ProposalPolicy) { p.PanelWeights["SUPERVISOR_1"] = 0 },
		func(p *ProposalPolicy) { p.LetterBands = []LetterBand{{80, "A"}, {85, "B"}, {0, "C"}} }, func(p *ProposalPolicy) { p.LetterBands = []LetterBand{{80, "A"}, {10, "B"}} },
		func(p *ProposalPolicy) { p.LetterBands = []LetterBand{{80, "A"}, {0, "A"}} }, func(p *ProposalPolicy) { p.PanelWeights["EXAMINER_3"] = math.Inf(1) },
	} {
		p := samplePolicy()
		change(&p)
		if p.Validate() == nil {
			t.Fatal("invalid grading policy accepted")
		}
	}
	p := samplePolicy()
	for _, v := range []float64{80, 79.99, 65, 0} {
		scores := map[string]float64{}
		for _, k := range PanelRoles {
			scores[k] = v
		}
		n, l, ok := p.Final(scores)
		want := "C"
		if v >= 80 {
			want = "A"
		} else if v >= 65 {
			want = "B"
		}
		if !ok || n != v || l != want {
			t.Fatalf("boundary %v: %v %s", v, n, l)
		}
	}
}
