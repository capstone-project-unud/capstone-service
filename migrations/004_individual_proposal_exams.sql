CREATE TABLE capstone_service.proposal_documents (
 application_id uuid NOT NULL REFERENCES capstone_service.proposal_applications(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('simak','approval','english','proposal','b100','b200')),
 file_id uuid NOT NULL REFERENCES capstone_service.files(id),
 status text NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','VALID','REVISION')),
 note text NOT NULL DEFAULT '', reviewer uuid, reviewed_at timestamptz,
 PRIMARY KEY(application_id,kind)
);
INSERT INTO capstone_service.proposal_documents
SELECT id,'simak',simak_file_id,simak_status,simak_note,simak_reviewer,simak_reviewed_at FROM capstone_service.proposal_applications
UNION ALL SELECT id,'approval',approval_file_id,approval_status,approval_note,approval_reviewer,approval_reviewed_at FROM capstone_service.proposal_applications;
CREATE INDEX proposal_documents_file_idx ON capstone_service.proposal_documents(file_id);

CREATE TABLE capstone_service.proposal_grading_policies (
 id uuid PRIMARY KEY, name text NOT NULL, rules jsonb NOT NULL, created_by uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE capstone_service.lecturer_signatures (
 lecturer_id uuid PRIMARY KEY, storage_key text NOT NULL, version integer NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE capstone_service.proposal_exams (
 id uuid PRIMARY KEY, application_id uuid NOT NULL UNIQUE REFERENCES capstone_service.proposal_applications(id),
 scheduled_at timestamptz NOT NULL, location text NOT NULL,
 panel jsonb NOT NULL CHECK(jsonb_array_length(panel)=5),
 policy_id uuid REFERENCES capstone_service.proposal_grading_policies(id),
 version integer NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE capstone_service.proposal_exam_evaluations (
 exam_id uuid NOT NULL REFERENCES capstone_service.proposal_exams(id), evaluator_id uuid NOT NULL,
 scores jsonb NOT NULL, total numeric(7,4) NOT NULL CHECK(total BETWEEN 0 AND 100),
 recommendation text NOT NULL CHECK(recommendation IN ('APPROVED','REVISION','REJECTED')),
 notes text NOT NULL DEFAULT '', signature_key text NOT NULL, version integer NOT NULL DEFAULT 1,
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(exam_id,evaluator_id)
);
CREATE TABLE capstone_service.proposal_exam_attendance (
 exam_id uuid NOT NULL REFERENCES capstone_service.proposal_exams(id), lecturer_id uuid NOT NULL,
 signature_key text NOT NULL, confirmed_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(exam_id,lecturer_id)
);
