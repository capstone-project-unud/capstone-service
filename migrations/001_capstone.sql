CREATE TABLE capstone_service.interest_fields (
 id uuid PRIMARY KEY, name varchar(150) UNIQUE NOT NULL, is_active boolean NOT NULL DEFAULT true
);
CREATE TABLE capstone_service.files (
 id uuid PRIMARY KEY, owner_id uuid NOT NULL, name varchar(200) NOT NULL,
 mime varchar(100) NOT NULL, size bigint NOT NULL CHECK(size BETWEEN 1 AND 10485760),
 storage_key text UNIQUE NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE capstone_service.projects (
 id uuid PRIMARY KEY, owner_id uuid NOT NULL, title varchar(500) NOT NULL,
 summary text NOT NULL DEFAULT '', interest_id uuid NOT NULL REFERENCES capstone_service.interest_fields(id),
 description_file_id uuid REFERENCES capstone_service.files(id), signed_description boolean NOT NULL DEFAULT false,
 status text NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','SUBMITTED','REVISION','FINALIZED')),
 revision_note text NOT NULL DEFAULT '', version integer NOT NULL DEFAULT 1,
 finalized_by uuid, finalized_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
-- A single detail table ties each student to each of their two supervisor positions.
CREATE TABLE capstone_service.project_students (
 project_id uuid NOT NULL REFERENCES capstone_service.projects(id), student_id uuid NOT NULL,
 student_name varchar(150) NOT NULL, nim varchar(30) NOT NULL,
 slot smallint NOT NULL CHECK(slot IN (1,2)), candidate_id uuid, candidate_name varchar(150) NOT NULL DEFAULT '',
 supervisor_id uuid, supervisor_name varchar(150) NOT NULL DEFAULT '',
 title_status text NOT NULL DEFAULT 'PENDING' CHECK(title_status IN ('PENDING','APPROVED','REJECTED')),
 confirmation_status text NOT NULL DEFAULT 'PENDING' CHECK(confirmation_status IN ('PENDING','APPROVED','REJECTED')),
 title_actor uuid, title_at timestamptz, confirmation_actor uuid, confirmation_at timestamptz,
 note text NOT NULL DEFAULT '', PRIMARY KEY(project_id,student_id,slot), UNIQUE(student_id,slot)
);
CREATE INDEX project_students_candidate ON capstone_service.project_students(candidate_id);
CREATE INDEX project_students_supervisor ON capstone_service.project_students(supervisor_id);
CREATE TABLE capstone_service.guidance_logs (
 id uuid PRIMARY KEY, project_id uuid NOT NULL REFERENCES capstone_service.projects(id), student_id uuid NOT NULL,
 supervisor_id uuid NOT NULL, supervisor_name varchar(150) NOT NULL, student_name varchar(150) NOT NULL,
 meeting_date date NOT NULL, topic varchar(300) NOT NULL, progress text NOT NULL, obstacles text NOT NULL DEFAULT '', next_plan text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','APPROVED','REVISION')),
 review_note text NOT NULL DEFAULT '', reviewed_at timestamptz, version integer NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE capstone_service.proposal_applications (
 id uuid PRIMARY KEY, project_id uuid NOT NULL REFERENCES capstone_service.projects(id), student_id uuid UNIQUE NOT NULL,
 simak_file_id uuid NOT NULL REFERENCES capstone_service.files(id), approval_file_id uuid NOT NULL REFERENCES capstone_service.files(id),
 simak_status text NOT NULL DEFAULT 'PENDING' CHECK(simak_status IN ('PENDING','VALID','REVISION')),
 approval_status text NOT NULL DEFAULT 'PENDING' CHECK(approval_status IN ('PENDING','VALID','REVISION')),
 simak_note text NOT NULL DEFAULT '', approval_note text NOT NULL DEFAULT '',
 simak_reviewer uuid, approval_reviewer uuid, simak_reviewed_at timestamptz, approval_reviewed_at timestamptz,
 version integer NOT NULL DEFAULT 1, submitted_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE capstone_service.audit_logs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, actor_id uuid NOT NULL,
 project_id uuid REFERENCES capstone_service.projects(id), action text NOT NULL, detail jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now()
);
