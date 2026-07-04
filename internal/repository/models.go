package repository

import "time"

type User struct {
	ID           int64     `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Name         string    `json:"name" db:"name"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Team struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Role      string    `json:"role,omitempty" db:"role"`
	CreatedBy int64     `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type TeamMember struct {
	UserID   int64     `json:"user_id" db:"user_id"`
	TeamID   int64     `json:"team_id" db:"team_id"`
	Email    string    `json:"email" db:"email"`
	Name     string    `json:"name" db:"name"`
	Role     string    `json:"role" db:"role"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

type Worker struct {
	ID        int64     `json:"id" db:"id"`
	TeamID    int64     `json:"team_id" db:"team_id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Skill     string    `json:"skill,omitempty" db:"skill"`
	Skills    []string  `json:"skills" db:"skills"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Task struct {
	ID           int64      `json:"id" db:"id"`
	TeamID       int64      `json:"team_id" db:"team_id"`
	Title        string     `json:"title" db:"title"`
	Description  string     `json:"description" db:"description"`
	Status       string     `json:"status" db:"status"`
	AssigneeID   *int64     `json:"assignee_id,omitempty" db:"assignee_id"`
	AssigneeName *string    `json:"assignee_name,omitempty" db:"assignee_name"`
	Skill        string     `json:"skill,omitempty" db:"skill"`
	Skills       []string   `json:"skills" db:"skills"`
	CreatedBy    int64      `json:"created_by" db:"created_by"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	DoneAt       *time.Time `json:"done_at,omitempty" db:"done_at"`
}

type TaskComment struct {
	ID        int64     `json:"id" db:"id"`
	TaskID    int64     `json:"task_id" db:"task_id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	UserName  string    `json:"user_name" db:"user_name"`
	Body      string    `json:"body" db:"body"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type TaskHistory struct {
	ID        int64     `json:"id" db:"id"`
	TaskID    int64     `json:"task_id" db:"task_id"`
	ChangedBy int64     `json:"changed_by" db:"changed_by"`
	Field     string    `json:"field" db:"field"`
	OldValue  *string   `json:"old_value,omitempty" db:"old_value"`
	NewValue  *string   `json:"new_value,omitempty" db:"new_value"`
	ChangedAt time.Time `json:"changed_at" db:"changed_at"`
}

type TeamSummary struct {
	TeamID        int64  `json:"team_id" db:"team_id"`
	TeamName      string `json:"team_name" db:"team_name"`
	MembersCount  int64  `json:"members_count" db:"members_count"`
	DoneLast7Days int64  `json:"done_last_7_days" db:"done_last_7_days"`
}

type TopCreator struct {
	TeamID       int64  `json:"team_id" db:"team_id"`
	TeamName     string `json:"team_name" db:"team_name"`
	UserID       int64  `json:"user_id" db:"user_id"`
	UserName     string `json:"user_name" db:"user_name"`
	CreatedTasks int64  `json:"created_tasks" db:"created_tasks"`
	Rank         int64  `json:"rank" db:"rank"`
}

type InvalidAssignee struct {
	TaskID        int64  `json:"task_id" db:"task_id"`
	TaskTitle     string `json:"task_title" db:"task_title"`
	TeamID        int64  `json:"team_id" db:"team_id"`
	TeamName      string `json:"team_name" db:"team_name"`
	AssigneeID    int64  `json:"assignee_id" db:"assignee_id"`
	AssigneeEmail string `json:"assignee_email" db:"assignee_email"`
}
