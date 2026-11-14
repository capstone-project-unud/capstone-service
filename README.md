# Capstone service

Go Fiber dan PostgreSQL untuk pendaftaran judul, pembimbing per mahasiswa, keputusan judul/kesediaan, finalisasi, log bimbingan, dan validasi berkas UP. Port 8083. Sesi/identitas diperiksa ke auth/user service; tidak menyimpan kredensial pengguna.

Jalankan bootstrap dari `infrastructure/scripts/bootstrap.py`, lalu Compose utama/dev. `.env.example` memuat konfigurasi driver `local` atau `r2`. Semua endpoint publik diakses melalui gateway/Nuxt BFF.

Panduan repo: [arsitektur dan konfigurasi](docs/architecture.md), [cakupan wajib dan batasnya](docs/coverage.md). Dashboard memakai endpoint ringkasan sesuai peran; kode antarmukanya dikelola dalam repositori frontend yang terpisah.

```powershell
go test ./...
go vet ./...
```

XLSX mempertahankan NIM sebagai teks dan tidak mengubah input menjadi formula. PDF memakai fpdf dan DejaVu dengan lisensi pada `internal/reports/fonts/LICENSE`. Format surat sementara sampai detail resmi diberikan.
