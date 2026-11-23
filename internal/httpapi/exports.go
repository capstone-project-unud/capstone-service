package httpapi

import (
	"capstone/capstone-service/internal/reports"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"strings"
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
	rows := [][]string{{"ID judul", "Judul", "Bidang minat", "Status", "Mahasiswa", "NIM", "Peran", "Posisi pembimbing", "Calon pembimbing", "Persetujuan judul", "Kesediaan membimbing", "Pembimbing ditetapkan", "Pembimbing Mitra/Eksternal", "Catatan", "Waktu finalisasi"}}
	for _, p := range items {
		at := ""
		if p.FinalizedAt != nil {
			at = p.FinalizedAt.In(time.FixedZone("WITA", 8*60*60)).Format("2006-01-02 15:04 MST")
		}
		for _, r := range p.Students {
			roleLabel := "Anggota"
			if r.IsLeader {
				roleLabel = "Ketua Kelompok"
			}
			extLabel := ""
			if r.ExternalSupervisorName != "" {
				extLabel = r.ExternalSupervisorName
				if r.ExternalSupervisorInstitution != "" {
					extLabel += " (" + r.ExternalSupervisorInstitution + ")"
				}
			}
			rows = append(rows, []string{p.ID, p.Title, p.InterestName, p.Status, r.StudentName, r.NIM, roleLabel, fmt.Sprint(r.Slot), r.CandidateName, r.TitleStatus, r.ConfirmationStatus, r.SupervisorName, extLabel, r.Note, at})
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
	byStudent := map[string][]string{}
	order := []string{}
	for _, r := range p.Students {
		if _, ok := byStudent[r.StudentID]; !ok {
			order = append(order, r.StudentID)
			role := "Anggota"
			if r.IsLeader {
				role = "Ketua Kelompok"
			}
			byStudent[r.StudentID] = []string{fmt.Sprintf("Mahasiswa: %s (NIM %s · %s)", r.StudentName, r.NIM, role)}
		}
		if r.SupervisorID != nil {
			byStudent[r.StudentID] = append(byStudent[r.StudentID], fmt.Sprintf("  - Pembimbing %d (%s): %s", r.Slot, r.StudentName, r.SupervisorName))
		}
		if r.Slot == 2 && r.ExternalSupervisorName != "" {
			ext := r.ExternalSupervisorName
			if r.ExternalSupervisorInstitution != "" {
				ext += " · " + r.ExternalSupervisorInstitution
			}
			byStudent[r.StudentID] = append(byStudent[r.StudentID], fmt.Sprintf("  - Pembimbing Tambahan / Mitra (%s): %s", r.StudentName, ext))
		}
	}
	for _, sid := range order {
		paras = append(paras, strings.Join(byStudent[sid], "\n"))
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
	if e := require(c, "MAHASISWA", "DOSEN", "KOORDINATOR_TA", "ADMIN", "KOPRODI"); e != nil {
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
