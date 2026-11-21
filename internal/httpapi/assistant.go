package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func extractSearchKeywords(query string) string {
	stop := map[string]bool{
		"apa": true, "apakah": true, "bagaimana": true, "kenapa": true, "mengapa": true,
		"siapa": true, "dimana": true, "kapan": true, "berapa": true, "cara": true,
		"yang": true, "atau": true, "dengan": true, "untuk": true, "dalam": true,
		"pada": true, "dari": true, "oleh": true, "sebuah": true, "suatu": true,
		"adalah": true, "bisa": true, "dapat": true, "saja": true, "dan": true,
		"itu": true, "ini": true, "tolong": true, "jelaskan": true, "sebutkan": true,
	}
	cleaned := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == ' ' {
			return r
		}
		return ' '
	}, strings.ToLower(query))
	kept := []string{}
	for _, w := range strings.Fields(cleaned) {
		if len(w) >= 2 && !stop[w] {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " OR ")
}

func retrieveKnowledge(ctx context.Context, db querier, query string, activeOnly bool, sourceID string) ([]domain.ChatCitation, float64, error) {
	tsQuery := extractSearchKeywords(query)
	if tsQuery == "" {
		return []domain.ChatCitation{}, 0, nil
	}
	rows, e := db.Query(ctx, `
		SELECT ks.title, ks.version_label, kc.section_title, kc.page_ref, kc.content,
		       ts_rank_cd(kc.search_vec, websearch_to_tsquery('simple', $1)) AS score
		FROM capstone_service.knowledge_chunks kc
		JOIN capstone_service.knowledge_sources ks ON ks.id=kc.source_id
		WHERE ((NOT $2 AND ks.id::text=$3) OR ($2 AND ks.status='ACTIVE'))
		  AND kc.search_vec @@ websearch_to_tsquery('simple', $1)
		ORDER BY score DESC, kc.chunk_index ASC
		LIMIT 4`, tsQuery, activeOnly, sourceID)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	cites := []domain.ChatCitation{}
	topScore := 0.0
	for rows.Next() {
		var c domain.ChatCitation
		var sc float64
		if e = rows.Scan(&c.SourceTitle, &c.VersionLabel, &c.SectionTitle, &c.PageRef, &c.Excerpt, &sc); e != nil {
			return nil, 0, e
		}
		if sc > topScore {
			topScore = sc
		}
		cites = append(cites, c)
	}
	return cites, topScore, rows.Err()
}

func formatGroundedAnswer(cites []domain.ChatCitation) (string, string) {
	if len(cites) == 0 {
		return "Maaf, informasi yang Anda tanyakan tidak ditemukan secara memadai pada dokumen pedoman Capstone aktif. Silakan perjelas pertanyaan Anda (misalnya mengenai kelompok, Sempro, B-100..B-400, Ujian Akhir, atau Expo) atau konfirmasi langsung ke Koordinator Capstone / Tata Usaha.", "INSUFFICIENT_CONTEXT"
	}
	var b strings.Builder
	for i, ct := range cites {
		b.WriteString(fmt.Sprintf("%d. **%s** (%s · %s):\n%s\n\n", i+1, ct.SectionTitle, ct.PageRef, ct.VersionLabel, ct.Excerpt))
	}
	return b.String(), "ANSWERED"
}

func (s *Server) listKnowledgeSources(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.KnowledgeSource{}})
	}
	u := actor(c)
	rows, e := s.DB.Query(c.Context(), `
		SELECT id,code,title,version_label,effective_date::text,approval_proof,status,chunk_count,last_test_status,activated_at,updated_at
		FROM capstone_service.knowledge_sources
		WHERE ($1 OR status='ACTIVE')
		ORDER BY CASE status WHEN 'ACTIVE' THEN 1 WHEN 'INDEXED' THEN 2 WHEN 'DRAFT' THEN 3 ELSE 4 END, updated_at DESC`, u.Has("ADMIN"))
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []domain.KnowledgeSource{}
	for rows.Next() {
		var ks domain.KnowledgeSource
		if e = rows.Scan(&ks.ID, &ks.Code, &ks.Title, &ks.VersionLabel, &ks.EffectiveDate, &ks.ApprovalProof, &ks.Status, &ks.ChunkCount, &ks.LastTestStatus, &ks.ActivatedAt, &ks.UpdatedAt); e != nil {
			return e
		}
		items = append(items, ks)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) saveKnowledgeSource(c fiber.Ctx) error {
	if e := require(c, "ADMIN"); e != nil {
		return e
	}
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Asisten belum tersedia. Hubungi administrator.")
	}
	var in struct {
		Code          string  `json:"code"`
		Title         string  `json:"title"`
		VersionLabel  string  `json:"version_label"`
		EffectiveDate string  `json:"effective_date"`
		ApprovalProof string  `json:"approval_proof"`
		SourceFileID  *string `json:"source_file_id"`
		RawText       string  `json:"raw_text"`
	}
	if c.Bind().JSON(&in) != nil || !domain.Text(in.Code, 3, 60) || !domain.Text(in.Title, 5, 300) || !domain.Text(in.VersionLabel, 2, 40) || !domain.Text(in.RawText, 30, 200000) || !domain.ValidOptional(in.SourceFileID) {
		return fiber.NewError(422, "Kode, judul, versi, tanggal berlaku, dan teks isi pedoman resmi (minimal 30 karakter) wajib diisi.")
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(in.EffectiveDate)); err != nil {
		return fiber.NewError(422, "Tanggal berlaku wajib berformat YYYY-MM-DD.")
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	id := uuid.NewString()
	_, e = tx.Exec(c.Context(), `
		INSERT INTO capstone_service.knowledge_sources(id,code,title,version_label,effective_date,approval_proof,source_file_id,raw_text,status,uploaded_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'DRAFT',$9)`,
		id, strings.ToUpper(strings.TrimSpace(in.Code)), strings.TrimSpace(in.Title), strings.TrimSpace(in.VersionLabel), strings.TrimSpace(in.EffectiveDate), strings.TrimSpace(in.ApprovalProof), in.SourceFileID, strings.TrimSpace(in.RawText), actor(c).ID)
	if e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), "", "KB_SOURCE_CREATED", map[string]any{"id": id, "code": in.Code, "version": in.VersionLabel}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}

func (s *Server) indexKnowledgeSource(c fiber.Ctx) error {
	if e := require(c, "ADMIN"); e != nil {
		return e
	}
	id := c.Params("id")
	if !domain.ValidID(id) {
		return fiber.ErrBadRequest
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var raw string
	if e = tx.QueryRow(c.Context(), `SELECT raw_text FROM capstone_service.knowledge_sources WHERE id=$1 FOR UPDATE`, id).Scan(&raw); e != nil {
		return e
	}
	sections := domain.SplitKnowledgeSections(raw)
	if len(sections) == 0 {
		return fiber.NewError(422, "Isi pedoman tidak menghasilkan bagian teks terindeks.")
	}
	if _, e = tx.Exec(c.Context(), `DELETE FROM capstone_service.knowledge_chunks WHERE source_id=$1`, id); e != nil {
		return e
	}
	for i, sec := range sections {
		if _, e = tx.Exec(c.Context(), `
			INSERT INTO capstone_service.knowledge_chunks(id,source_id,chunk_index,section_title,page_ref,content)
			VALUES($1,$2,$3,$4,$5,$6)`,
			uuid.NewString(), id, i+1, sec.Title, fmt.Sprintf("Bagian %d", i+1), sec.Body); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.knowledge_sources SET status='INDEXED',chunk_count=$2,last_test_status='UNTESTED',updated_at=now() WHERE id=$1`, id, len(sections)); e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), "", "KB_SOURCE_INDEXED", map[string]any{"id": id, "chunks": len(sections)}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"chunk_count": len(sections)})
}

func (s *Server) testKnowledgeSource(c fiber.Ctx) error {
	if e := require(c, "ADMIN"); e != nil {
		return e
	}
	id := c.Params("id")
	if !domain.ValidID(id) {
		return fiber.ErrBadRequest
	}
	var in struct {
		Query string `json:"query"`
	}
	_ = c.Bind().JSON(&in)
	q := strings.TrimSpace(in.Query)
	if q == "" {
		q = "syarat"
	}
	cites, _, e := retrieveKnowledge(c.Context(), s.DB, q, false, id)
	if e != nil {
		return e
	}
	testStatus := "PASSED"
	if len(cites) == 0 {
		var count int
		_ = s.DB.QueryRow(c.Context(), `SELECT chunk_count FROM capstone_service.knowledge_sources WHERE id=$1`, id).Scan(&count)
		if count == 0 {
			testStatus = "FAILED"
		}
	}
	_, _ = s.DB.Exec(c.Context(), `UPDATE capstone_service.knowledge_sources SET last_test_status=$2,updated_at=now() WHERE id=$1`, id, testStatus)
	answer, msgStatus := formatGroundedAnswer(cites)
	return c.JSON(fiber.Map{"test_status": testStatus, "answer": answer, "status": msgStatus, "citations": cites})
}

func (s *Server) activateKnowledgeSource(c fiber.Ctx) error {
	if e := require(c, "ADMIN"); e != nil {
		return e
	}
	id := c.Params("id")
	if !domain.ValidID(id) {
		return fiber.ErrBadRequest
	}
	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	var code, lastTest string
	var chunkCount int
	if e = tx.QueryRow(c.Context(), `SELECT code,chunk_count,last_test_status FROM capstone_service.knowledge_sources WHERE id=$1 FOR UPDATE`, id).Scan(&code, &chunkCount, &lastTest); e != nil {
		return e
	}
	if chunkCount == 0 || lastTest != "PASSED" {
		return fiber.NewError(422, "Jalankan pengindeksan dan uji jawaban sumber pedoman sebelum mengaktifkannya.")
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.knowledge_sources SET status='ARCHIVED',updated_at=now() WHERE code=$1 AND status='ACTIVE' AND id <> $2`, code, id); e != nil {
		return e
	}
	if _, e = tx.Exec(c.Context(), `UPDATE capstone_service.knowledge_sources SET status='ACTIVE',activated_by=$2,activated_at=now(),updated_at=now() WHERE id=$1`, id, actor(c).ID); e != nil {
		return e
	}
	if e = audit(c.Context(), tx, actor(c), "", "KB_SOURCE_ACTIVATED", map[string]any{"id": id, "code": code}); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.SendStatus(204)
}

func (s *Server) listChatSessions(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return c.JSON(fiber.Map{"items": []domain.ChatSession{}})
	}
	rows, e := s.DB.Query(c.Context(), `SELECT id,title,updated_at FROM capstone_service.chat_sessions WHERE user_id=$1 ORDER BY updated_at DESC LIMIT 20`, actor(c).ID)
	if e != nil {
		return e
	}
	sessions := []domain.ChatSession{}
	for rows.Next() {
		var cs domain.ChatSession
		cs.Messages = []domain.ChatMessage{}
		if e = rows.Scan(&cs.ID, &cs.Title, &cs.UpdatedAt); e != nil {
			rows.Close()
			return e
		}
		sessions = append(sessions, cs)
	}
	rows.Close()
	for i := range sessions {
		mRows, e := s.DB.Query(c.Context(), `SELECT id,session_id,role,content,status,citations,created_at FROM capstone_service.chat_messages WHERE session_id=$1 ORDER BY created_at ASC`, sessions[i].ID)
		if e != nil {
			return e
		}
		for mRows.Next() {
			var m domain.ChatMessage
			var rawCites []byte
			if e = mRows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Status, &rawCites, &m.CreatedAt); e != nil {
				mRows.Close()
				return e
			}
			_ = json.Unmarshal(rawCites, &m.Citations)
			sessions[i].Messages = append(sessions[i].Messages, m)
		}
		mRows.Close()
	}
	return c.JSON(fiber.Map{"items": sessions})
}

func (s *Server) askAssistant(c fiber.Ctx) error {
	ok, e := hasFullSOPSchema(c.Context(), s.DB)
	if e != nil {
		return e
	}
	if !ok {
		return fiber.NewError(503, "Asisten belum tersedia. Hubungi administrator.")
	}
	var in struct {
		SessionID *string `json:"session_id"`
		Question  string  `json:"question"`
	}
	if c.Bind().JSON(&in) != nil || !domain.ValidOptional(in.SessionID) || !domain.Text(in.Question, 3, 1000) {
		return fiber.NewError(422, "Pertanyaan pedoman 3–1000 karakter wajib diisi.")
	}
	u := actor(c)
	cites, _, e := retrieveKnowledge(c.Context(), s.DB, in.Question, true, "")
	if e != nil {
		return e
	}
	answer, status := formatGroundedAnswer(cites)

	tx, e := s.DB.Begin(c.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(c.Context())
	sessionID := ""
	if in.SessionID != nil {
		var owner string
		if e = tx.QueryRow(c.Context(), `SELECT user_id FROM capstone_service.chat_sessions WHERE id=$1 FOR UPDATE`, *in.SessionID).Scan(&owner); e != nil {
			return e
		}
		if owner != u.ID {
			return fiber.ErrForbidden
		}
		sessionID = *in.SessionID
		_, _ = tx.Exec(c.Context(), `UPDATE capstone_service.chat_sessions SET updated_at=now() WHERE id=$1`, sessionID)
	} else {
		sessionID = uuid.NewString()
		title := in.Question
		if len([]rune(title)) > 80 {
			title = string([]rune(title)[:80]) + "…"
		}
		_, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.chat_sessions(id,user_id,title) VALUES($1,$2,$3)`, sessionID, u.ID, title)
		if e != nil {
			return e
		}
	}
	if _, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.chat_messages(id,session_id,role,content,status) VALUES($1,$2,'USER',$3,'ANSWERED')`,
		uuid.NewString(), sessionID, strings.TrimSpace(in.Question)); e != nil {
		return e
	}
	citeBytes, _ := json.Marshal(cites)
	replyID := uuid.NewString()
	if _, e = tx.Exec(c.Context(), `INSERT INTO capstone_service.chat_messages(id,session_id,role,content,status,citations) VALUES($1,$2,'ASSISTANT',$3,$4,$5)`,
		replyID, sessionID, answer, status, citeBytes); e != nil {
		return e
	}
	if e = tx.Commit(c.Context()); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"session_id": sessionID, "message": domain.ChatMessage{ID: replyID, SessionID: sessionID, Role: "ASSISTANT", Content: answer, Status: status, Citations: cites, CreatedAt: time.Now()}})
}
