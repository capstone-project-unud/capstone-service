package domain

import (
	"fmt"
	"math"
	"strings"
)

var ProposalKinds = []string{"simak", "approval", "english", "proposal", "b100", "b200"}
var PanelRoles = []string{"SUPERVISOR_1", "SUPERVISOR_2", "EXAMINER_1", "EXAMINER_2", "EXAMINER_3"}

func ProposalKind(kind string) bool {
	for _, k := range ProposalKinds {
		if k == kind {
			return true
		}
	}
	return false
}
func ProposalIndicators() []string {
	out := []string{}
	for _, g := range cpmkGroups {
		out = append(out, g...)
	}
	return out
}

type LetterBand struct {
	Minimum float64 `json:"minimum"`
	Letter  string  `json:"letter"`
}
type ProposalPolicy struct {
	Name             string             `json:"name"`
	IndicatorWeights map[string]float64 `json:"indicator_weights"`
	PanelWeights     map[string]float64 `json:"panel_weights"`
	LetterBands      []LetterBand       `json:"letter_bands"`
}

func validWeights(weights map[string]float64, keys []string) bool {
	if len(weights) != len(keys) {
		return false
	}
	sum := 0.0
	for _, k := range keys {
		v, ok := weights[k]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 100 {
			return false
		}
		sum += v
	}
	return math.Abs(sum-100) < 0.00001
}
func (p ProposalPolicy) Validate() error {
	if !Text(strings.TrimSpace(p.Name), 3, 150) || !validWeights(p.IndicatorWeights, ProposalIndicators()) || !validWeights(p.PanelWeights, PanelRoles) {
		return fmt.Errorf("Nama aturan wajib diisi; bobot 16 indikator dan lima penilai masing-masing harus berjumlah 100%%.")
	}
	if len(p.LetterBands) < 2 || len(p.LetterBands) > 10 {
		return fmt.Errorf("Isi batas konversi huruf, berurutan dari nilai tertinggi sampai 0.")
	}
	last := 101.0
	seen := map[string]bool{}
	for _, b := range p.LetterBands {
		if math.IsNaN(b.Minimum) || math.IsInf(b.Minimum, 0) || b.Minimum < 0 || b.Minimum > 100 || b.Minimum >= last || !Text(b.Letter, 1, 5) || seen[b.Letter] {
			return fmt.Errorf("Batas nilai huruf tidak valid atau berulang.")
		}
		last = b.Minimum
		seen[b.Letter] = true
	}
	if last != 0 {
		return fmt.Errorf("Batas nilai huruf terakhir harus 0.")
	}
	return nil
}
func (p ProposalPolicy) Score(scores map[string]float64) (float64, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if _, _, err := EvaluateScores(scores); err != nil {
		return 0, err
	}
	total := 0.0
	for k, w := range p.IndicatorWeights {
		total += scores[k] * w / 100
	}
	return math.Round(total*10000) / 10000, nil
}
func (p ProposalPolicy) Final(scores map[string]float64) (float64, string, bool) {
	if p.Validate() != nil || len(scores) != 5 {
		return 0, "", false
	}
	total := 0.0
	for _, k := range PanelRoles {
		v, ok := scores[k]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return 0, "", false
		}
		total += v * p.PanelWeights[k] / 100
	}
	total = math.Round(total*100) / 100
	for _, b := range p.LetterBands {
		if total >= b.Minimum {
			return total, b.Letter, true
		}
	}
	return 0, "", false
}
