package reports

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/xml"
	"fmt"
	"github.com/go-pdf/fpdf"
	"strings"
)

//go:embed fonts/DejaVuSansCondensed.ttf
var font []byte

func esc(s string) string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
func col(n int) string {
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}
func XLSX(rows [][]string) ([]byte, error) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Judul Capstone" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
	}
	var sheet strings.Builder
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="15" width="28" customWidth="1"/></cols><sheetData>`)
	for i, row := range rows {
		fmt.Fprintf(&sheet, `<row r="%d">`, i+1)
		for j, v := range row {
			fmt.Fprintf(&sheet, `<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, col(j+1), i+1, esc(v))
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData>`)
	if len(rows) > 0 {
		fmt.Fprintf(&sheet, `<autoFilter ref="A1:%s%d"/>`, col(len(rows[0])), len(rows))
	}
	sheet.WriteString(`</worksheet>`)
	files["xl/worksheets/sheet1.xml"] = sheet.String()
	for name, content := range files {
		f, e := z.Create(name)
		if e != nil {
			return nil, e
		}
		if _, e = f.Write([]byte(content)); e != nil {
			return nil, e
		}
	}
	if e := z.Close(); e != nil {
		return nil, e
	}
	return b.Bytes(), nil
}
func PDF(title string, paragraphs []string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 18, 20)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddUTF8FontFromBytes("DejaVu", "", font)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-14)
		pdf.SetFont("DejaVu", "", 9)
		pdf.CellFormat(170, 5, fmt.Sprintf("Halaman %d", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()
	pdf.SetFont("DejaVu", "", 17)
	pdf.MultiCell(170, 8, title, "", "L", false)
	pdf.Ln(5)
	pdf.SetFont("DejaVu", "", 10)
	for _, p := range paragraphs {
		pdf.MultiCell(170, 5.5, p, "", "L", false)
		pdf.Ln(3)
	}
	var b bytes.Buffer
	e := pdf.Output(&b)
	return b.Bytes(), e
}
