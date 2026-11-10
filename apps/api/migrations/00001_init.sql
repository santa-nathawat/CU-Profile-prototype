-- +goose Up
CREATE TYPE project_status AS ENUM ('not_started', 'in_progress', 'completed', 'cancelled');
CREATE TYPE recruitment_status AS ENUM ('open', 'closed');
CREATE TYPE duration_unit AS ENUM ('day', 'week', 'month');
CREATE TYPE member_left_reason AS ENUM ('left', 'removed', 'team_deleted', 'account_closed');
CREATE TYPE request_type AS ENUM ('application', 'invitation');
CREATE TYPE request_status AS ENUM ('pending', 'approved', 'rejected', 'accepted', 'declined', 'withdrawn', 'cancelled', 'expired');
CREATE TYPE request_cancel_reason AS ENUM ('lead_cancelled', 'role_removed', 'role_full', 'recruitment_closed', 'project_completed', 'project_cancelled', 'team_deleted', 'account_closed');
CREATE TYPE transfer_status AS ENUM ('pending', 'accepted', 'declined', 'cancelled');

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE CHECK (email = lower(email) AND email ~ '^[^@[:space:]]+@student\.chula\.ac\.th$'),
  oauth_subject text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  last_signed_in_at timestamptz,
  closed_at timestamptz
);

CREATE TABLE sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id),
  token_hash text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz
);
CREATE INDEX sessions_active_user ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE faculties (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE profiles (
  user_id uuid PRIMARY KEY REFERENCES users (id),
  display_name text,
  faculty_id uuid REFERENCES faculties (id),
  year smallint CHECK (year BETWEEN 1 AND 8),
  headline text,
  working_style text,
  cv_storage_key text,
  cv_file_name text,
  cv_size_bytes integer CHECK (cv_size_bytes BETWEEN 1 AND 5242880),
  cv_uploaded_at timestamptz,
  cv_shared boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (num_nulls(cv_storage_key, cv_file_name, cv_size_bytes, cv_uploaded_at) IN (0, 4))
);

CREATE TABLE skills (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  normalized_name text GENERATED ALWAYS AS (lower(trim(name))) STORED UNIQUE,
  is_predefined boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_skills (
  user_id uuid NOT NULL REFERENCES users (id),
  skill_id uuid NOT NULL REFERENCES skills (id),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, skill_id)
);
CREATE INDEX user_skills_skill ON user_skills (skill_id);

CREATE TABLE availability_slots (
  user_id uuid NOT NULL REFERENCES users (id),
  day_of_week smallint NOT NULL CHECK (day_of_week BETWEEN 1 AND 7),
  start_time time NOT NULL CHECK (extract(minute FROM start_time) IN (0, 30) AND extract(second FROM start_time) = 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, day_of_week, start_time)
);

CREATE TABLE project_categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  description text NOT NULL,
  category_id uuid NOT NULL REFERENCES project_categories (id),
  lead_user_id uuid NOT NULL REFERENCES users (id),
  status project_status NOT NULL DEFAULT 'not_started',
  recruitment_status recruitment_status NOT NULL DEFAULT 'open',
  expected_duration_value integer NOT NULL CHECK (expected_duration_value > 0),
  expected_duration_unit duration_unit NOT NULL,
  expected_end_date date,
  line_url text CHECK (line_url LIKE 'https://%'),
  discord_url text CHECK (discord_url LIKE 'https://%'),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz,
  cancelled_at timestamptz,
  deleted_at timestamptz,
  end_reminder_sent_at timestamptz,
  CHECK ((status = 'not_started') = (started_at IS NULL)),
  CHECK ((status = 'completed') = (completed_at IS NOT NULL)),
  CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL)),
  CHECK (status = 'not_started' OR expected_end_date IS NOT NULL),
  CHECK (status NOT IN ('completed', 'cancelled') OR recruitment_status = 'closed'),
  CHECK (deleted_at IS NULL OR (status = 'not_started' AND recruitment_status = 'closed'))
);
CREATE INDEX projects_browse ON projects (category_id) WHERE recruitment_status = 'open' AND deleted_at IS NULL;
CREATE INDEX projects_lead ON projects (lead_user_id);

CREATE TABLE project_roles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects (id),
  name text NOT NULL,
  capacity integer NOT NULL CHECK (capacity >= 1),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  removed_at timestamptz,
  UNIQUE (id, project_id)
);
CREATE UNIQUE INDEX project_roles_unique_name ON project_roles (project_id, lower(name)) WHERE removed_at IS NULL;

CREATE TABLE project_members (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL,
  role_id uuid NOT NULL,
  user_id uuid NOT NULL REFERENCES users (id),
  joined_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  left_at timestamptz,
  left_reason member_left_reason,
  FOREIGN KEY (role_id, project_id) REFERENCES project_roles (id, project_id),
  CHECK ((left_at IS NULL) = (left_reason IS NULL))
);
CREATE UNIQUE INDEX project_members_one_active ON project_members (project_id, user_id) WHERE left_at IS NULL;
CREATE INDEX project_members_role_active ON project_members (role_id) WHERE left_at IS NULL;
CREATE INDEX project_members_user ON project_members (user_id);

CREATE TABLE join_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type request_type NOT NULL,
  project_id uuid NOT NULL,
  role_id uuid NOT NULL,
  student_id uuid NOT NULL REFERENCES users (id),
  invited_by_id uuid REFERENCES users (id),
  message text,
  status request_status NOT NULL DEFAULT 'pending',
  cancel_reason request_cancel_reason,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL DEFAULT now() + interval '48 hours',
  resolved_at timestamptz,
  FOREIGN KEY (role_id, project_id) REFERENCES project_roles (id, project_id),
  CHECK ((type = 'invitation') = (invited_by_id IS NOT NULL)),
  CHECK (type = 'application' OR message IS NULL),
  CHECK (
    (type = 'application' AND status IN ('pending', 'approved', 'rejected', 'withdrawn', 'cancelled', 'expired'))
    OR (type = 'invitation' AND status IN ('pending', 'accepted', 'declined', 'cancelled', 'expired'))
  ),
  CHECK ((status = 'cancelled') = (cancel_reason IS NOT NULL)),
  CHECK ((status = 'pending') = (resolved_at IS NULL))
);
CREATE UNIQUE INDEX join_requests_one_pending ON join_requests (project_id, student_id) WHERE status = 'pending';
CREATE INDEX join_requests_history ON join_requests (project_id, student_id, created_at DESC);
CREATE INDEX join_requests_student ON join_requests (student_id, created_at DESC);
CREATE INDEX join_requests_expiry ON join_requests (expires_at) WHERE status = 'pending';

CREATE TABLE leadership_transfers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects (id),
  from_user_id uuid NOT NULL REFERENCES users (id),
  to_user_id uuid NOT NULL REFERENCES users (id),
  status transfer_status NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  CHECK (from_user_id <> to_user_id),
  CHECK ((status = 'pending') = (resolved_at IS NULL))
);
CREATE UNIQUE INDEX leadership_transfers_one_pending ON leadership_transfers (project_id) WHERE status = 'pending';

CREATE TABLE ratings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects (id),
  reviewer_id uuid NOT NULL REFERENCES users (id),
  recipient_id uuid NOT NULL REFERENCES users (id),
  score smallint NOT NULL CHECK (score BETWEEN 1 AND 5),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, reviewer_id, recipient_id),
  CHECK (reviewer_id <> recipient_id)
);
CREATE INDEX ratings_recipient ON ratings (recipient_id);

CREATE TABLE notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id),
  type text NOT NULL,
  project_id uuid REFERENCES projects (id),
  join_request_id uuid REFERENCES join_requests (id),
  data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  read_at timestamptz
);
CREATE INDEX notifications_inbox ON notifications (user_id, created_at DESC);
CREATE INDEX notifications_unread ON notifications (user_id) WHERE read_at IS NULL;

-- PDFs live in Postgres because hosting has no persistent disk. profiles.cv_storage_key holds db:<user_id>.
CREATE TABLE cv_files (
  user_id uuid PRIMARY KEY REFERENCES users (id),
  content bytea NOT NULL
);

-- +goose Down
DROP TABLE cv_files;
DROP TABLE notifications;
DROP TABLE ratings;
DROP TABLE leadership_transfers;
DROP TABLE join_requests;
DROP TABLE project_members;
DROP TABLE project_roles;
DROP TABLE projects;
DROP TABLE project_categories;
DROP TABLE availability_slots;
DROP TABLE user_skills;
DROP TABLE skills;
DROP TABLE profiles;
DROP TABLE faculties;
DROP TABLE sessions;
DROP TABLE users;
DROP TYPE transfer_status;
DROP TYPE request_cancel_reason;
DROP TYPE request_status;
DROP TYPE request_type;
DROP TYPE member_left_reason;
DROP TYPE duration_unit;
DROP TYPE recruitment_status;
DROP TYPE project_status;
