ALTER TABLE workers ADD COLUMN IF NOT EXISTS skills TEXT[] NOT NULL DEFAULT ARRAY['general']::TEXT[];
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS skills TEXT[] NOT NULL DEFAULT ARRAY['general']::TEXT[];

UPDATE workers
SET skills = ARRAY[skill]
WHERE (skills = ARRAY['general']::TEXT[] OR skills IS NULL) AND skill IS NOT NULL AND skill <> '';

UPDATE tasks
SET skills = ARRAY[skill]
WHERE (skills = ARRAY['general']::TEXT[] OR skills IS NULL) AND skill IS NOT NULL AND skill <> '';

CREATE INDEX IF NOT EXISTS idx_workers_team_skills ON workers USING GIN (skills);
CREATE INDEX IF NOT EXISTS idx_tasks_team_skills ON tasks USING GIN (skills);
