package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"github.com/gofiber/fiber/v3"
)

func (s *Server) reopen(c fiber.Ctx) error {
	if e := require(c, "KOORDINATOR_TA"); e != nil {
		return e
	}
	var in struct {
		Version int    `json:"version"`
		Note    string `json:"note"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Note, 5, 2000) {
		return fiber.NewError(422, "Alasan koreksi 5–2000 karakter wajib diisi.")
	}
	tx, p, e := s.lock(c, in.Version)
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	if p.Status != "FINALIZED" {
		return fiber.ErrConflict
	}
	var submitted bool
	if e = tx.QueryRow(c.Context(), "SELECT EXISTS(SELECT 1 FROM capstone_service.proposal_applications WHERE project_id=$1)", p.ID).Scan(&submitted); e != nil {
		return e
	}
	if submitted {
		return fiber.NewError(409, "Penetapan tidak dapat dibuka kembali setelah berkas UP diajukan.")
	}
	if _, e = tx.Exec(c.Context(), "UPDATE capstone_service.projects SET status='SUBMITTED',finalized_at=NULL,finalized_by=NULL,revision_note=$2 WHERE id=$1", p.ID, in.Note); e != nil {
		return e
	}
	return s.finish(c, tx, p, "FINALIZATION_REOPENED", in)
}
