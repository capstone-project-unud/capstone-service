-- SOP-004: Data Master Akademik
CREATE TABLE capstone_service.academic_periods (
 id uuid PRIMARY KEY,
 code varchar(40) UNIQUE NOT NULL,
 name varchar(150) NOT NULL,
 semester text NOT NULL CHECK (semester IN ('GANJIL','GENAP','ANTARA')),
 start_date date NOT NULL,
 end_date date NOT NULL CHECK (end_date >= start_date),
 group_open boolean NOT NULL DEFAULT true,
 topic_open boolean NOT NULL DEFAULT true,
 sempro_open boolean NOT NULL DEFAULT true,
 exam_open boolean NOT NULL DEFAULT true,
 is_active boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE capstone_service.document_types (
 code varchar(50) PRIMARY KEY,
 name varchar(200) NOT NULL,
 category text NOT NULL CHECK (category IN ('B_SERIES','PROPOSAL_REPORT','APPROVAL_ADMIN','FINAL_REQUIREMENT','EXPO_OUTPUT')),
 stage text NOT NULL CHECK (stage IN ('TOPIK','SEMPRO','BIMBINGAN','UJIAN_AKHIR','EXPO')),
 scope text NOT NULL CHECK (scope IN ('GROUP','INDIVIDUAL')),
 description text NOT NULL DEFAULT '',
 accepted_mime text NOT NULL DEFAULT 'application/pdf',
 is_required boolean NOT NULL DEFAULT true,
 is_active boolean NOT NULL DEFAULT true,
 sort_order integer NOT NULL DEFAULT 100,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE capstone_service.stage_requirements (
 stage text PRIMARY KEY CHECK (stage IN ('KELOMPOK','SEMINAR_TOPIK','SEMINAR_PROPOSAL','UJIAN_AKHIR','EXPO')),
 label varchar(120) NOT NULL,
 min_group_members smallint NOT NULL DEFAULT 2,
 max_group_members smallint NOT NULL DEFAULT 5,
 min_sks integer NOT NULL DEFAULT 0,
 min_grade varchar(5) NOT NULL DEFAULT 'C',
 min_guidance_logs integer NOT NULL DEFAULT 0,
 max_plagiarism_pct integer NOT NULL DEFAULT 20,
 max_assignment_age_days integer NOT NULL DEFAULT 365,
 required_doc_codes jsonb NOT NULL DEFAULT '[]',
 notes text NOT NULL DEFAULT '',
 updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE capstone_service.assessment_rubrics (
 id uuid PRIMARY KEY,
 stage text NOT NULL CHECK (stage IN ('PROYEK_AKHIR_1','PROYEK_AKHIR_2')),
 cpmk_code varchar(20) NOT NULL,
 ik_code varchar(20) NOT NULL,
 cpmk_title varchar(250) NOT NULL,
 ik_description text NOT NULL,
 weight numeric(5,2) NOT NULL DEFAULT 6.25,
 max_score numeric(5,2) NOT NULL DEFAULT 100,
 is_active boolean NOT NULL DEFAULT true,
 sort_order integer NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(stage, ik_code)
);

-- Seed Periode Akademik, Bidang Minat resmi PSTI, Jenis Dokumen lengkap, Persyaratan Tahap, dan Rubrik PA I & PA II
INSERT INTO capstone_service.academic_periods(id, code, name, semester, start_date, end_date, group_open, topic_open, sempro_open, exam_open, is_active) VALUES
 ('11000000-0000-4000-8000-000000000001', '2026-GENAP', 'Tahun Akademik 2025/2026 Genap', 'GENAP', '2026-02-01', '2026-07-31', true, true, true, true, true),
 ('11000000-0000-4000-8000-000000000002', '2026-GANJIL', 'Tahun Akademik 2026/2027 Ganjil', 'GANJIL', '2026-09-01', '2027-01-31', true, true, true, true, false)
ON CONFLICT (code) DO NOTHING;

INSERT INTO capstone_service.interest_fields(id, name, is_active) VALUES
 ('12000000-0000-4000-8000-000000000001', 'Data Sains dan Sistem Cerdas', true),
 ('12000000-0000-4000-8000-000000000002', 'Tata Kelola dan Bisnis Teknologi Informasi', true),
 ('12000000-0000-4000-8000-000000000003', 'Sistem Informasi', true),
 ('12000000-0000-4000-8000-000000000004', 'IoT dan Jaringan', true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO capstone_service.document_types(code, name, category, stage, scope, description, is_required, is_active, sort_order) VALUES
 ('USULAN_TOPIK', 'Usulan Topik / Proposal Awal Bertanda Tangan', 'APPROVAL_ADMIN', 'TOPIK', 'GROUP', 'Dokumen usulan topik sesuai format bidang minat dengan tanda tangan calon Pembimbing 1 (SOP-006/007).', true, true, 10),
 ('B-100', 'B-100 · Dokumen Identifikasi Masalah', 'B_SERIES', 'SEMPRO', 'GROUP', 'Analisis kontekstual, kompleksitas masalah teknis/non-teknis/K3, dan rumusan masalah (Prasyarat Sempro).', true, true, 20),
 ('B-200', 'B-200 · Dokumen Spesifikasi dan Perancangan Desain', 'B_SERIES', 'BIMBINGAN', 'GROUP', 'Spesifikasi terukur, standar rekayasa, trade-off analysis, dan desain rinci. Wajib disetujui sebelum membeli komponen/pemrograman.', true, true, 30),
 ('B-300', 'B-300 · Dokumen Spesifikasi dan Implementasi Produk', 'B_SERIES', 'UJIAN_AKHIR', 'GROUP', 'Manajemen proyek, K3, bukti realisasi PCB/kode/FGD, dan panduan penggunaan (User Guide / SOP alat).', true, true, 40),
 ('B-400', 'B-400 · Dokumen Pengujian dan Validasi Desain', 'B_SERIES', 'UJIAN_AKHIR', 'GROUP', 'Skenario uji, grafik Target vs Aktual, failure analysis, evaluasi standar, dan kesimpulan kelayakan.', true, true, 50),
 ('PROPOSAL_USULAN', 'Naskah Usulan / Proposal Capstone Project', 'PROPOSAL_REPORT', 'SEMPRO', 'GROUP', 'Naskah usulan lengkap Bab I–III beserta Lampiran A (B-100), B (Datasheet), dan C (Sumber Daya).', true, true, 60),
 ('PERSETUJUAN_SEMPRO', 'Lembar Persetujuan Maju Seminar Proposal (Lampiran 3a)', 'APPROVAL_ADMIN', 'SEMPRO', 'GROUP', 'Lembar persetujuan maju Seminar Proposal yang ditandatangani Pembimbing 1 dan Pembimbing 2.', true, true, 70),
 ('PENGESAHAN_REVISI_SEMPRO', 'Lembar Pengesahan Revisi Usulan Proposal', 'APPROVAL_ADMIN', 'SEMPRO', 'GROUP', 'Bukti penyelesaian seluruh poin revisi penguji pasca-Sempro yang disahkan penguji & Koordinator Capstone.', true, true, 80),
 ('SURAT_TUGAS', 'Surat Tugas Dosen Pembimbing Capstone', 'APPROVAL_ADMIN', 'BIMBINGAN', 'INDIVIDUAL', 'Dokumen Surat Tugas pembimbing yang diterbitkan institusi (berlaku < 1 tahun saat pendaftaran Ujian Akhir).', true, true, 90),
 ('LAPORAN_AKHIR', 'Naskah Laporan Akhir Capstone Project (Buku Capstone)', 'PROPOSAL_REPORT', 'UJIAN_AKHIR', 'GROUP', 'Laporan lengkap Bab I–V, Abstrak ID/EN, Pernyataan Orisinalitas, serta Lampiran A–E.', true, true, 100),
 ('BUKTI_PLAGIARISME', 'Bukti Uji Plagiarisme (Turnitin < 20%)', 'APPROVAL_ADMIN', 'UJIAN_AKHIR', 'GROUP', 'Laporan hasil pemeriksaan kemiripan naskah dengan batas similarity index < 20%.', true, true, 110),
 ('JURNAL_ILMIAH', 'Artikel Jurnal Ilmiah Nasional / Internasional', 'EXPO_OUTPUT', 'UJIAN_AKHIR', 'GROUP', 'Naskah/bukti submit atau publikasi jurnal ilmiah (Nasional, SINTA, Internasional, atau Bereputasi).', false, true, 120),
 ('PERSETUJUAN_UA', 'Lembar Persetujuan Maju Ujian Akhir (Lampiran 3b)', 'APPROVAL_ADMIN', 'UJIAN_AKHIR', 'GROUP', 'Lembar persetujuan maju Ujian Akhir bertanggal terbaru ditandatangani Pembimbing 1 & Pembimbing 2.', true, true, 130),
 ('PENGESAHAN_REVISI_UA', 'Lembar Pengesahan Revisi Laporan Akhir', 'APPROVAL_ADMIN', 'UJIAN_AKHIR', 'GROUP', 'Lembar pengesahan penyelesaian revisi Ujian Akhir bertanda tangan seluruh penguji.', true, true, 140),
 ('SYARAT_UA_INDIVIDU', 'Bundel Persyaratan Administrasi Individu Ujian Akhir', 'FINAL_REQUIREMENT', 'UJIAN_AKHIR', 'INDIVIDUAL', 'Transkrip+KHS (>=140 SKS, min C), SPP terakhir, biodata, salinan ijazah, foto 4x6 B/W, dan surat diketahui PA.', true, true, 150),
 ('EXPO_POSTER', 'Poster Ilmiah / X-Banner Expo (60 x 160 cm)', 'EXPO_OUTPUT', 'EXPO', 'GROUP', 'Poster visual berisi masalah, blok diagram sistem, hasil pengujian utama, dan kesimpulan.', true, true, 160),
 ('EXPO_VIDEO', 'Ringkasan Video Demonstrasi & Repositori Expo', 'EXPO_OUTPUT', 'EXPO', 'GROUP', 'Dokumen/bukti video demo real-time (5–15 menit) beserta tautan repositori showcase.', false, true, 170)
ON CONFLICT (code) DO NOTHING;

INSERT INTO capstone_service.stage_requirements(stage, label, min_group_members, max_group_members, min_sks, min_grade, min_guidance_logs, max_plagiarism_pct, max_assignment_age_days, required_doc_codes, notes) VALUES
 ('KELOMPOK', 'Pembentukan Kelompok (SOP-005)', 2, 5, 0, 'C', 0, 20, 365, '[]', 'Minimal 2 dan maksimal 5 mahasiswa aktif dengan 1 Ketua Kelompok dan pembagian tugas yang jelas.'),
 ('SEMINAR_TOPIK', 'Seminar Kelayakan Topik (SOP-008)', 2, 5, 0, 'C', 0, 20, 365, '["USULAN_TOPIK"]', 'Usulan topik disetujui calon Pembimbing 1, lolos verifikasi administrasi, dan dipaparkan pada Tim Bidang Minat.'),
 ('SEMINAR_PROPOSAL', 'Seminar Proposal / Proyek Akhir I (SOP-012)', 2, 5, 0, 'C', 1, 20, 365, '["B-100","PROPOSAL_USULAN","PERSETUJUAN_SEMPRO"]', 'Memiliki Pembimbing 1 & 2, bimbingan rutin, B-100 disetujui, berkas UP (SIMAK + Persetujuan) valid.'),
 ('UJIAN_AKHIR', 'Ujian Akhir / Proyek Akhir II (SOP-015)', 2, 5, 140, 'C', 1, 20, 365, '["B-100","B-200","B-300","B-400","PENGESAHAN_REVISI_SEMPRO","LAPORAN_AKHIR","BUKTI_PLAGIARISME","PERSETUJUAN_UA"]', 'Memenuhi 11 syarat Ujian Akhir: >=140 SKS nilai min C, revisi Sempro tuntas, B-100..B-400 terbaru, Turnitin <20%, Surat Tugas <1 tahun.'),
 ('EXPO', 'Capstone Expo & Publikasi Karya (SOP-018)', 2, 5, 140, 'C', 1, 20, 365, '["EXPO_POSTER"]', 'Lulus Ujian Akhir, menuntaskan revisi akhir, menyiapkan Poster X-Banner 60x160 cm, video demo, dan repositori GitHub.')
ON CONFLICT (stage) DO NOTHING;

INSERT INTO capstone_service.assessment_rubrics(id, stage, cpmk_code, ik_code, cpmk_title, ik_description, weight, sort_order) VALUES
 ('13000000-0000-4000-8000-000000000101', 'PROYEK_AKHIR_1', 'CPMK-1', 'IK-1a', 'Analisis Persoalan Kompleks & Disiplin Ilmu TI', 'Mampu menganalisis persoalan computing yang kompleks yang melibatkan banyak komponen dan berisiko tinggi pada proposal.', 6.25, 1),
 ('13000000-0000-4000-8000-000000000102', 'PROYEK_AKHIR_1', 'CPMK-1', 'IK-1b', 'Analisis Persoalan Kompleks & Disiplin Ilmu TI', 'Mampu memahami prinsip-prinsip computing mencakup komputasi, komunikasi, dan desain.', 6.25, 2),
 ('13000000-0000-4000-8000-000000000103', 'PROYEK_AKHIR_1', 'CPMK-1', 'IK-1c', 'Analisis Persoalan Kompleks & Disiplin Ilmu TI', 'Mampu menerapkan disiplin ilmu teknologi informasi yang relevan pada usulan proyek.', 6.25, 3),
 ('13000000-0000-4000-8000-000000000104', 'PROYEK_AKHIR_1', 'CPMK-2', 'IK-2a', 'Desain, Implementasi, Pengujian & Evaluasi Solusi TI', 'Mampu menerapkan ilmu dan teknologi informasi pada proses penyelesaian studi kasus.', 6.25, 4),
 ('13000000-0000-4000-8000-000000000105', 'PROYEK_AKHIR_1', 'CPMK-2', 'IK-2b', 'Desain, Implementasi, Pengujian & Evaluasi Solusi TI', 'Mampu memenuhi kebutuhan masyarakat dengan menggunakan ilmu dan teknologi informasi.', 6.25, 5),
 ('13000000-0000-4000-8000-000000000106', 'PROYEK_AKHIR_1', 'CPMK-2', 'IK-2c', 'Desain, Implementasi, Pengujian & Evaluasi Solusi TI', 'Mampu melakukan proses desain mencakup analisis persoalan, pemodelan, penetapan kebutuhan, hingga rencana evaluasi.', 6.25, 6),
 ('13000000-0000-4000-8000-000000000107', 'PROYEK_AKHIR_1', 'CPMK-3', 'IK-3a', 'Komunikasi Lisan dan Tulisan Efektif', 'Mampu menunjukkan keterampilan komunikasi lisan aktif dan efektif dalam pemaparan proposal.', 6.25, 7),
 ('13000000-0000-4000-8000-000000000108', 'PROYEK_AKHIR_1', 'CPMK-3', 'IK-3b', 'Komunikasi Lisan dan Tulisan Efektif', 'Memiliki kemampuan tertulis yang sistematis, ilmiah, dan taat pada panduan penulisan.', 6.25, 8),
 ('13000000-0000-4000-8000-000000000109', 'PROYEK_AKHIR_1', 'CPMK-4', 'IK-4a', 'Tanggung Jawab Profesional, Prinsip Legal & Etika', 'Memiliki tanggung jawab profesional meliputi hubungan teknologi, masyarakat, dan profesi TI.', 6.25, 9),
 ('13000000-0000-4000-8000-000000000110', 'PROYEK_AKHIR_1', 'CPMK-4', 'IK-4b', 'Tanggung Jawab Profesional, Prinsip Legal & Etika', 'Mampu memahami isu prinsip legal seperti hak cipta, lisensi, dan perlindungan data.', 6.25, 10),
 ('13000000-0000-4000-8000-000000000111', 'PROYEK_AKHIR_1', 'CPMK-4', 'IK-4c', 'Tanggung Jawab Profesional, Prinsip Legal & Etika', 'Mampu memahami isu prinsip etika, integritas akademik, dan bebas konflik kepentingan.', 6.25, 11),
 ('13000000-0000-4000-8000-000000000112', 'PROYEK_AKHIR_1', 'CPMK-5', 'IK-5a', 'Kerja Sama Anggota dan Pemimpin Tim', 'Mampu bekerja sama dalam tim dengan matriks peran pemimpin/anggota dan tanggung jawab proporsional.', 6.25, 12),
 ('13000000-0000-4000-8000-000000000113', 'PROYEK_AKHIR_1', 'CPMK-5', 'IK-5b', 'Kerja Sama Anggota dan Pemimpin Tim', 'Mampu merencanakan evaluasi kemajuan dan efektivitas kinerja tim.', 6.25, 13),
 ('13000000-0000-4000-8000-000000000114', 'PROYEK_AKHIR_1', 'CPMK-6', 'IK-6a', 'Kebutuhan Pengguna dan Integrasi Standar TI', 'Mampu mengidentifikasi dan menganalisis kebutuhan pengguna secara empiris untuk sistem TI.', 6.25, 14),
 ('13000000-0000-4000-8000-000000000115', 'PROYEK_AKHIR_1', 'CPMK-6', 'IK-6b', 'Kebutuhan Pengguna dan Integrasi Standar TI', 'Mampu mengevaluasi dan memilih alternatif solusi atau standar teknologi informasi yang tepat.', 6.25, 15),
 ('13000000-0000-4000-8000-000000000116', 'PROYEK_AKHIR_1', 'CPMK-6', 'IK-6c', 'Kebutuhan Pengguna dan Integrasi Standar TI', 'Mampu merencanakan integrasi, pengelolaan, keamanan, dan dokumentasi sistem computing.', 6.25, 16),
 ('13000000-0000-4000-8000-000000000201', 'PROYEK_AKHIR_2', 'CPMK-1', 'IK-1a', 'Analisis Persoalan Kompleks & Disiplin Ilmu TI', 'Produk/sistem akhir terbukti memecahkan persoalan komputasi kompleks kritis dengan data empiris implementasi.', 6.25, 1),
 ('13000000-0000-4000-8000-000000000202', 'PROYEK_AKHIR_2', 'CPMK-1', 'IK-1b', 'Analisis Persoalan Kompleks & Disiplin Ilmu TI', 'Produk akhir menunjukkan penerapan nyata prinsip komputasi, komunikasi data, dan arsitektur desain.', 6.25, 2),
 ('13000000-0000-4000-8000-000000000203', 'PROYEK_AKHIR_2', 'CPMK-1', 'IK-1c', 'Analisis Persoalan Kompleks & Disiplin Ilmu TI', 'Disiplin ilmu TI terwujud menjadi solusi produk yang berfungsi penuh dan sesuai teori.', 6.25, 3),
 ('13000000-0000-4000-8000-000000000204', 'PROYEK_AKHIR_2', 'CPMK-2', 'IK-2a', 'Desain, Implementasi, Pengujian & Evaluasi Solusi TI', 'Solusi TI yang dibangun berfungsi efektif menyelesaikan studi kasus awal.', 6.25, 4),
 ('13000000-0000-4000-8000-000000000205', 'PROYEK_AKHIR_2', 'CPMK-2', 'IK-2b', 'Desain, Implementasi, Pengujian & Evaluasi Solusi TI', 'Produk akhir diuji kepada pengguna sasaran dan terbukti memenuhi kebutuhan masyarakat.', 6.25, 5),
 ('13000000-0000-4000-8000-000000000206', 'PROYEK_AKHIR_2', 'CPMK-2', 'IK-2c', 'Desain, Implementasi, Pengujian & Evaluasi Solusi TI', 'Seluruh siklus desain, implementasi kode/alat, pengujian, dan failure analysis berjalan robust dan terdokumentasi.', 6.25, 6),
 ('13000000-0000-4000-8000-000000000207', 'PROYEK_AKHIR_2', 'CPMK-3', 'IK-3a', 'Komunikasi Lisan dan Tulisan Efektif', 'Demonstrasi produk dan presentasi hasil akhir disampaikan artikulatif dan berbasis data akurat.', 6.25, 7),
 ('13000000-0000-4000-8000-000000000208', 'PROYEK_AKHIR_2', 'CPMK-3', 'IK-3b', 'Komunikasi Lisan dan Tulisan Efektif', 'Laporan Akhir (Buku Capstone) tertulis sistematis, analisis hasil tajam, bebas salah format.', 6.25, 8),
 ('13000000-0000-4000-8000-000000000209', 'PROYEK_AKHIR_2', 'CPMK-4', 'IK-4a', 'Tanggung Jawab Profesional, Prinsip Legal & Etika', 'Bukti nyata tanggung jawab profesi, keamanan pengguna, panduan penggunaan, dan analisis dampak sosio-teknologis.', 6.25, 9),
 ('13000000-0000-4000-8000-000000000210', 'PROYEK_AKHIR_2', 'CPMK-4', 'IK-4b', 'Tanggung Jawab Profesional, Prinsip Legal & Etika', 'Kepatuhan prinsip legal (hak cipta, lisensi open-source, regulasi ITE/PDP) terdokumentasi pada produk akhir.', 6.25, 10),
 ('13000000-0000-4000-8000-000000000211', 'PROYEK_AKHIR_2', 'CPMK-4', 'IK-4c', 'Tanggung Jawab Profesional, Prinsip Legal & Etika', 'Integritas akademik dan kejujuran data hasil pengujian tanpa fabrikasi atau falsifikasi data.', 6.25, 11),
 ('13000000-0000-4000-8000-000000000212', 'PROYEK_AKHIR_2', 'CPMK-5', 'IK-5a', 'Kerja Sama Anggota dan Pemimpin Tim', 'Realisasi kontribusi kerja tim proporsional sesuai peran dan seluruh modul terintegrasi baik.', 6.25, 12),
 ('13000000-0000-4000-8000-000000000213', 'PROYEK_AKHIR_2', 'CPMK-5', 'IK-5b', 'Kerja Sama Anggota dan Pemimpin Tim', 'Evaluasi akhir kinerja tim terdokumentasi dengan metrik pencapaian dan penyelesaian kendala internal.', 6.25, 13),
 ('13000000-0000-4000-8000-000000000214', 'PROYEK_AKHIR_2', 'CPMK-6', 'IK-6a', 'Kebutuhan Pengguna dan Integrasi Standar TI', 'Hasil UAT menunjukkan produk akhir memenuhi seluruh kebutuhan pengguna yang dianalisis.', 6.25, 14),
 ('13000000-0000-4000-8000-000000000215', 'PROYEK_AKHIR_2', 'CPMK-6', 'IK-6b', 'Kebutuhan Pengguna dan Integrasi Standar TI', 'Standar TI yang diterapkan terbukti stabil, andal, dan sesuai beban kerja pengguna.', 6.25, 15),
 ('13000000-0000-4000-8000-000000000216', 'PROYEK_AKHIR_2', 'CPMK-6', 'IK-6c', 'Kebutuhan Pengguna dan Integrasi Standar TI', 'Sistem akhir terintegrasi utuh, aman, dan disertai dokumentasi administrasi serta pemeliharaan lengkap.', 6.25, 16)
ON CONFLICT (stage, ik_code) DO NOTHING;

-- SOP-005: Pembentukan Kelompok Capstone Project terlebih dahulu sebelum Usulan Topik
CREATE TABLE capstone_service.capstone_groups (
 id uuid PRIMARY KEY,
 code varchar(40) UNIQUE NOT NULL,
 academic_period_id uuid NOT NULL REFERENCES capstone_service.academic_periods(id),
 interest_id uuid NOT NULL REFERENCES capstone_service.interest_fields(id),
 working_title varchar(500) NOT NULL,
 initial_description text NOT NULL DEFAULT '',
 leader_id uuid NOT NULL,
 status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','READY','LOCKED')),
 version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE capstone_service.capstone_group_members (
 group_id uuid NOT NULL REFERENCES capstone_service.capstone_groups(id) ON DELETE CASCADE,
 student_id uuid NOT NULL,
 student_name varchar(150) NOT NULL,
 nim varchar(30) NOT NULL,
 is_leader boolean NOT NULL DEFAULT false,
 responsibility text NOT NULL DEFAULT '',
 joined_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (group_id, student_id)
);
CREATE INDEX capstone_group_members_student ON capstone_service.capstone_group_members(student_id);

-- Tautkan projects & project_students ke kelompok, periode akademik, verifikasi topik (SOP-007), dan Surat Tugas (SOP-009)
ALTER TABLE capstone_service.projects
 ADD COLUMN IF NOT EXISTS group_id uuid REFERENCES capstone_service.capstone_groups(id),
 ADD COLUMN IF NOT EXISTS academic_period_id uuid REFERENCES capstone_service.academic_periods(id),
 ADD COLUMN IF NOT EXISTS topic_verify_status text NOT NULL DEFAULT 'PENDING' CHECK (topic_verify_status IN ('PENDING','VALID','REVISION')),
 ADD COLUMN IF NOT EXISTS topic_verify_note text NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS topic_verified_by uuid,
 ADD COLUMN IF NOT EXISTS topic_verified_at timestamptz,
 ADD COLUMN IF NOT EXISTS assignment_letter_file_id uuid REFERENCES capstone_service.files(id),
 ADD COLUMN IF NOT EXISTS assignment_letter_number varchar(100) NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS assignment_letter_issued_at date;

ALTER TABLE capstone_service.project_students
 ADD COLUMN IF NOT EXISTS academic_period_id uuid REFERENCES capstone_service.academic_periods(id),
 ADD COLUMN IF NOT EXISTS is_leader boolean NOT NULL DEFAULT false,
 ADD COLUMN IF NOT EXISTS responsibility text NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS is_external_supervisor boolean NOT NULL DEFAULT false;

ALTER TABLE capstone_service.project_students DROP CONSTRAINT IF EXISTS project_students_student_id_slot_key;
ALTER TABLE capstone_service.proposal_applications DROP CONSTRAINT IF EXISTS proposal_applications_student_id_key;
ALTER TABLE capstone_service.proposal_applications ADD CONSTRAINT proposal_applications_project_student_key UNIQUE(project_id, student_id);

-- Backfill any pre-existing projects cleanly into capstone_groups so existing rows always have group_id
INSERT INTO capstone_service.capstone_groups(id, code, academic_period_id, interest_id, working_title, initial_description, leader_id, status)
SELECT p.id, 'GRP-' || upper(substr(replace(p.id::text,'-',''),1,8)), '11000000-0000-4000-8000-000000000001'::uuid, p.interest_id, p.title, p.summary, p.owner_id,
 CASE WHEN p.status IN ('SUBMITTED','FINALIZED') THEN 'LOCKED' ELSE 'READY' END
FROM capstone_service.projects p
WHERE p.group_id IS NULL
ON CONFLICT (id) DO NOTHING;

INSERT INTO capstone_service.capstone_group_members(group_id, student_id, student_name, nim, is_leader, responsibility)
SELECT DISTINCT p.id, m.student_id, m.student_name, m.nim, (m.student_id = p.owner_id), CASE WHEN m.student_id = p.owner_id THEN 'Ketua Kelompok' ELSE 'Anggota Tim' END
FROM capstone_service.projects p
JOIN capstone_service.project_students m ON m.project_id = p.id AND m.slot = 1
ON CONFLICT (group_id, student_id) DO NOTHING;

UPDATE capstone_service.projects SET group_id = id, academic_period_id = coalesce(academic_period_id, '11000000-0000-4000-8000-000000000001'::uuid) WHERE group_id IS NULL;
UPDATE capstone_service.project_students m SET academic_period_id = coalesce(m.academic_period_id, p.academic_period_id), is_leader = (m.student_id = p.owner_id), responsibility = CASE WHEN m.responsibility = '' THEN (CASE WHEN m.student_id = p.owner_id THEN 'Ketua Kelompok' ELSE 'Anggota Tim' END) ELSE m.responsibility END FROM capstone_service.projects p WHERE p.id = m.project_id;
CREATE UNIQUE INDEX IF NOT EXISTS project_students_period_student_slot ON capstone_service.project_students(coalesce(academic_period_id,'11000000-0000-4000-8000-000000000001'::uuid), student_id, slot);

-- SOP-008: Pendaftaran dan Pelaksanaan Seminar Kelayakan Topik
CREATE TABLE capstone_service.topic_seminars (
 id uuid PRIMARY KEY,
 project_id uuid UNIQUE NOT NULL REFERENCES capstone_service.projects(id) ON DELETE CASCADE,
 scheduled_at timestamptz,
 location varchar(250) NOT NULL DEFAULT '',
 panelist_1_id uuid,
 panelist_1_name varchar(150) NOT NULL DEFAULT '',
 panelist_2_id uuid,
 panelist_2_name varchar(150) NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'REGISTERED' CHECK (status IN ('REGISTERED','SCHEDULED','LAYAK','REVISI','DITOLAK')),
 feasibility_notes text NOT NULL DEFAULT '',
 decision_note text NOT NULL DEFAULT '',
 scheduled_by uuid,
 decided_by uuid,
 decided_at timestamptz,
 version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);

-- SOP-011: Perluas Pengelolaan Dokumen (semua jenis dokumen + multi-versi + lingkup individu/kelompok)
ALTER TABLE capstone_service.project_documents DROP CONSTRAINT IF EXISTS project_documents_series_check;
ALTER TABLE capstone_service.project_documents DROP CONSTRAINT IF EXISTS project_documents_project_id_series_key;

ALTER TABLE capstone_service.project_documents
 ADD COLUMN IF NOT EXISTS doc_type_code varchar(50) NOT NULL DEFAULT 'B-100',
 ADD COLUMN IF NOT EXISTS student_id uuid,
 ADD COLUMN IF NOT EXISTS title varchar(300) NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS current_version_no integer NOT NULL DEFAULT 1;

UPDATE capstone_service.project_documents SET doc_type_code = series WHERE doc_type_code IS NULL OR (series <> '' AND doc_type_code = 'B-100' AND series <> 'B-100');
CREATE UNIQUE INDEX IF NOT EXISTS project_documents_unique_scope ON capstone_service.project_documents(project_id, doc_type_code, coalesce(student_id, '00000000-0000-0000-0000-000000000000'::uuid));

CREATE TABLE capstone_service.project_document_versions (
 id uuid PRIMARY KEY,
 document_id uuid NOT NULL REFERENCES capstone_service.project_documents(id) ON DELETE CASCADE,
 version_no integer NOT NULL,
 file_id uuid NOT NULL REFERENCES capstone_service.files(id),
 file_name varchar(200) NOT NULL DEFAULT '',
 summary text NOT NULL DEFAULT '',
 uploaded_by uuid NOT NULL,
 uploaded_by_name varchar(150) NOT NULL,
 review_status text NOT NULL DEFAULT 'PENDING' CHECK (review_status IN ('PENDING','APPROVED','REVISION')),
 review_note text NOT NULL DEFAULT '',
 reviewer_id uuid,
 reviewer_name varchar(150) NOT NULL DEFAULT '',
 reviewed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(document_id, version_no)
);

INSERT INTO capstone_service.project_document_versions(id, document_id, version_no, file_id, file_name, summary, uploaded_by, uploaded_by_name, review_status, review_note, reviewer_id, reviewer_name, reviewed_at, created_at)
SELECT d.id, d.id, greatest(d.revision_no, 1), d.file_id, coalesce(f.name, d.doc_type_code || '.pdf'), d.summary, d.uploaded_by, d.uploaded_by_name, d.status, d.review_note, d.reviewer_id, d.reviewer_name, d.reviewed_at, d.created_at
FROM capstone_service.project_documents d
LEFT JOIN capstone_service.files f ON f.id = d.file_id
ON CONFLICT (document_id, version_no) DO NOTHING;

-- SOP-012, SOP-013, SOP-014 & SOP-017: Persetujuan Maju Sempro, Riwayat Jadwal, & Revisi Per-Catatan Penguji
ALTER TABLE capstone_service.proposal_seminars
 ADD COLUMN IF NOT EXISTS schedule_history jsonb NOT NULL DEFAULT '[]',
 ADD COLUMN IF NOT EXISTS minutes_file_id uuid REFERENCES capstone_service.files(id),
 ADD COLUMN IF NOT EXISTS revision_approved_file_id uuid REFERENCES capstone_service.files(id);

CREATE TABLE capstone_service.sempro_advisor_approvals (
 id uuid PRIMARY KEY,
 project_id uuid NOT NULL REFERENCES capstone_service.projects(id) ON DELETE CASCADE,
 supervisor_id uuid NOT NULL,
 supervisor_name varchar(150) NOT NULL,
 status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','REVISION')),
 note text NOT NULL DEFAULT '',
 signed_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(project_id, supervisor_id)
);

CREATE TABLE capstone_service.seminar_revision_items (
 id uuid PRIMARY KEY,
 stage text NOT NULL CHECK (stage IN ('SEMPRO','FINAL_EXAM')),
 seminar_id uuid NOT NULL,
 project_id uuid NOT NULL REFERENCES capstone_service.projects(id) ON DELETE CASCADE,
 student_id uuid,
 examiner_id uuid NOT NULL,
 examiner_name varchar(150) NOT NULL,
 note text NOT NULL,
 student_response text NOT NULL DEFAULT '',
 response_file_id uuid REFERENCES capstone_service.files(id),
 status text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','SUBMITTED','APPROVED','REVISION')),
 reviewer_note text NOT NULL DEFAULT '',
 reviewed_at timestamptz,
 version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX seminar_revision_items_lookup ON capstone_service.seminar_revision_items(stage, seminar_id, project_id);

-- SOP-015, SOP-016, SOP-017: Pendaftaran 11 Syarat Ujian Akhir, Penjadwalan, dan Penilaian PA II
CREATE TABLE capstone_service.final_exam_applications (
 id uuid PRIMARY KEY,
 project_id uuid NOT NULL REFERENCES capstone_service.projects(id) ON DELETE CASCADE,
 student_id uuid NOT NULL,
 student_name varchar(150) NOT NULL,
 nim varchar(30) NOT NULL,
 sks_completed integer NOT NULL DEFAULT 0 CHECK (sks_completed >= 0 AND sks_completed <= 300),
 min_grade_ok boolean NOT NULL DEFAULT false,
 transcript_file_id uuid REFERENCES capstone_service.files(id),
 spp_file_id uuid REFERENCES capstone_service.files(id),
 biodata_summary text NOT NULL DEFAULT '',
 ijazah_file_id uuid REFERENCES capstone_service.files(id),
 photo_file_id uuid REFERENCES capstone_service.files(id),
 pa_letter_file_id uuid REFERENCES capstone_service.files(id),
 plagiarism_pct numeric(5,2) NOT NULL DEFAULT 0 CHECK (plagiarism_pct >= 0 AND plagiarism_pct <= 100),
 plagiarism_file_id uuid REFERENCES capstone_service.files(id),
 advisor_1_approved boolean NOT NULL DEFAULT false,
 advisor_2_approved boolean NOT NULL DEFAULT false,
 advisor_note text NOT NULL DEFAULT '',
 advisor_approved_at timestamptz,
 tu_status text NOT NULL DEFAULT 'PENDING' CHECK (tu_status IN ('PENDING','VALID','REVISION')),
 tu_note text NOT NULL DEFAULT '',
 tu_reviewer uuid,
 tu_reviewed_at timestamptz,
 coordinator_status text NOT NULL DEFAULT 'PENDING' CHECK (coordinator_status IN ('PENDING','READY','REVISION')),
 coordinator_note text NOT NULL DEFAULT '',
 version integer NOT NULL DEFAULT 1,
 submitted_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(project_id, student_id)
);

CREATE TABLE capstone_service.final_exams (
 id uuid PRIMARY KEY,
 project_id uuid UNIQUE NOT NULL REFERENCES capstone_service.projects(id) ON DELETE CASCADE,
 scheduled_at timestamptz NOT NULL,
 location varchar(250) NOT NULL,
 examiner_1_id uuid NOT NULL,
 examiner_1_name varchar(150) NOT NULL,
 examiner_2_id uuid NOT NULL,
 examiner_2_name varchar(150) NOT NULL,
 examiner_3_id uuid NOT NULL,
 examiner_3_name varchar(150) NOT NULL,
 status text NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED','REVISION','PASSED','FAILED')),
 outcome_note text NOT NULL DEFAULT '',
 minutes_file_id uuid REFERENCES capstone_service.files(id),
 revision_approved_file_id uuid REFERENCES capstone_service.files(id),
 schedule_history jsonb NOT NULL DEFAULT '[]',
 scheduled_by uuid NOT NULL,
 decided_by uuid,
 decided_at timestamptz,
 version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK (examiner_1_id <> examiner_2_id AND examiner_1_id <> examiner_3_id AND examiner_2_id <> examiner_3_id)
);

CREATE TABLE capstone_service.final_exam_evaluations (
 id uuid PRIMARY KEY,
 exam_id uuid NOT NULL REFERENCES capstone_service.final_exams(id) ON DELETE CASCADE,
 project_id uuid NOT NULL REFERENCES capstone_service.projects(id),
 student_id uuid NOT NULL,
 student_name varchar(150) NOT NULL,
 nim varchar(30) NOT NULL,
 evaluator_id uuid NOT NULL,
 evaluator_name varchar(150) NOT NULL,
 evaluator_role text NOT NULL CHECK (evaluator_role IN ('EXAMINER_1','EXAMINER_2','EXAMINER_3','SUPERVISOR')),
 scores jsonb NOT NULL,
 total_score numeric(5,2) NOT NULL CHECK (total_score >= 0 AND total_score <= 100),
 grade_label text NOT NULL,
 recommendation text NOT NULL CHECK (recommendation IN ('PASSED','REVISION','FAILED')),
 notes text NOT NULL DEFAULT '',
 version integer NOT NULL DEFAULT 1,
 submitted_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(exam_id, student_id, evaluator_id)
);

-- SOP-018: Pengajuan dan Publikasi Karya Capstone Expo
CREATE TABLE capstone_service.expo_submissions (
 id uuid PRIMARY KEY,
 project_id uuid UNIQUE NOT NULL REFERENCES capstone_service.projects(id) ON DELETE CASCADE,
 title varchar(500) NOT NULL,
 category text NOT NULL CHECK (category IN ('SOFTWARE','HARDWARE','ALGORITMA','DOKUMEN_STRATEGIS')),
 abstract_id text NOT NULL,
 abstract_en text NOT NULL DEFAULT '',
 poster_file_id uuid REFERENCES capstone_service.files(id),
 video_url text NOT NULL DEFAULT '',
 repo_url text NOT NULL DEFAULT '',
 demo_url text NOT NULL DEFAULT '',
 journal_title varchar(500) NOT NULL DEFAULT '',
 journal_class varchar(100) NOT NULL DEFAULT '',
 journal_url text NOT NULL DEFAULT '',
 publication_consent boolean NOT NULL DEFAULT false,
 status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','SUBMITTED','REVISION','APPROVED','PUBLISHED')),
 review_note text NOT NULL DEFAULT '',
 reviewed_by uuid,
 reviewed_at timestamptz,
 published_at timestamptz,
 version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);

-- SOP-019: Pengelolaan Pengetahuan Resmi & Asisten Pedoman (RAG)
CREATE TABLE capstone_service.knowledge_sources (
 id uuid PRIMARY KEY,
 code varchar(60) NOT NULL,
 title varchar(300) NOT NULL,
 version_label varchar(40) NOT NULL,
 effective_date date NOT NULL,
 approval_proof text NOT NULL DEFAULT '',
 source_file_id uuid REFERENCES capstone_service.files(id),
 raw_text text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','INDEXED','ACTIVE','ARCHIVED')),
 chunk_count integer NOT NULL DEFAULT 0,
 last_test_status text NOT NULL DEFAULT 'UNTESTED',
 uploaded_by uuid NOT NULL,
 activated_by uuid,
 activated_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(code, version_label)
);

CREATE TABLE capstone_service.knowledge_chunks (
 id uuid PRIMARY KEY,
 source_id uuid NOT NULL REFERENCES capstone_service.knowledge_sources(id) ON DELETE CASCADE,
 chunk_index integer NOT NULL,
 section_title varchar(300) NOT NULL,
 page_ref varchar(60) NOT NULL DEFAULT '',
 content text NOT NULL,
 search_vec tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(section_title,'') || ' ' || content)) STORED,
 UNIQUE(source_id, chunk_index)
);
CREATE INDEX knowledge_chunks_search_idx ON capstone_service.knowledge_chunks USING GIN(search_vec);

CREATE TABLE capstone_service.chat_sessions (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL,
 title varchar(200) NOT NULL DEFAULT 'Percakapan Pedoman',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE capstone_service.chat_messages (
 id uuid PRIMARY KEY,
 session_id uuid NOT NULL REFERENCES capstone_service.chat_sessions(id) ON DELETE CASCADE,
 role text NOT NULL CHECK (role IN ('USER','ASSISTANT')),
 content text NOT NULL,
 status text NOT NULL DEFAULT 'ANSWERED' CHECK (status IN ('ANSWERED','INSUFFICIENT_CONTEXT','FAILED')),
 citations jsonb NOT NULL DEFAULT '[]',
 created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO capstone_service.knowledge_sources(id, code, title, version_label, effective_date, approval_proof, status, chunk_count, last_test_status, uploaded_by, activated_at) VALUES
 ('14000000-0000-4000-8000-000000000001', 'PANDUAN-PSTI-UNUD', 'Buku Panduan Capstone Project PSTI Unud Edisi Agustus & SOP Capstone', 'v2026.1', '2026-08-01', 'Disahkan oleh Koordinator Program Studi Teknologi Informasi FT Unud', 'ACTIVE', 8, 'PASSED', '00000000-0000-4000-8000-000000000000', now())
ON CONFLICT (code, version_label) DO NOTHING;

INSERT INTO capstone_service.knowledge_chunks(id, source_id, chunk_index, section_title, page_ref, content) VALUES
 ('14100000-0000-4000-8000-000000000001', '14000000-0000-4000-8000-000000000001', 1, 'Pembentukan Kelompok Capstone Project (SOP-005 & Bab 2.1.1)', 'Hal. 13 · SOP-005', 'Kelompok Capstone Project dibentuk pada Mata Kuliah Studi Proses Bisnis dan Praktik Industri beranggotakan minimal 2 orang dan maksimal 5 mahasiswa aktif. Setiap kelompok wajib menetapkan 1 Ketua Kelompok, bidang minat, judul kerja awal, serta matriks pembagian tanggung jawab setiap anggota sebelum mengajukan usulan topik.'),
 ('14100000-0000-4000-8000-000000000002', '14000000-0000-4000-8000-000000000001', 2, 'Usulan Topik, Pembimbing, dan Seminar Topik (SOP-006 s.d. SOP-009)', 'Hal. 13–14 · SOP-006..009', 'Setelah kelompok terbentuk, mahasiswa berkonsultasi dengan calon Pembimbing 1 dan mengunggah dokumen usulan topik bertanda tangan untuk diverifikasi administrasi dan mengikuti Seminar Kelayakan Topik pada Tim Bidang Minat. Setiap mahasiswa dalam kelompok memiliki Pembimbing 1 dan Pembimbing 2 yang ditetapkan secara resmi oleh Koordinator Capstone beserta Surat Tugas.'),
 ('14100000-0000-4000-8000-000000000003', '14000000-0000-4000-8000-000000000001', 3, 'Dokumen Pendukung Seri B-100 s.d. B-400 (Bab 4 & SOP-011)', 'Hal. 21–23 · SOP-011', 'Dokumen B-Series disusun bertahap pada semester 7–8: B-100 Identifikasi Masalah wajib diselesaikan sebagai prasyarat Seminar Proposal; B-200 Spesifikasi dan Perancangan Desain wajib disetujui pembimbing sebelum membeli komponen atau memulai pemrograman; B-300 Dokumentasi Implementasi Produk dan B-400 Pengujian & Validasi Desain wajib dilengkapi revisi terbarunya sebelum mendaftar Ujian Akhir.'),
 ('14100000-0000-4000-8000-000000000004', '14000000-0000-4000-8000-000000000001', 4, 'Syarat Pendaftaran Seminar Proposal / Proyek Akhir I (SOP-012 & Lampiran 3a)', 'Hal. 13, 65 · SOP-012', 'Pendaftaran Seminar Proposal (Proyek Akhir I, 3 SKS) mensyaratkan kelompok telah memiliki 2 dosen pembimbing tetap, logbook bimbingan rutin tervalidasi, Dokumen B-100 disetujui, Naskah Proposal Usulan Bab I–III, berkas persyaratan UP (bukti SIMAK oleh TU dan Surat Persetujuan Pembimbing), serta Lembar Persetujuan Maju Sempro bertanda tangan Pembimbing 1 & 2.'),
 ('14100000-0000-4000-8000-000000000005', '14000000-0000-4000-8000-000000000001', 5, 'Syarat Pendaftaran Ujian Akhir / Proyek Akhir II - 11 Ketentuan (SOP-015 & Bab 2.2)', 'Hal. 14–15 · SOP-015', 'Ujian Akhir (Proyek Akhir II, 4 SKS) mengharuskan 11 syarat per mahasiswa: (1) Transkrip & KHS >= 140 SKS lulus nilai minimal C, (2) Kuitansi SPP terakhir, (3) Biodata Ujian Akhir, (4) Salinan ijazah terakhir, (5) Pas foto hitam-putih 4x6, (6) Kartu bimbingan tervalidasi, (7) Fotokopi Surat Tugas pembimbing berusia < 1 tahun, (8) Surat permohonan maju ujian diketahui PA, (9) Lembar Pengesahan Revisi Sempro bertanda tangan penguji, (10) B-100 s.d. B-400 revisi terbaru + Laporan Akhir dengan uji plagiarisme Turnitin < 20%, (11) Lembar Persetujuan Maju Ujian Akhir terbaru bertanda tangan Pembimbing 1 & 2.'),
 ('14100000-0000-4000-8000-000000000006', '14000000-0000-4000-8000-000000000001', 6, 'Rubrik Penilaian CPMK & Skala Nilai Proyek Akhir I dan II (Bab 6 & Tabel 6.1)', 'Hal. 38–58 · Tabel 6.1', 'Penilaian individu pada Seminar Proposal dan Ujian Akhir menggunakan 6 CPMK dan 16 Indikator Kinerja (IK-1a..IK-6c). Skala penilaian Tabel 6.1: Istimewa (80–100 / A), Sangat Baik (71–79 / B+), Baik (65–70 / B), Cukup Baik (60–64 / C+), Cukup (55–59 / C), Kurang Cukup (50–54 / D+), Kurang (40–49 / D), Sangat Kurang (0–39 / E). Setiap anggota tim dinilai secara individual oleh 3 dosen penguji dan pembimbing.'),
 ('14100000-0000-4000-8000-000000000007', '14000000-0000-4000-8000-000000000001', 7, 'Luaran Capstone Expo & Publikasi Karya (Bab 3.4 & SOP-018)', 'Hal. 16–20 · SOP-018', 'Mahasiswa yang telah lulus Ujian Akhir dan menyelesaikan revisi wajib mengikuti Capstone Expo dengan menyiapkan Poster Ilmiah/X-Banner standar ukuran 60 x 160 cm, Video Demonstrasi Produk (durasi 5–15 menit), serta mengunggah hasil karya ke repositori GitHub. Publikasi ke galeri publik memerlukan persetujuan izin publikasi oleh Admin.'),
 ('14100000-0000-4000-8000-000000000008', '14000000-0000-4000-8000-000000000001', 8, 'Logbook Bimbingan & Validasi SIMAK (SOP-010 & Logbook Unud)', 'SOP-010 · Catatan SIMAK', 'Logbook bimbingan dicatat per individu mahasiswa kepada pembimbing terkait (Pembimbing 1 atau 2). Persetujuan logbook pada aplikasi berfungsi sebagai rekam jejak bimbingan dan dapat dicetak ke PDF per log maupun seluruh log; validasi akhir di sistem SIMAK tetap mengikuti prosedur resmi universitas.')
ON CONFLICT (source_id, chunk_index) DO NOTHING;
