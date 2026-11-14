package reports

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestExcelPreservesTextAndDoesNotCreateFormulas(t *testing.T) {
	b, e := XLSX([][]string{{"NIM", "Judul"}, {"0012345", "=HYPERLINK(\"https://example.com\")"}, {"é & <judul>", "data"}})
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range z.File {
		if f.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		r, _ := f.Open()
		body, _ := io.ReadAll(r)
		r.Close()
		s := string(body)
		if !strings.Contains(s, "0012345") || strings.Contains(s, "<f>") || !strings.Contains(s, "&amp;") {
			t.Fatal(s)
		}
		found = true
	}
	if !found {
		t.Fatal("worksheet missing")
	}
}
func TestPDFLongUnicodeContentProducesMultiplePages(t *testing.T) {
	paragraphs := make([]string, 35)
	for i := range paragraphs {
		paragraphs[i] = strings.Repeat("Bimbingan mahasiswa é: kemajuan dan rencana berikutnya. ", 10)
	}
	b, e := PDF("Log Bimbingan Capstone", paragraphs)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) || bytes.Count(b, []byte("/Type /Page\n")) < 2 {
		t.Fatal("invalid or truncated PDF")
	}
}
