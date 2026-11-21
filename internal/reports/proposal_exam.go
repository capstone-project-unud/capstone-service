package reports

import (
	"bytes"
	"fmt"
	"github.com/go-pdf/fpdf"
	"math"
)

type ExamSignatory struct {
	Name, Role string
	Signature  []byte
	SignedAt   string
}
type ExamScoreRow struct{ Code, Label, Weight, Score string }
type ExamReport struct {
	Kind, Reference, Student, NIM, Title, Date, Location, Policy, Result, Notes string
	Panel                                                                       []ExamSignatory
	Scores                                                                      []ExamScoreRow
}

func ProposalExamPDF(r ExamReport) ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(20, 18, 20)
	p.SetAutoPageBreak(true, 20)
	p.AddUTF8FontFromBytes("DejaVu", "", font)
	p.SetHeaderFunc(func() {
		if p.PageNo() > 1 {
			p.SetFont("DejaVu", "", 9)
			p.MultiCell(170, 5, r.Kind+" (lanjutan)", "", "L", false)
			p.MultiCell(170, 5, r.NIM+" · "+r.Student, "", "L", false)
			p.Ln(3)
		}
	})
	p.SetFooterFunc(func() {
		p.SetY(-14)
		p.SetFont("DejaVu", "", 8)
		p.CellFormat(170, 5, fmt.Sprintf("%s · Halaman %d", r.Reference, p.PageNo()), "", 0, "R", false, 0, "")
	})
	p.AddPage()
	p.SetFont("DejaVu", "", 16)
	p.MultiCell(170, 8, r.Kind, "", "L", false)
	p.Ln(3)
	p.SetFont("DejaVu", "", 10)
	for _, line := range []string{"Program Studi Teknologi Informasi · Universitas Udayana", "Nama: " + r.Student, "NIM: " + r.NIM, "Judul: " + r.Title, "Waktu (WITA): " + r.Date, "Lokasi: " + r.Location} {
		p.MultiCell(170, 5.5, line, "", "L", false)
	}
	p.Ln(4)
	if r.Policy != "" {
		p.MultiCell(170, 5, "Aturan perhitungan: "+r.Policy, "", "L", false)
		p.Ln(3)
	}
	if len(r.Scores) > 0 {
		header := func() {
			p.SetFont("DejaVu", "", 9)
			p.CellFormat(128, 8, "Indikator kinerja", "B", 0, "L", false, 0, "")
			p.CellFormat(20, 8, "Bobot %", "B", 0, "R", false, 0, "")
			p.CellFormat(22, 8, "Nilai / 100", "B", 1, "R", false, 0, "")
		}
		header()
		for _, row := range r.Scores {
			label := row.Code + " · " + row.Label
			h := float64(len(p.SplitText(label, 124)))*5 + 3
			if p.GetY()+h > 275 {
				p.AddPage()
				header()
			}
			x, y := p.GetXY()
			p.MultiCell(128, 5, label, "", "L", false)
			p.SetXY(x+128, y)
			p.CellFormat(20, h, row.Weight, "", 0, "R", false, 0, "")
			p.CellFormat(22, h, row.Score, "", 0, "R", false, 0, "")
			p.SetXY(x, y+h)
			p.Line(x, y+h, x+170, y+h)
		}
		p.Ln(4)
	}
	if r.Result != "" {
		p.MultiCell(170, 6, r.Result, "", "L", false)
		p.Ln(3)
	}
	if r.Notes != "" {
		p.MultiCell(170, 5.5, r.Notes, "", "L", false)
		p.Ln(3)
	}
	p.SetFont("DejaVu", "", 9)
	for i, s := range r.Panel {
		h := math.Max(30, 10+float64(len(p.SplitText(s.Role+"\n"+s.Name, 106)))*5)
		if p.GetY()+h > 275 {
			p.AddPage()
		}
		x, y := p.GetXY()
		p.MultiCell(106, 5, s.Role+"\n"+s.Name, "", "L", false)
		if len(s.Signature) > 0 {
			key := fmt.Sprintf("signature-%d", i)
			info := p.RegisterImageOptionsReader(key, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(s.Signature))
			if info != nil {
				w, hImage := info.Extent()
				ratio := w / hImage
				drawW, drawH := 40.0, 16.0
				if ratio > drawW/drawH {
					drawH = drawW / ratio
				} else {
					drawW = drawH * ratio
				}
				p.ImageOptions(key, x+115, y, drawW, drawH, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			}
			p.SetXY(x+110, y+18)
			p.MultiCell(60, 4, s.SignedAt, "", "L", false)
		} else {
			p.SetXY(x+110, y)
			p.MultiCell(60, 5, "Belum dikonfirmasi\n\nTanda tangan: __________", "", "L", false)
		}
		p.SetXY(x, y+h)
		p.Line(x, y+h-3, x+170, y+h-3)
	}
	var b bytes.Buffer
	e := p.Output(&b)
	return b.Bytes(), e
}
