package reports

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProposalExamDocuments(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 240, 80))
	for x := 10; x < 230; x++ {
		im.Set(x, 20+x%35, color.NRGBA{A: 255})
	}
	var signature bytes.Buffer
	_ = png.Encode(&signature, im)
	base := ExamReport{Reference: "contoh-ujian-0001", Student: "Mahasiswa Pengujian dengan Nama Panjang", NIM: "001900001", Title: strings.Repeat("Rancangan sistem informasi capstone dengan pembimbing berbeda per mahasiswa. ", 4), Date: "19 November 2026, 09:00", Location: "Ruang Sidang Program Studi Teknologi Informasi", Policy: "Aturan sintetis uji dokumen", Result: "Nilai akhir: 72.00 · Huruf: B · SANGAT BAIK"}
	for i := 0; i < 5; i++ {
		base.Panel = append(base.Panel, ExamSignatory{Name: strings.Repeat("Dr. Nama Dosen Pengujian, S.T., M.T. ", 2), Role: "Dosen pembimbing / penguji", Signature: signature.Bytes(), SignedAt: "19/11/2026 10:30 WITA"})
	}
	for _, kind := range []string{"Presensi Ujian Seminar Proposal", "Berita Acara Ujian Seminar Proposal", "Form Nilai Seminar Proposal"} {
		r := base
		r.Kind = kind
		if strings.HasPrefix(kind, "Form Nilai") {
			r.Panel = r.Panel[:1]
			for i := 0; i < 16; i++ {
				r.Scores = append(r.Scores, ExamScoreRow{Code: "IK-1a", Label: "Deskripsi indikator dengan teks panjang untuk memeriksa pembungkusan teks dalam kolom penilaian.", Weight: "6.25", Score: "80.00"})
			}
		}
		b, e := ProposalExamPDF(r)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.HasPrefix(b, []byte("%PDF-")) || !bytes.Contains(b, []byte("/Subtype /Image")) {
			t.Fatal("PDF or embedded signature missing")
		}
		if dir := os.Getenv("PDF_QA_DIR"); dir != "" {
			if e = os.MkdirAll(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(kind, " ", "-")+".pdf"), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
}
