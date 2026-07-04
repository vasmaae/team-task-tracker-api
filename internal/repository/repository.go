package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, name string) (User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users(email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, name, created_at`,
		email, passwordHash, name,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	return u, err
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, name, created_at
		FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (r *Repository) SaveEmailCode(ctx context.Context, email, code, name, passwordHash string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO email_verification_codes(email, code, name, password_hash, expires_at)
		VALUES ($1, $2, $3, $4, now() + interval '10 minutes')
		ON CONFLICT (email) DO UPDATE
		SET code = EXCLUDED.code,
		    name = EXCLUDED.name,
		    password_hash = EXCLUDED.password_hash,
		    expires_at = EXCLUDED.expires_at,
		    created_at = now()`,
		email, code, name, passwordHash)
	return err
}

func (r *Repository) CreateUserFromEmailCode(ctx context.Context, email, code string) (User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var name, passwordHash string
	err = tx.QueryRow(ctx, `
		SELECT name, password_hash
		FROM email_verification_codes
		WHERE email = $1 AND code = $2 AND expires_at > now()
		FOR UPDATE`, email, code,
	).Scan(&name, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users(email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, name, created_at`,
		email, passwordHash, name,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	if err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM email_verification_codes WHERE email = $1`, email); err != nil {
		return User{}, err
	}
	return u, tx.Commit(ctx)
}

func (r *Repository) CreateTeam(ctx context.Context, name string, createdBy int64) (Team, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Team{}, err
	}
	defer tx.Rollback(ctx)

	var team Team
	err = tx.QueryRow(ctx, `
		INSERT INTO teams(name, created_by)
		VALUES ($1, $2)
		ON CONFLICT (created_by) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
		RETURNING id, name, created_by, created_at`, name, createdBy,
	).Scan(&team.ID, &team.Name, &team.CreatedBy, &team.CreatedAt)
	if err != nil {
		return Team{}, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO team_members(user_id, team_id, role)
		VALUES ($1, $2, 'owner')
		ON CONFLICT (user_id, team_id) DO UPDATE SET role = 'owner'`, createdBy, team.ID,
	); err != nil {
		return Team{}, err
	}
	team.Role = "owner"
	return team, tx.Commit(ctx)
}

func (r *Repository) ListTeamsForUser(ctx context.Context, userID int64) ([]Team, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.name, tm.role, t.created_by, t.created_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1
		ORDER BY t.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[Team])
}

func (r *Repository) TeamRole(ctx context.Context, teamID, userID int64) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, `
		SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

func (r *Repository) AddTeamMember(ctx context.Context, teamID, userID int64, role string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO team_members(user_id, team_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, team_id) DO UPDATE SET role = EXCLUDED.role`,
		userID, teamID, role,
	)
	return err
}

func (r *Repository) ListTeamMembers(ctx context.Context, teamID int64) ([]TeamMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tm.user_id, tm.team_id, u.email, u.name, tm.role, tm.joined_at
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1
		ORDER BY CASE tm.role WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 ELSE 3 END, u.name`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[TeamMember])
}

func (r *Repository) CreateWorker(ctx context.Context, worker Worker) (Worker, error) {
	var created Worker
	err := r.db.QueryRow(ctx, `
		INSERT INTO workers(team_id, name, email, skill, skills)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, team_id, name, email, skill, skills, created_at`,
		worker.TeamID, worker.Name, worker.Email, firstSkill(worker.Skills), worker.Skills,
	).Scan(&created.ID, &created.TeamID, &created.Name, &created.Email, &created.Skill, &created.Skills, &created.CreatedAt)
	return created, err
}

func (r *Repository) ListWorkers(ctx context.Context, teamID int64) ([]Worker, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, team_id, name, email, skill, skills, created_at
		FROM workers
		WHERE team_id = $1
		ORDER BY skill, name`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[Worker])
}

func (r *Repository) UpdateWorker(ctx context.Context, worker Worker) (Worker, error) {
	var updated Worker
	err := r.db.QueryRow(ctx, `
		UPDATE workers
		SET name = $3, email = $4, skill = $5, skills = $6
		WHERE id = $1 AND team_id = $2
		RETURNING id, team_id, name, email, skill, skills, created_at`,
		worker.ID, worker.TeamID, worker.Name, worker.Email, firstSkill(worker.Skills), worker.Skills,
	).Scan(&updated.ID, &updated.TeamID, &updated.Name, &updated.Email, &updated.Skill, &updated.Skills, &updated.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return updated, ErrNotFound
	}
	return updated, err
}

func (r *Repository) DeleteWorker(ctx context.Context, teamID, workerID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM workers WHERE id = $1 AND team_id = $2`, workerID, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CreateTask(ctx context.Context, task Task) (Task, error) {
	var created Task
	err := r.db.QueryRow(ctx, `
		INSERT INTO tasks(team_id, title, description, status, assignee_id, skill, skills, created_by, done_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CASE WHEN $4 = 'done' THEN now() ELSE NULL END)
		RETURNING id, team_id, title, description, status, assignee_id, NULL::text AS assignee_name, skill, skills, created_by, created_at, updated_at, done_at`,
		task.TeamID, task.Title, task.Description, task.Status, task.AssigneeID, firstSkill(task.Skills), task.Skills, task.CreatedBy,
	).Scan(&created.ID, &created.TeamID, &created.Title, &created.Description, &created.Status, &created.AssigneeID, &created.AssigneeName, &created.Skill, &created.Skills, &created.CreatedBy, &created.CreatedAt, &created.UpdatedAt, &created.DoneAt)
	return created, err
}

type TaskFilter struct {
	TeamID     int64
	Status     string
	AssigneeID *int64
	Limit      int
	Offset     int
}

func (r *Repository) ListTasks(ctx context.Context, f TaskFilter) ([]Task, error) {
	args := []any{f.TeamID}
	where := []string{"task.team_id = $1"}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("task.status = $%d", len(args)))
	}
	if f.AssigneeID != nil {
		args = append(args, *f.AssigneeID)
		where = append(where, fmt.Sprintf("task.assignee_id = $%d", len(args)))
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	args = append(args, limit, max(f.Offset, 0))
	query := fmt.Sprintf(`
		SELECT task.id, task.team_id, task.title, task.description, task.status, task.assignee_id, w.name AS assignee_name, task.skill, task.skills, task.created_by, task.created_at, task.updated_at, task.done_at
		FROM tasks task
		LEFT JOIN workers w ON w.id = task.assignee_id
		WHERE %s
		ORDER BY task.created_at DESC
		LIMIT $%d OFFSET $%d`, strings.Join(where, " AND "), len(args)-1, len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[Task])
}

func (r *Repository) AddTaskComment(ctx context.Context, taskID, userID int64, body string) (TaskComment, error) {
	var comment TaskComment
	err := r.db.QueryRow(ctx, `
		INSERT INTO task_comments(task_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, task_id, user_id, (SELECT name FROM users WHERE id = $2), body, created_at`,
		taskID, userID, body,
	).Scan(&comment.ID, &comment.TaskID, &comment.UserID, &comment.UserName, &comment.Body, &comment.CreatedAt)
	return comment, err
}

func (r *Repository) ListTaskComments(ctx context.Context, taskID int64) ([]TaskComment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.task_id, c.user_id, u.name AS user_name, c.body, c.created_at
		FROM task_comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.task_id = $1
		ORDER BY c.created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[TaskComment])
}

func (r *Repository) GetTask(ctx context.Context, taskID int64) (Task, error) {
	var t Task
	err := r.db.QueryRow(ctx, `
		SELECT task.id, task.team_id, task.title, task.description, task.status, task.assignee_id, w.name AS assignee_name, task.skill, task.skills, task.created_by, task.created_at, task.updated_at, task.done_at
		FROM tasks task
		LEFT JOIN workers w ON w.id = task.assignee_id
		WHERE task.id = $1`, taskID,
	).Scan(&t.ID, &t.TeamID, &t.Title, &t.Description, &t.Status, &t.AssigneeID, &t.AssigneeName, &t.Skill, &t.Skills, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.DoneAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (r *Repository) UpdateTask(ctx context.Context, actorID, taskID int64, title, description, status string, skills []string, assigneeID *int64) (Task, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback(ctx)

	var old Task
	err = tx.QueryRow(ctx, `
		SELECT id, team_id, title, description, status, assignee_id, NULL::text AS assignee_name, skill, skills, created_by, created_at, updated_at, done_at
		FROM tasks WHERE id = $1 FOR UPDATE`, taskID,
	).Scan(&old.ID, &old.TeamID, &old.Title, &old.Description, &old.Status, &old.AssigneeID, &old.AssigneeName, &old.Skill, &old.Skills, &old.CreatedBy, &old.CreatedAt, &old.UpdatedAt, &old.DoneAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}

	var updated Task
	err = tx.QueryRow(ctx, `
		UPDATE tasks
		SET title = COALESCE(NULLIF($2, ''), title),
		    description = COALESCE($3, description),
		    status = COALESCE(NULLIF($4, ''), status),
		    assignee_id = $5,
		    skill = $6,
		    skills = $7,
		    updated_at = now(),
		    done_at = CASE
		        WHEN COALESCE(NULLIF($4, ''), status) = 'done' AND status <> 'done' THEN now()
		        WHEN COALESCE(NULLIF($4, ''), status) <> 'done' THEN NULL
		        ELSE done_at
		    END
		WHERE id = $1
		RETURNING id, team_id, title, description, status, assignee_id, NULL::text AS assignee_name, skill, skills, created_by, created_at, updated_at, done_at`,
		taskID, title, description, status, assigneeID, firstSkill(skills), skills,
	).Scan(&updated.ID, &updated.TeamID, &updated.Title, &updated.Description, &updated.Status, &updated.AssigneeID, &updated.AssigneeName, &updated.Skill, &updated.Skills, &updated.CreatedBy, &updated.CreatedAt, &updated.UpdatedAt, &updated.DoneAt)
	if err != nil {
		return Task{}, err
	}

	changes := map[string][2]*string{
		"title":       {strPtr(old.Title), strPtr(updated.Title)},
		"description": {strPtr(old.Description), strPtr(updated.Description)},
		"status":      {strPtr(old.Status), strPtr(updated.Status)},
		"assignee_id": {intPtrString(old.AssigneeID), intPtrString(updated.AssigneeID)},
		"skills":      {strPtr(strings.Join(old.Skills, ",")), strPtr(strings.Join(updated.Skills, ","))},
	}
	for field, pair := range changes {
		if value(pair[0]) == value(pair[1]) {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO task_history(task_id, changed_by, field, old_value, new_value)
			VALUES ($1, $2, $3, $4, $5)`, taskID, actorID, field, pair[0], pair[1],
		); err != nil {
			return Task{}, err
		}
	}
	return updated, tx.Commit(ctx)
}

func (r *Repository) DeleteTask(ctx context.Context, teamID, taskID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id = $1 AND team_id = $2`, taskID, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) AutoAssignTasks(ctx context.Context, teamID int64) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	type workerForAssign struct {
		id     int64
		skills []string
		load   int
	}
	workerRows, err := tx.Query(ctx, `
		SELECT w.id, w.skills, COUNT(task.id) AS load
		FROM workers
		w
		LEFT JOIN tasks task ON task.assignee_id = w.id
		WHERE w.team_id = $1
		GROUP BY w.id, w.skills
		ORDER BY w.id`, teamID)
	if err != nil {
		return 0, err
	}
	var workers []workerForAssign
	for workerRows.Next() {
		var worker workerForAssign
		if err := workerRows.Scan(&worker.id, &worker.skills, &worker.load); err != nil {
			workerRows.Close()
			return 0, err
		}
		workers = append(workers, worker)
	}
	workerRows.Close()

	taskRows, err := tx.Query(ctx, `
		SELECT id, skills
		FROM tasks
		WHERE team_id = $1 AND assignee_id IS NULL
		ORDER BY cardinality(skills) DESC, created_at ASC`, teamID)
	if err != nil {
		return 0, err
	}
	type taskForAssign struct {
		id     int64
		skills []string
	}
	var tasks []taskForAssign
	for taskRows.Next() {
		var t taskForAssign
		if err := taskRows.Scan(&t.id, &t.skills); err != nil {
			taskRows.Close()
			return 0, err
		}
		tasks = append(tasks, t)
	}
	taskRows.Close()
	sort.SliceStable(tasks, func(i, j int) bool {
		return len(tasks[i].skills) > len(tasks[j].skills)
	})

	assigned := 0
	for _, task := range tasks {
		best := -1
		for i := range workers {
			if !coversAll(workers[i].skills, task.skills) {
				continue
			}
			if best == -1 ||
				workers[i].load < workers[best].load ||
				(workers[i].load == workers[best].load && len(workers[i].skills) > len(workers[best].skills)) {
				best = i
			}
		}
		if best == -1 {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE tasks SET assignee_id = $2, updated_at = now() WHERE id = $1`, task.id, workers[best].id); err != nil {
			return 0, err
		}
		workers[best].load++
		assigned++
	}
	return assigned, tx.Commit(ctx)
}

func (r *Repository) TaskHistory(ctx context.Context, taskID int64) ([]TaskHistory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, changed_by, field, old_value, new_value, changed_at
		FROM task_history
		WHERE task_id = $1
		ORDER BY changed_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[TaskHistory])
}

func (r *Repository) TeamSummary(ctx context.Context) ([]TeamSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id AS team_id,
		       t.name AS team_name,
		       COUNT(DISTINCT w.id) AS members_count,
		       COUNT(DISTINCT task.id) FILTER (
		           WHERE task.status = 'done' AND task.done_at >= now() - interval '7 days'
		       ) AS done_last_7_days
		FROM teams t
		LEFT JOIN workers w ON w.team_id = t.id
		LEFT JOIN tasks task ON task.team_id = t.id
		GROUP BY t.id, t.name
		ORDER BY t.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[TeamSummary])
}

func (r *Repository) TopCreators(ctx context.Context) ([]TopCreator, error) {
	rows, err := r.db.Query(ctx, `
		WITH ranked AS (
		    SELECT t.id AS team_id,
		           t.name AS team_name,
		           u.id AS user_id,
		           u.name AS user_name,
		           COUNT(task.id) AS created_tasks,
		           DENSE_RANK() OVER (PARTITION BY t.id ORDER BY COUNT(task.id) DESC) AS rank
		    FROM teams t
		    JOIN tasks task ON task.team_id = t.id
		    JOIN users u ON u.id = task.created_by
		    WHERE task.created_at >= date_trunc('month', now())
		    GROUP BY t.id, t.name, u.id, u.name
		)
		SELECT team_id, team_name, user_id, user_name, created_tasks, rank
		FROM ranked
		WHERE rank <= 3
		ORDER BY team_name, rank, user_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[TopCreator])
}

func (r *Repository) InvalidAssignees(ctx context.Context) ([]InvalidAssignee, error) {
	rows, err := r.db.Query(ctx, `
		SELECT task.id AS task_id,
		       task.title AS task_title,
		       t.id AS team_id,
		       t.name AS team_name,
		       w.id AS assignee_id,
		       w.email AS assignee_email
		FROM tasks task
		JOIN teams t ON t.id = task.team_id
		JOIN workers w ON w.id = task.assignee_id
		WHERE task.assignee_id IS NOT NULL
		  AND NOT EXISTS (
		      SELECT 1
		      FROM workers worker
		      WHERE worker.team_id = task.team_id AND worker.id = task.assignee_id
		  )
		ORDER BY t.name, task.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[InvalidAssignee])
}

func strPtr(v string) *string { return &v }

func intPtrString(v *int64) *string {
	if v == nil {
		return nil
	}
	s := fmt.Sprint(*v)
	return &s
}

func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func firstSkill(skills []string) string {
	if len(skills) == 0 {
		return "general"
	}
	return skills[0]
}

func coversAll(workerSkills, taskSkills []string) bool {
	if len(taskSkills) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(workerSkills))
	for _, skill := range workerSkills {
		set[strings.ToLower(strings.TrimSpace(skill))] = struct{}{}
	}
	for _, skill := range taskSkills {
		if _, ok := set[strings.ToLower(strings.TrimSpace(skill))]; !ok {
			return false
		}
	}
	return true
}
