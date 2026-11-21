CREATE TABLE capstone_service.project_documents (
 id uuid PRIMARY KEY,
 project_id uuid NOT NULL REFERENCES capstone_service.projects(id),
 series text NOT NULL CHECK (series IN ('B-100','B-200','B-300','B-400')),
 file_id uuid NOT NULL REFERENCES capstone_service.files(id),
 uploaded_by uuid NOT NULL,
 uploaded_by_name varchar(150) NOT NULL,
 summary text NOT NULL DEFAULT '',
 revision_no integer NOT NULL DEFAULT 1,
 status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','REVISION')),
 review_note text NOT NULL DEFAULT '',
 reviewer_id uuid,
 reviewer_name varchar(150) NOT NULL DEFAULT '',
 reviewed_at timestamptz,
 version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(project_id, series)
);
CREATE INDEX project_documents_project ON capstone_service.project_documents(project_id);

CREATE TABLE capstone_service.proposal_seminars (
 id uuid PRIMARY KEY,
 project_id uuid UNIQUE NOT NULL REFERENCES capstone_service.projects(id),
 scheduled_at timestamptz NOT NULL,
 location varchar(250) NOT NULL,
 examiner_1_id uuid NOT NULL,
 examiner_1_name varchar(150) NOT NULL,
 examiner_2_id uuid NOT NULL,
 examiner_2_name varchar(150) NOT NULL,
 examiner_3_id uuid NOT NULL,
 examiner_3_name varchar(150) NOT NULL,
 status text NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED','REVISION','APPROVED','REJECTED')),
 outcome_note text NOT NULL DEFAULT '',
 revision_file_id uuid REFERENCES capstone_service.files(id),
 revision_note text NOT NULL DEFAULT '',
 revision_submitted_at timestamptz,
 scheduled_by uuid NOT NULL,
 decided_by uuid,
 decided_at timestamptz,
 version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK (examiner_1_id <> examiner_2_id AND examiner_1_id <> examiner_3_id AND examiner_2_id <> examiner_3_id)
);

CREATE TABLE capstone_service.seminar_evaluations (
 id uuid PRIMARY KEY,
 seminar_id uuid NOT NULL REFERENCES capstone_service.proposal_seminars(id) ON DELETE CASCADE,
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
 recommendation text NOT NULL CHECK (recommendation IN ('APPROVED','REVISION','REJECTED')),
 notes text NOT NULL DEFAULT '',
 version integer NOT NULL DEFAULT 1,
 submitted_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(seminar_id, student_id, evaluator_id)
);
CREATE INDEX seminar_evaluations_seminar ON capstone_service.seminar_evaluations(seminar_id);
