# Cakupan revisi pendaftaran dan ujian proposal

Revisi 19 November 2026 menggunakan enam peran: ADMIN, KOPRODI, DOSEN, MAHASISWA, KOORDINATOR_TA, dan TU. Koprodi merupakan peran tambahan dosen untuk pemantauan, bukan kewenangan finalisasi koordinator.

| Kebutuhan wajib | Implementasi |
| --- | --- |
| Deskripsi mendapat pengesahan calon pembimbing | PDF dan pernyataan pengesahan wajib sebelum pengajuan; isi dan tanda tangan diperiksa manusia |
| Form judul, bidang minat, anggota dan kandidat tiap anggota | Form kelompok/judul; satu tabel `project_students` dengan dua posisi per mahasiswa |
| Mahasiswa memperbarui kandidat miliknya | Identitas dari sesi; perubahan mengulang keputusan pada posisi terkait |
| Persetujuan judul dan kesediaan pembimbing berbeda | `title_status` dan `confirmation_status`, masing-masing memiliki pelaku dan waktu |
| Koordinator memvalidasi dan menetapkan P1/P2 | Tabel finalisasi dengan kontrol versi; Koprodi tidak otomatis mendapat akses mutasi ini |
| Dosen, TU, koordinator melihat/mengekspor judul | Daftar dan XLSX dengan filter bidang minat atau seluruh data |
| Hasil finalisasi judul | XLSX dan PDF Penetapan Judul Capstone Mahasiswa; rincian surat masih menunggu format resmi |
| Log bimbingan mahasiswa dan cetak | Input individu; PDF per log/seluruh log sesuai hak baca |
| Pendaftaran proposal enam berkas | SIMAK diterima, persetujuan pembimbing, sertifikat Inggris, proposal, B100, B200 |
| Validasi TU dan pemantauan mahasiswa | Keputusan/catatan/waktu per berkas; penggantian satu berkas mempertahankan lima keputusan lain |
| Daftar proposal dan ekspor | Dosen, TU, koordinator dan Koprodi; filter bidang minat dan kevalidan; NIM, Nama, Judul, P1, P2, status tiap berkas |
| Jadwal ditetapkan di luar sistem | Mahasiswa mencatat tanggal/waktu WITA, lokasi, dan tiga penguji setelah enam berkas valid |
| P1/P2 otomatis dan jadwal per mahasiswa | Diambil dari penetapan mahasiswa bersangkutan; anggota satu judul dapat memiliki pembimbing dan jadwal berbeda |
| Dokumen ujian | PDF presensi, berita acara, serta form nilai masing-masing P1, P2 dan tiga penguji |
| Penilaian dosen terkait | Hanya lima dosen pada panel yang dapat menilai mahasiswa tersebut; tepat 16 indikator 0–100 |
| Perhitungan angka dan huruf | Server menghitung setelah kelima penilai lengkap menggunakan bobot indikator, bobot panel dan konversi versi tersimpan |
| Template tanda tangan dosen | PNG/JPEG melalui profil, maksimal 2 MB; disimpan privat dan dikonfirmasi terpisah untuk presensi/penilaian |
| Dashboard dan monitoring | Koprodi dipulihkan; kesiapan proposal menggunakan enam berkas; ujian individu masuk agregat dan laporan |

## Acuan penilaian

Sumber: `docs/Panduan Capstone Project PSTI Unud Edisi Agustus.pdf`, Bab 6, Tabel 6.1 dan bagian 6.1. Sistem menggunakan 16 indikator Proyek Akhir I dan kategori: Istimewa >=80, Sangat Baik >=71, Baik >=65, Cukup Baik >=60, Cukup >=55, Kurang Cukup >=50, Kurang >=40, Sangat Kurang <40.

Teks pedoman tidak menetapkan bobot agregasi indikator/panel maupun batas huruf A/B/dst. Admin atau koordinator menerbitkan aturan yang telah disetujui program studi di halaman Ujian Proposal. Form tidak mengisi bobot, nilai mahasiswa, atau konversi huruf secara otomatis. Masing-masing kelompok bobot harus berjumlah 100%. Aturan dibekukan saat penilaian pertama pada suatu ujian. Nilai akhir ditampilkan dua desimal; konversi menggunakan nilai akhir tersebut. Angka dan huruf tidak diterbitkan selama lima penilai belum lengkap. Berita acara menampilkan draf jika belum lengkap.

## API dan penyimpanan

- `GET/POST /proposals`, `PUT /proposals/:id/review`, `GET /proposals/export?interest_id=&validity=` (`ready`, `pending`, `revision`).
- `GET/POST /proposal-exams`, `PUT /proposal-exams/:id/evaluation`, `POST /proposal-exams/:id/attendance`.
- `GET /proposal-exams/:id/report?kind=attendance|minutes|score`; form nilai memerlukan `evaluator_id` dari panel.
- `GET/POST /proposal-grading-policy` (publikasi hanya admin/koordinator).
- `GET/POST /signature`, `GET /signature/preview` (hanya template milik dosen yang sedang masuk).
- Seluruh rute berada di bawah `/api/v1/capstone`. Gateway meneruskan multipart hanya ke `/files` dan `/signature`.

`capstone-service` memiliki data akademik, berkas R2, tanda tangan, penilaian dan PDF/XLSX. `user-service` memiliki profil dan peran; referensi UUID dosen/mahasiswa divalidasi melalui API internal. `auth-service` memiliki sesi/kredensial; gateway menangani batas publik dan rate limit Redis. Tidak ada join lintas database layanan.

Migrasi tambahan: `capstone-service/004_individual_proposal_exams.sql` dan `user-service/004_restore_koprodi_role.sql`. Pendaftaran lama dua berkas dipertahankan, tetapi belum dianggap lengkap sampai empat berkas tambahan diunggah dan diperiksa. Migrasi yang sudah diterapkan tidak diedit.

`POST /sempro` lama mengembalikan 410 karena sistem tidak lagi menyusun jadwal proposal. Data seminar kelompok dan API revisi historis tetap dipertahankan. Jalur Ujian Proposal menggunakan API individu baru. Template tanda tangan saja tidak berarti dosen hadir atau menyetujui nilai. Dokumen menyematkan salinan tanda tangan yang telah dikonfirmasi; penggantian template tidak mengubah persetujuan terdahulu.

## Batas administratif

Tidak ada integrasi otomatis SIMAK atau verifikasi kriptografis tanda tangan. TU memeriksa kesesuaian isi keenam berkas. Rincian kop/nomor surat penetapan dan form resmi Pak Githa belum diberikan; dokumen ujian saat ini memakai susunan identitas, panel dan rubrik pedoman. Satu mahasiswa mengikuti satu kelompok; riwayat lintas kelulusan belum dimodelkan. Pengujian revisi menggunakan database sementara dan storage lokal, terpisah dari Neon/R2.
