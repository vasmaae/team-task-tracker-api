ALTER TABLE teams ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

DELETE FROM teams t
WHERE EXISTS (
    SELECT 1
    FROM teams keep
    WHERE keep.created_by = t.created_by
      AND keep.id < t.id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_teams_one_per_leader ON teams(created_by);

CREATE TABLE IF NOT EXISTS workers (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    email CITEXT NOT NULL,
    skill TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(team_id, email)
);

CREATE INDEX IF NOT EXISTS idx_workers_team_skill ON workers(team_id, skill);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS skill TEXT NOT NULL DEFAULT 'general';

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check
CHECK (status IN ('backlog', 'todo', 'in_progress', 'review', 'ready_for_testing', 'testing', 'done'));

UPDATE tasks SET status = 'todo' WHERE status NOT IN ('backlog', 'todo', 'in_progress', 'review', 'ready_for_testing', 'testing', 'done');

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_assignee_id_fkey;
UPDATE tasks SET assignee_id = NULL
WHERE assignee_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM workers WHERE workers.id = tasks.assignee_id);
ALTER TABLE tasks ADD CONSTRAINT tasks_assignee_id_fkey
FOREIGN KEY (assignee_id) REFERENCES workers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_team_skill_status ON tasks(team_id, skill, status, created_at DESC);

CREATE TABLE IF NOT EXISTS email_verification_codes (
    email CITEXT PRIMARY KEY,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
