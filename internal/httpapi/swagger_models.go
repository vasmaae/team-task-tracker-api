package httpapi

type errorResponse struct {
	Error string `json:"error" example:"invalid credentials"`
}

type authRequest struct {
	Email    string `json:"email" example:"leader@example.com"`
	Password string `json:"password" example:"secret123"`
}

type registerRequest struct {
	Email    string `json:"email" example:"leader@example.com"`
	Password string `json:"password" example:"secret123"`
	Name     string `json:"name" example:"Alex Leader"`
}

type verifyRegisterCodeRequest struct {
	Email string `json:"email" example:"leader@example.com"`
	Code  string `json:"code" example:"123456"`
}

type authResponse struct {
	User  userResponse `json:"user"`
	Token string       `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

type userResponse struct {
	ID        int64  `json:"id" example:"1"`
	Email     string `json:"email" example:"leader@example.com"`
	Name      string `json:"name" example:"Alex Leader"`
	CreatedAt string `json:"created_at" example:"2026-07-05T10:00:00Z"`
}

type messageResponse struct {
	Message string `json:"message" example:"verification code sent"`
}

type statusResponse struct {
	Status string `json:"status" example:"deleted"`
}

type createTeamRequest struct {
	Name string `json:"name" example:"Platform Team"`
}

type workerRequest struct {
	Name   string   `json:"name" example:"Ivan Petrov"`
	Email  string   `json:"email" example:"ivan@example.com"`
	Skill  string   `json:"skill,omitempty" example:"go"`
	Skills []string `json:"skills" example:"go,postgres,redis"`
}

type taskRequest struct {
	TeamID      int64    `json:"team_id" example:"1"`
	Title       string   `json:"title" example:"Build task API"`
	Description string   `json:"description" example:"Implement task CRUD and filters"`
	Status      string   `json:"status" enums:"backlog,todo,in_progress,review,ready_for_testing,testing,done" example:"todo"`
	AssigneeID  *int64   `json:"assignee_id,omitempty" example:"2"`
	Skill       string   `json:"skill,omitempty" example:"go"`
	Skills      []string `json:"skills" example:"go,postgres"`
}

type updateTaskRequest struct {
	Title       string   `json:"title" example:"Build task API"`
	Description string   `json:"description" example:"Implement task CRUD and filters"`
	Status      string   `json:"status" enums:"backlog,todo,in_progress,review,ready_for_testing,testing,done" example:"in_progress"`
	AssigneeID  *int64   `json:"assignee_id,omitempty" example:"2"`
	Skill       string   `json:"skill,omitempty" example:"go"`
	Skills      []string `json:"skills" example:"go,postgres"`
}

type commentRequest struct {
	Body string `json:"body" example:"Need to update tests after this change."`
}

type autoAssignResponse struct {
	Assigned int `json:"assigned" example:"3"`
}
