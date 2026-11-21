package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/reports"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"time"
)

var indicatorDescriptions = []string{
	"Analisis persoalan computing kompleks dan berisiko", "Pemahaman prinsip computing", "Penerapan disiplin ilmu TI yang relevan",
	"Penerapan ilmu dan TI pada studi kasus", "Pemenuhan kebutuhan masyarakat melalui solusi TI", "Proses desain sistem hingga evaluasi",
	"Komunikasi lisan aktif dan efektif", "Penulisan dokumen sistematis dan sesuai tata bahasa",
	"Tanggung jawab profesional TI terhadap masyarakat", "Aspek legal, hak cipta, dan lisensi", "Integritas dan etika akademik",
	"Kontribusi kerja sama tim sesuai peran", "Evaluasi efektivitas kerja tim", "Analisis kebutuhan pengguna", "Pemilihan solusi dan standar teknologi", "Integrasi dan administrasi sistem yang aman",
}

func panelRoleLabel(role string) string {
	switch role {
	case "SUPERVISOR_1":
		return "Pembimbing 1"
	case "SUPERVISOR_2":
		return "Pembimbing 2"
	case "EXAMINER_1":
		return "Penguji 1"
	case "EXAMINER_2":
		return "Penguji 2"
	default:
		return "Penguji 3"
	}
}
func (s *Server) proposalExamReport(c fiber.Ctx) error {
	if !domain.ValidID(c.Params("id")) {
		return fiber.ErrBadRequest
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(c.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	ex, e := loadProposalExam(c.Context(), tx, c.Params("id"), false)
	if e != nil {
		return e
	}
	if !ex.canRead(actor(c)) {
		return fiber.ErrForbidden
	}
	kind := c.Query("kind")
	if kind != "attendance" && kind != "minutes" && kind != "score" {
		return fiber.ErrBadRequest
	}
	zone := time.FixedZone("WITA", 8*3600)
	r := reports.ExamReport{Reference: ex.ID, Student: ex.StudentName, NIM: ex.NIM, Title: ex.Title, Date: ex.ScheduledAt.In(zone).Format("02 January 2006, 15:04"), Location: ex.Location}
	if ex.Policy != nil {
		r.Policy = ex.Policy.Name + " (versi " + *ex.PolicyID + ")"
	}
	selected := ex.Panel
	if kind == "score" {
		panel := ex.panelMember(c.Query("evaluator_id"))
		if panel == nil {
			return fiber.ErrBadRequest
		}
		selected = []examPanel{*panel}
		r.Kind = "Form Nilai Seminar Proposal · " + panelRoleLabel(panel.Role)
		var ev *examEvaluation
		for _, v := range ex.Evaluations {
			if v.EvaluatorID == panel.ID {
				copy := v
				ev = &copy
			}
		}
		for i, k := range domain.ProposalIndicators() {
			row := reports.ExamScoreRow{Code: k, Label: indicatorDescriptions[i], Weight: "—", Score: "_____"}
			if ex.Policy != nil {
				row.Weight = fmt.Sprintf("%.4g", ex.Policy.IndicatorWeights[k])
			}
			if ev != nil {
				row.Score = fmt.Sprintf("%.2f", ev.Scores[k])
			}
			r.Scores = append(r.Scores, row)
		}
		if ev != nil {
			r.Result = fmt.Sprintf("Nilai penilai: %.2f / 100 · %s\nRekomendasi: %s", ev.Total, domain.GradeCategory(ev.Total), ev.Recommendation)
			r.Notes = ev.Notes
		} else {
			r.Result = "DRAF · Penilai belum mengirim nilai melalui sistem."
		}
	} else if kind == "attendance" {
		r.Kind = "Presensi Ujian Seminar Proposal"
		r.Notes = "Tanda tangan dicantumkan setelah masing-masing dosen mengonfirmasi presensi di sistem. Mahasiswa: ____________________"
	} else {
		r.Kind = "Berita Acara Ujian Seminar Proposal"
		r.Result = "DRAF · Nilai akhir menunggu lima penilai."
		if ex.FinalScore != nil {
			r.Result = fmt.Sprintf("Nilai akhir: %.2f · Huruf: %s · %s\nRekomendasi panel: %s", *ex.FinalScore, ex.Letter, ex.Category, ex.Outcome)
		}
		r.Notes = "Tanda tangan berikut merujuk konfirmasi penilaian masing-masing dosen. Presensi dicetak terpisah."
	}
	for _, panel := range selected {
		signer := reports.ExamSignatory{Name: panel.Name, Role: panelRoleLabel(panel.Role)}
		var key string
		var signed time.Time
		if kind == "attendance" {
			for _, a := range ex.Attendance {
				if a.LecturerID == panel.ID {
					signed = a.ConfirmedAt
				}
			}
			if !signed.IsZero() {
				e = tx.QueryRow(c.Context(), `SELECT signature_key FROM capstone_service.proposal_exam_attendance WHERE exam_id=$1 AND lecturer_id=$2`, ex.ID, panel.ID).Scan(&key)
			}
		} else {
			for _, ev := range ex.Evaluations {
				if ev.EvaluatorID == panel.ID {
					signed = ev.UpdatedAt
				}
			}
			if !signed.IsZero() {
				e = tx.QueryRow(c.Context(), `SELECT signature_key FROM capstone_service.proposal_exam_evaluations WHERE exam_id=$1 AND evaluator_id=$2`, ex.ID, panel.ID).Scan(&key)
			}
		}
		if e != nil {
			return e
		}
		if key != "" {
			signer.Signature, e = s.Files.Get(c.Context(), key)
			if e != nil {
				return fiber.NewError(503, "Tanda tangan belum dapat dimuat.")
			}
			signer.SignedAt = signed.In(zone).Format("02/01/2006 15:04 WITA")
		}
		r.Panel = append(r.Panel, signer)
	}
	b, e := reports.ProposalExamPDF(r)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="proposal-`+kind+`.pdf"`)
	return c.Send(b)
}
