package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"capstone/capstone-service/internal/reports"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type monitoringData struct {
	Type    string     `json:"type"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
	Empty   bool       `json:"empty"`
}

func buildMonitoringReport(ctx context.Context, db querier, rType, periodID, interestID, status, groupID string) (monitoringData, error) {
	full, e := hasFullSOPSchema(ctx, db)
	if e != nil {
		return monitoringData{}, e
	}
	if !full {
		return monitoringData{Type: rType, Headers: []string{}, Rows: [][]string{}, Empty: true}, nil
	}
	projectSempro := `coalesce((SELECT s.status FROM capstone_service.proposal_seminars s WHERE s.project_id=p.id),'BELUM')`
	studentSempro := projectSempro
	if revisionSchema(ctx, db) == nil {
		projectSempro = `coalesce((SELECT count(*)::text || ' ujian individu tercatat' FROM capstone_service.proposal_exams e JOIN capstone_service.proposal_applications a ON a.id=e.application_id WHERE a.project_id=p.id HAVING count(*)>0),` + projectSempro + `)`
		studentSempro = `coalesce((SELECT CASE WHEN count(v.evaluator_id)<5 THEN 'MENUNGGU NILAI' WHEN count(*) FILTER(WHERE v.recommendation='REJECTED')>0 THEN 'REJECTED' WHEN count(*) FILTER(WHERE v.recommendation='REVISION')>0 THEN 'REVISION' ELSE 'APPROVED' END FROM capstone_service.proposal_exams e JOIN capstone_service.proposal_applications a ON a.id=e.application_id LEFT JOIN capstone_service.proposal_exam_evaluations v ON v.exam_id=e.id WHERE a.project_id=p.id AND a.student_id=ps.student_id GROUP BY e.id),` + studentSempro + `)`
	}
	switch rType {
	case "progress":
		res := monitoringData{Type: rType, Headers: []string{"Kode Kelompok", "Periode", "Judul Capstone", "Bidang Minat", "Ketua Kelompok", "Jumlah Anggota", "Status Judul", "Status Sempro", "Status Ujian Akhir", "Status Expo"}}
		rows, e := db.Query(ctx, `
			SELECT coalesce(g.code,'-'), coalesce(ap.name,'-'), p.title, i.name,
			       coalesce((SELECT ps.student_name FROM capstone_service.project_students ps WHERE ps.project_id=p.id AND ps.is_leader LIMIT 1),'-'),
			       (SELECT count(DISTINCT ps.student_id) FROM capstone_service.project_students ps WHERE ps.project_id=p.id),
			       p.status,
			       `+projectSempro+`,
			       coalesce((SELECT fe.status FROM capstone_service.final_exams fe WHERE fe.project_id=p.id),'BELUM'),
			       coalesce((SELECT ex.status FROM capstone_service.expo_submissions ex WHERE ex.project_id=p.id),'BELUM')
			FROM capstone_service.projects p
			JOIN capstone_service.interest_fields i ON i.id=p.interest_id
			LEFT JOIN capstone_service.capstone_groups g ON g.id=p.group_id
			LEFT JOIN capstone_service.academic_periods ap ON ap.id=p.academic_period_id
			WHERE ($1='' OR p.academic_period_id::text=$1)
			  AND ($2='' OR p.interest_id::text=$2)
			  AND ($3='' OR p.status=$3)
			  AND ($4='' OR p.group_id::text=$4 OR p.id::text=$4)
			ORDER BY p.updated_at DESC LIMIT 500`, periodID, interestID, status, groupID)
		if e != nil {
			return res, e
		}
		defer rows.Close()
		for rows.Next() {
			var code, per, title, intr, leader, st, sp, fe, ex string
			var cnt int
			if e = rows.Scan(&code, &per, &title, &intr, &leader, &cnt, &st, &sp, &fe, &ex); e != nil {
				return res, e
			}
			res.Rows = append(res.Rows, []string{code, per, title, intr, leader, fmt.Sprint(cnt), st, sp, fe, ex})
		}
		res.Empty = len(res.Rows) == 0
		return res, rows.Err()

	case "guidance":
		res := monitoringData{Type: rType, Headers: []string{"Tanggal", "Mahasiswa", "NIM", "Judul Capstone", "Pembimbing", "Topik Pertemuan", "Status Validasi", "Catatan Pembimbing"}}
		rows, e := db.Query(ctx, `
			SELECT l.meeting_date::text, ps.student_name, ps.nim, p.title, l.supervisor_name, l.topic, l.status, l.review_note
			FROM capstone_service.guidance_logs l
			JOIN capstone_service.projects p ON p.id=l.project_id
			JOIN capstone_service.project_students ps ON ps.project_id=l.project_id AND ps.student_id=l.student_id AND ps.slot=1
			WHERE ($1='' OR p.academic_period_id::text=$1)
			  AND ($2='' OR p.interest_id::text=$2)
			  AND ($3='' OR l.status=$3)
			  AND ($4='' OR p.group_id::text=$4 OR p.id::text=$4)
			ORDER BY l.meeting_date DESC LIMIT 1000`, periodID, interestID, status, groupID)
		if e != nil {
			return res, e
		}
		defer rows.Close()
		for rows.Next() {
			var dt, sn, nim, title, sup, topic, st, note string
			if e = rows.Scan(&dt, &sn, &nim, &title, &sup, &topic, &st, &note); e != nil {
				return res, e
			}
			res.Rows = append(res.Rows, []string{dt, sn, nim, title, sup, topic, st, note})
		}
		res.Empty = len(res.Rows) == 0
		return res, rows.Err()

	case "outcomes":
		res := monitoringData{Type: rType, Headers: []string{"Mahasiswa", "NIM", "Judul Capstone", "Bidang Minat", "Status Sempro", "Status Ujian Akhir", "Nilai Akhir (PA II)", "Kategori Mutu", "Status Expo Publik"}}
		rows, e := db.Query(ctx, `
			SELECT DISTINCT ps.student_name, ps.nim, p.title, i.name,
			       `+studentSempro+`,
			       coalesce((SELECT fe.status FROM capstone_service.final_exams fe WHERE fe.project_id=p.id),'BELUM'),
			       coalesce((SELECT round(avg(ev.total_score),2)::text FROM capstone_service.final_exam_evaluations ev WHERE ev.project_id=p.id AND ev.student_id=ps.student_id AND ev.evaluator_role IN ('EXAMINER_1','EXAMINER_2','EXAMINER_3')),'Belum dinilai'),
			       coalesce((SELECT ev.grade_label FROM capstone_service.final_exam_evaluations ev WHERE ev.project_id=p.id AND ev.student_id=ps.student_id LIMIT 1),'-'),
			       coalesce((SELECT ex.status FROM capstone_service.expo_submissions ex WHERE ex.project_id=p.id),'BELUM')
			FROM capstone_service.project_students ps
			JOIN capstone_service.projects p ON p.id=ps.project_id
			JOIN capstone_service.interest_fields i ON i.id=p.interest_id
			WHERE ps.slot=1
			  AND ($1='' OR p.academic_period_id::text=$1)
			  AND ($2='' OR p.interest_id::text=$2)
			  AND ($3='' OR p.status=$3)
			  AND ($4='' OR p.group_id::text=$4 OR p.id::text=$4)
			ORDER BY ps.nim ASC LIMIT 500`, periodID, interestID, status, groupID)
		if e != nil {
			return res, e
		}
		defer rows.Close()
		for rows.Next() {
			var sn, nim, title, intr, sp, fe, sc, gr, ex string
			if e = rows.Scan(&sn, &nim, &title, &intr, &sp, &fe, &sc, &gr, &ex); e != nil {
				return res, e
			}
			res.Rows = append(res.Rows, []string{sn, nim, title, intr, sp, fe, sc, gr, ex})
		}
		res.Empty = len(res.Rows) == 0
		return res, rows.Err()
	default:
		return monitoringData{}, fiber.NewError(422, "Jenis laporan tidak dikenali (pilih progress, guidance, atau outcomes).")
	}
}

func (s *Server) monitoringReport(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA", "TU", "KOPRODI"); e != nil {
		return e
	}
	rType := strings.TrimSpace(c.Query("type", "progress"))
	periodID, interestID, status, groupID := c.Query("period_id"), c.Query("interest_id"), c.Query("status"), c.Query("group_id")
	for _, id := range []string{periodID, interestID, groupID} {
		if id != "" && !domain.ValidID(id) {
			return fiber.ErrBadRequest
		}
	}
	res, e := buildMonitoringReport(c.Context(), s.DB, rType, periodID, interestID, status, groupID)
	if e != nil {
		return e
	}
	return c.JSON(res)
}

func (s *Server) exportMonitoring(c fiber.Ctx) error {
	if e := require(c, "ADMIN", "KOORDINATOR_TA", "TU", "KOPRODI"); e != nil {
		return e
	}
	rType := strings.TrimSpace(c.Query("type", "progress"))
	format := strings.ToLower(strings.TrimSpace(c.Query("format", "xlsx")))
	res, e := buildMonitoringReport(c.Context(), s.DB, rType, c.Query("period_id"), c.Query("interest_id"), c.Query("status"), c.Query("group_id"))
	if e != nil {
		return e
	}
	if res.Empty {
		return fiber.NewError(422, "Tidak ada data laporan untuk filter yang dipilih.")
	}
	if format == "pdf" {
		paras := []string{"Jenis Laporan: " + rType + " | Dicetak: " + time.Now().In(time.FixedZone("WITA", 8*3600)).Format("02-01-2006 15:04 MST")}
		for idx, row := range res.Rows {
			paras = append(paras, fmt.Sprintf("%d. %s", idx+1, strings.Join(row, " | ")))
		}
		b, e := reports.PDF("Laporan Monitoring Capstone Project", paras)
		if e != nil {
			return e
		}
		c.Set("Content-Type", "application/pdf")
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Laporan_%s.pdf"`, rType))
		return c.Send(b)
	}
	allRows := append([][]string{res.Headers}, res.Rows...)
	b, e := reports.XLSX(allRows)
	if e != nil {
		return e
	}
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Laporan_%s.xlsx"`, rType))
	return c.Send(b)
}
