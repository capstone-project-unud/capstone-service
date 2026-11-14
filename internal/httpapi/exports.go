package httpapi

import (
	"capstone/capstone-service/internal/reports"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"time"
)

func (s *Server) exportProjects(c fiber.Ctx) error {
	if !actor(c).Staff() {
		return fiber.ErrForbidden
	}
	items, _, e := s.projectList(c, true)
	if e != nil {
		return e
	}
	rows := [][]string{{"ID judul", "Judul", "Bidang minat", "Status", "Mahasiswa", "NIM", "Posisi pembimbing", "Calon pembimbing", "Persetujuan judul", "Kesediaan membimbing", "Pembimbing ditetapkan", "Catatan", "Waktu finalisasi"}}
	for _, p := range items {
		at := ""
		if p.FinalizedAt != nil {
			at = p.FinalizedAt.In(time.FixedZone("WITA", 8*60*60)).Format("2006-01-02 15:04 MST")
		}
		for _, r := range p.Students {
			rows = append(rows, []string{p.ID, p.Title, p.InterestName, p.Status, r.StudentName, r.NIM, fmt.Sprint(r.Slot), r.CandidateName, r.TitleStatus, r.ConfirmationStatus, r.SupervisorName, r.Note, at})
		}
	}
	b, e := reports.XLSX(rows)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", `attachment; filename="Daftar_Judul_Capstone.xlsx"`)
	return c.Send(b)
}
func (s *Server) letter(c fiber.Ctx) error {
	p, e := s.project(c)
	if e != nil {
		return e
	}
	if p.Status != "FINALIZED" {
		return fiber.NewError(422, "Surat tersedia setelah finalisasi judul.")
	}
	paras := []string{"Format sementara. Nomor surat, kop, dasar penetapan, dan penandatangan mengikuti format resmi yang akan ditetapkan program studi.", "Judul: " + p.Title, "Bidang minat: " + p.InterestName}
	for _, r := range p.Students {
		if r.Slot == 1 {
			paras = append(paras, "Mahasiswa: "+r.StudentName+" | NIM: "+r.NIM)
		}
		if r.SupervisorID != nil {
			paras = append(paras, fmt.Sprintf("Pembimbing %d: %s", r.Slot, r.SupervisorName))
		}
	}
	if p.FinalizedAt != nil {
		paras = append(paras, "Tanggal penetapan: "+p.FinalizedAt.In(time.FixedZone("WITA", 8*60*60)).Format("02-01-2006"))
	}
	b, e := reports.PDF("Penetapan Judul Capstone Mahasiswa", paras)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="Penetapan_Judul_Capstone.pdf"`)
	return c.Send(b)
}
func (s *Server) exportGuidance(c fiber.Ctx) error {
	if e := require(c, "MAHASISWA", "DOSEN", "KOORDINATOR_TA", "ADMIN"); e != nil {
		return e
	}
	items, e := s.guidanceList(c)
	if e != nil {
		return e
	}
	paras := []string{}
	for _, l := range items {
		paras = append(paras, fmt.Sprintf("Tanggal: %s\nMahasiswa: %s | NIM: %s\nJudul: %s\nBidang minat: %s\nPembimbing: %s\nTopik: %s\nKemajuan: %s\nKendala: %s\nRencana berikutnya: %s\nStatus: %s\nCatatan pembimbing: %s", l.MeetingDate.Format("02-01-2006"), l.StudentName, l.NIM, l.Title, l.InterestName, l.SupervisorName, l.Topic, l.Progress, l.Obstacles, l.NextPlan, l.Status, l.ReviewNote))
	}
	if len(paras) == 0 {
		return fiber.NewError(422, "Belum ada log bimbingan untuk dicetak.")
	}
	b, e := reports.PDF("Log Bimbingan Capstone", paras)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="Log_Bimbingan_Capstone.pdf"`)
	return c.Send(b)
}
