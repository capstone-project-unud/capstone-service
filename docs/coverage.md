# Cakupan alur wajib

| Aturan | Implementasi |
| --- | --- |
| Lima jenis pengguna | ADMIN, DOSEN, MAHASISWA, KOORDINATOR_TA, TU diverifikasi melalui user service |
| Deskripsi disahkan calon pembimbing | Upload PDF dan pernyataan pengesahan wajib sebelum submit; isi/tanda tangan diperiksa manusia |
| Anggota dengan calon dosen masing-masing | 2–5 mahasiswa, dua posisi per mahasiswa, referensi akun aktif diverifikasi |
| Mahasiswa mengubah calon dosen sendiri | Identitas berasal dari sesi; keputusan diulang untuk posisi yang berubah |
| Judul dan kesediaan merupakan dua keputusan | title_status dan confirmation_status beserta pelaku/waktu independen |
| Calon dosen atau koordinator dapat memutuskan | Pemeriksaan hak akses per mahasiswa/posisi; KOPRODI tidak otomatis menjadi koordinator |
| Satu tabel detail mahasiswa–dosen | project_students; kandidat dan penetapan terikat mahasiswa, bukan satu pasangan dosen untuk seluruh kelompok |
| Finalisasi tabel dan set posisi 1/2 | API assignment/finalize untuk koordinator; UI tabel berada di repo frontend |
| Dosen/TU/koordinator melihat dan mengekspor | Daftar berfilter bidang minat; ekspor XLSX semua hasil filter |
| Hasil penetapan Excel/PDF | XLSX asli dan PDF “Penetapan Judul Capstone Mahasiswa”; detail surat sementara |
| Log individu dan cetak | Mahasiswa menginput; dosen terkait memeriksa; PDF satu/semua log sesuai hak baca |
| SIMAK dan surat persetujuan untuk UP | Dua berkas berbeda dan verifikasi TU terpisah; penggantian satu berkas mempertahankan keputusan berkas lain |
| Dashboard per peran | Agregat pribadi atau program studi, tanpa memberi kewenangan mutasi tambahan |

Tabel ini memetakan permintaan pendaftaran sampai verifikasi dua berkas UP. Pedoman lengkap memuat B-100 pada persyaratan seminar proposal, keikutsertaan mata kuliah, dokumen B-100–B-400, penjadwalan/penilaian ujian, revisi akhir, Expo, dan RAG pada rancangan sistem. Modul-modul tersebut belum semuanya tersedia. Karena itu kelengkapan dua berkas tidak diklaim sebagai kepatuhan seluruh pedoman atau keputusan boleh mengikuti ujian.

Belum ada integrasi otomatis SIMAK atau verifikasi kriptografis tanda tangan. Batas ukuran unggahan 10 MB. Ekspor judul dibatasi 10000 judul dan log PDF 1000 log per permintaan. Satu mahasiswa mengikuti satu kelompok pada tahap ini; riwayat lintas periode/kelulusan belum dimodelkan. Kuota storage, pemindaian malware, dan retensi unggahan tanpa relasi belum diotomatisasi.
