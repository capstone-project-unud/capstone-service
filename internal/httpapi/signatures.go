package httpapi

import (
	"bytes"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"strconv"
)

func signatureKey(ctx context.Context, db querier, id string) (string, error) {
	var key string
	e := db.QueryRow(ctx, `SELECT storage_key FROM capstone_service.lecturer_signatures WHERE lecturer_id=$1`, id).Scan(&key)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", fiber.NewError(422, "Unggah template tanda tangan di profil terlebih dahulu.")
	}
	return key, e
}
func canonicalSignature(data []byte) ([]byte, error) {
	if len(data) > 2*1024*1024 {
		return nil, fiber.NewError(422, "Template tanda tangan maksimal 2 MB.")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 8000000 {
		return nil, fiber.NewError(422, "Gunakan gambar PNG/JPEG yang valid, maksimal 4096 piksel per sisi dan 8 megapiksel.")
	}
	im, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return nil, fiber.ErrUnprocessableEntity
	}
	var b bytes.Buffer
	if e = png.Encode(&b, im); e != nil {
		return nil, e
	}
	if b.Len() > 2*1024*1024 {
		return nil, fiber.NewError(422, "Gambar setelah diproses melebihi 2 MB. Perkecil resolusi template tanda tangan.")
	}
	return b.Bytes(), nil
}
func (s *Server) getSignature(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	var version int
	var updated string
	e := s.DB.QueryRow(c.Context(), `SELECT version,updated_at::text FROM capstone_service.lecturer_signatures WHERE lecturer_id=$1`, actor(c).ID).Scan(&version, &updated)
	if errors.Is(e, pgx.ErrNoRows) {
		return c.JSON(fiber.Map{"signature": nil})
	}
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"signature": fiber.Map{"version": version, "updated_at": updated}})
}
func (s *Server) previewSignature(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	key, e := signatureKey(c.Context(), s.DB, actor(c).ID)
	if e != nil {
		return e
	}
	b, e := s.Files.Get(c.Context(), key)
	if e != nil {
		return fiber.ErrServiceUnavailable
	}
	c.Set("Content-Type", "image/png")
	return c.Send(b)
}
func (s *Server) saveSignature(c fiber.Ctx) error {
	if e := require(c, "DOSEN"); e != nil {
		return e
	}
	if e := revisionSchema(c.Context(), s.DB); e != nil {
		return e
	}
	f, e := c.FormFile("file")
	if e != nil {
		return fiber.ErrBadRequest
	}
	if f.Size > 2*1024*1024 {
		return fiber.ErrRequestEntityTooLarge
	}
	version, e := strconv.Atoi(c.FormValue("version"))
	if e != nil || version < 0 {
		return fiber.ErrBadRequest
	}
	stream, e := f.Open()
	if e != nil {
		return e
	}
	data, e := io.ReadAll(io.LimitReader(stream, 2*1024*1024+1))
	stream.Close()
	if e != nil {
		return e
	}
	data, e = canonicalSignature(data)
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	// The user lock also serializes first-time uploads, when no signature row exists yet.
	if _, e = tx.Exec(c.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actor(c).ID); e != nil {
		return e
	}
	var current int
	e = tx.QueryRow(c.Context(), `SELECT version FROM capstone_service.lecturer_signatures WHERE lecturer_id=$1 FOR UPDATE`, actor(c).ID).Scan(&current)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	if e = stale(current, version); e != nil {
		return e
	}
	key := uuid.NewString() + ".png"
	if e = s.Files.Put(c.Context(), key, data, "image/png"); e != nil {
		return fiber.ErrServiceUnavailable
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Files.Delete(context.WithoutCancel(c.Context()), key)
		}
	}()
	_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.lecturer_signatures(lecturer_id,storage_key) VALUES($1,$2) ON CONFLICT(lecturer_id) DO UPDATE SET storage_key=excluded.storage_key,version=lecturer_signatures.version+1,updated_at=now()`, actor(c).ID, key)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), "", "LECTURER_SIGNATURE_UPDATED", fiber.Map{"version": current + 1}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	committed = true
	return c.JSON(fiber.Map{"version": current + 1})
}
