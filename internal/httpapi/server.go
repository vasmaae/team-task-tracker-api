package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"team-task-tracker-api/internal/auth"
	"team-task-tracker-api/internal/ratelimit"
	"team-task-tracker-api/internal/repository"
	"team-task-tracker-api/internal/service"

	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Server struct {
	mux     *http.ServeMux
	repo    *repository.Repository
	redis   *redis.Client
	email   *service.EmailService
	jwtKey  string
	jwtTTL  time.Duration
	limiter *ratelimit.Limiter
}

func New(repo *repository.Repository, redisClient *redis.Client, email *service.EmailService, jwtKey string, jwtTTL time.Duration, limiter *ratelimit.Limiter) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		repo:    repo,
		redis:   redisClient,
		email:   email,
		jwtKey:  jwtKey,
		jwtTTL:  jwtTTL,
		limiter: limiter,
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return cors(s.rateLimit(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusFound)
	})
	s.mux.Handle("/swagger/", httpSwagger.WrapHandler)
	s.mux.HandleFunc("POST /api/v1/register/request-code", s.requestRegisterCode)
	s.mux.HandleFunc("POST /api/v1/register/verify", s.verifyRegisterCode)
	s.mux.HandleFunc("POST /api/v1/register", s.register)
	s.mux.HandleFunc("POST /api/v1/login", s.login)
	s.mux.HandleFunc("POST /api/v1/teams", s.auth(s.createTeam))
	s.mux.HandleFunc("GET /api/v1/teams", s.auth(s.listTeams))
	s.mux.HandleFunc("GET /api/v1/teams/{id}/workers", s.auth(s.listWorkers))
	s.mux.HandleFunc("POST /api/v1/teams/{id}/workers", s.auth(s.createWorker))
	s.mux.HandleFunc("PUT /api/v1/teams/{id}/workers/{worker_id}", s.auth(s.updateWorker))
	s.mux.HandleFunc("DELETE /api/v1/teams/{id}/workers/{worker_id}", s.auth(s.deleteWorker))
	s.mux.HandleFunc("POST /api/v1/teams/{id}/auto-assign", s.auth(s.autoAssignTasks))
	s.mux.HandleFunc("POST /api/v1/tasks", s.auth(s.createTask))
	s.mux.HandleFunc("GET /api/v1/tasks", s.auth(s.listTasks))
	s.mux.HandleFunc("PUT /api/v1/tasks/{id}", s.auth(s.updateTask))
	s.mux.HandleFunc("DELETE /api/v1/tasks/{id}", s.auth(s.deleteTask))
	s.mux.HandleFunc("GET /api/v1/tasks/{id}/history", s.auth(s.taskHistory))
	s.mux.HandleFunc("GET /api/v1/tasks/{id}/comments", s.auth(s.taskComments))
	s.mux.HandleFunc("POST /api/v1/tasks/{id}/comments", s.auth(s.addTaskComment))
	s.mux.HandleFunc("GET /api/v1/reports/team-summary", s.auth(s.teamSummary))
	s.mux.HandleFunc("GET /api/v1/reports/top-creators", s.auth(s.topCreators))
	s.mux.HandleFunc("GET /api/v1/reports/invalid-assignees", s.auth(s.invalidAssignees))
}

// requestRegisterCode godoc
// @Summary Request registration code
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body registerRequest true "Leader registration data"
// @Success 202 {object} messageResponse
// @Failure 400 {object} errorResponse
// @Failure 502 {object} errorResponse
// @Router /api/v1/register/request-code [post]
func (s *Server) requestRegisterCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	code, err := verificationCode()
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.repo.SaveEmailCode(r.Context(), in.Email, code, in.Name, hash); err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	if err := s.email.SendVerificationCode(r.Context(), in.Email, code); err != nil {
		errorJSON(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "verification code sent"})
}

// verifyRegisterCode godoc
// @Summary Verify registration code
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body verifyRegisterCodeRequest true "Email and verification code"
// @Success 201 {object} authResponse
// @Failure 400 {object} errorResponse
// @Router /api/v1/register/verify [post]
func (s *Server) verifyRegisterCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	user, err := s.repo.CreateUserFromEmailCode(r.Context(), in.Email, in.Code)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, errors.New("invalid or expired code"))
		return
	}
	token, err := auth.IssueToken(s.jwtKey, s.jwtTTL, user.ID, user.Email)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

// register godoc
// @Summary Register leader without email code
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body registerRequest true "Leader registration data"
// @Success 201 {object} authResponse
// @Failure 400 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Router /api/v1/register [post]
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.repo.CreateUser(r.Context(), in.Email, hash, in.Name)
	if err != nil {
		errorJSON(w, http.StatusConflict, err)
		return
	}
	token, err := auth.IssueToken(s.jwtKey, s.jwtTTL, user.ID, user.Email)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

// login godoc
// @Summary Login
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body authRequest true "Credentials"
// @Success 200 {object} authResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /api/v1/login [post]
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	user, err := s.repo.GetUserByEmail(r.Context(), in.Email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, in.Password) {
		errorJSON(w, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}
	token, err := auth.IssueToken(s.jwtKey, s.jwtTTL, user.ID, user.Email)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

// createTeam godoc
// @Summary Create team
// @Tags Teams
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body createTeamRequest true "Team data"
// @Success 201 {object} repository.Team
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /api/v1/teams [post]
func (s *Server) createTeam(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		errorJSON(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	team, err := s.repo.CreateTeam(r.Context(), in.Name, userID(r))
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, team)
}

// listTeams godoc
// @Summary List my teams
// @Tags Teams
// @Produce json
// @Security BearerAuth
// @Success 200 {array} repository.Team
// @Failure 401 {object} errorResponse
// @Router /api/v1/teams [get]
func (s *Server) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.repo.ListTeamsForUser(r.Context(), userID(r))
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

// listWorkers godoc
// @Summary List team workers
// @Tags Workers
// @Produce json
// @Security BearerAuth
// @Param id path int true "Team ID"
// @Success 200 {array} repository.Worker
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/teams/{id}/workers [get]
func (s *Server) listWorkers(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.requireMember(r.Context(), teamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	workers, err := s.repo.ListWorkers(r.Context(), teamID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, workers)
}

// createWorker godoc
// @Summary Create worker
// @Tags Workers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Team ID"
// @Param payload body workerRequest true "Worker data"
// @Success 201 {object} repository.Worker
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/teams/{id}/workers [post]
func (s *Server) createWorker(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.requireMember(r.Context(), teamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	var in struct {
		Name   string   `json:"name"`
		Email  string   `json:"email"`
		Skill  string   `json:"skill"`
		Skills []string `json:"skills"`
	}
	if !decode(w, r, &in) {
		return
	}
	worker, err := s.repo.CreateWorker(r.Context(), repository.Worker{TeamID: teamID, Name: in.Name, Email: in.Email, Skills: normalizeSkills(in.Skills, in.Skill)})
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, worker)
}

// updateWorker godoc
// @Summary Update worker
// @Tags Workers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Team ID"
// @Param worker_id path int true "Worker ID"
// @Param payload body workerRequest true "Worker data"
// @Success 200 {object} repository.Worker
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/teams/{id}/workers/{worker_id} [put]
func (s *Server) updateWorker(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	workerID, ok := pathID(w, r, "worker_id")
	if !ok {
		return
	}
	if err := s.requireMember(r.Context(), teamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	var in struct {
		Name   string   `json:"name"`
		Email  string   `json:"email"`
		Skill  string   `json:"skill"`
		Skills []string `json:"skills"`
	}
	if !decode(w, r, &in) {
		return
	}
	worker, err := s.repo.UpdateWorker(r.Context(), repository.Worker{ID: workerID, TeamID: teamID, Name: in.Name, Email: in.Email, Skills: normalizeSkills(in.Skills, in.Skill)})
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, worker)
}

// deleteWorker godoc
// @Summary Delete worker
// @Tags Workers
// @Produce json
// @Security BearerAuth
// @Param id path int true "Team ID"
// @Param worker_id path int true "Worker ID"
// @Success 200 {object} statusResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/teams/{id}/workers/{worker_id} [delete]
func (s *Server) deleteWorker(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	workerID, ok := pathID(w, r, "worker_id")
	if !ok {
		return
	}
	if err := s.requireMember(r.Context(), teamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	if err := s.repo.DeleteWorker(r.Context(), teamID, workerID); err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// autoAssignTasks godoc
// @Summary Auto assign unassigned tasks by skills
// @Tags Teams
// @Produce json
// @Security BearerAuth
// @Param id path int true "Team ID"
// @Success 200 {object} autoAssignResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/teams/{id}/auto-assign [post]
func (s *Server) autoAssignTasks(w http.ResponseWriter, r *http.Request) {
	teamID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.requireMember(r.Context(), teamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	assigned, err := s.repo.AutoAssignTasks(r.Context(), teamID)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	s.invalidateTasksCache(r.Context(), teamID)
	writeJSON(w, http.StatusOK, map[string]int{"assigned": assigned})
}

// createTask godoc
// @Summary Create task
// @Tags Tasks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body taskRequest true "Task data"
// @Success 201 {object} repository.Task
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/tasks [post]
func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TeamID      int64    `json:"team_id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Status      string   `json:"status"`
		AssigneeID  *int64   `json:"assignee_id"`
		Skill       string   `json:"skill"`
		Skills      []string `json:"skills"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Status == "" {
		in.Status = "backlog"
	}
	if err := s.requireMember(r.Context(), in.TeamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	task, err := s.repo.CreateTask(r.Context(), repository.Task{
		TeamID: in.TeamID, Title: in.Title, Description: in.Description, Status: in.Status,
		AssigneeID: in.AssigneeID, Skills: normalizeSkills(in.Skills, in.Skill), CreatedBy: userID(r),
	})
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	s.invalidateTasksCache(r.Context(), in.TeamID)
	writeJSON(w, http.StatusCreated, task)
}

// listTasks godoc
// @Summary List tasks with filters and pagination
// @Tags Tasks
// @Produce json
// @Security BearerAuth
// @Param team_id query int true "Team ID"
// @Param status query string false "Task status"
// @Param assignee_id query int false "Assignee ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {array} repository.Task
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/v1/tasks [get]
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	teamID, err := strconv.ParseInt(q.Get("team_id"), 10, 64)
	if err != nil || teamID == 0 {
		errorJSON(w, http.StatusBadRequest, errors.New("team_id is required"))
		return
	}
	if err := s.requireMember(r.Context(), teamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	filter := repository.TaskFilter{TeamID: teamID, Status: q.Get("status"), Limit: intParam(q.Get("limit"), 20), Offset: intParam(q.Get("offset"), 0)}
	if v := q.Get("assignee_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			errorJSON(w, http.StatusBadRequest, errors.New("invalid assignee_id"))
			return
		}
		filter.AssigneeID = &id
	}
	cacheKey := fmt.Sprintf("team:%d:tasks:%s:%d:%d:%v", filter.TeamID, filter.Status, filter.Limit, filter.Offset, filter.AssigneeID)
	if b, err := s.redis.Get(r.Context(), cacheKey).Bytes(); err == nil {
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return
	}
	tasks, err := s.repo.ListTasks(r.Context(), filter)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	b, _ := json.Marshal(tasks)
	_ = s.redis.Set(r.Context(), cacheKey, b, 5*time.Minute).Err()
	writeJSON(w, http.StatusOK, tasks)
}

// updateTask godoc
// @Summary Update task
// @Tags Tasks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID"
// @Param payload body updateTaskRequest true "Task data"
// @Success 200 {object} repository.Task
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id} [put]
func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	old, err := s.repo.GetTask(r.Context(), taskID)
	if err != nil {
		errorJSON(w, http.StatusNotFound, err)
		return
	}
	if err := s.requireMember(r.Context(), old.TeamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	var in struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Status      string   `json:"status"`
		AssigneeID  *int64   `json:"assignee_id"`
		Skill       string   `json:"skill"`
		Skills      []string `json:"skills"`
	}
	if !decode(w, r, &in) {
		return
	}
	task, err := s.repo.UpdateTask(r.Context(), userID(r), taskID, in.Title, in.Description, in.Status, normalizeSkills(in.Skills, in.Skill), in.AssigneeID)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	s.invalidateTasksCache(r.Context(), task.TeamID)
	writeJSON(w, http.StatusOK, task)
}

// deleteTask godoc
// @Summary Delete task
// @Tags Tasks
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID"
// @Success 200 {object} statusResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id} [delete]
func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	task, err := s.repo.GetTask(r.Context(), taskID)
	if err != nil {
		errorJSON(w, http.StatusNotFound, err)
		return
	}
	if err := s.requireMember(r.Context(), task.TeamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	if err := s.repo.DeleteTask(r.Context(), task.TeamID, taskID); err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	s.invalidateTasksCache(r.Context(), task.TeamID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// taskHistory godoc
// @Summary Get task history
// @Tags Tasks
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID"
// @Success 200 {array} repository.TaskHistory
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id}/history [get]
func (s *Server) taskHistory(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	task, err := s.repo.GetTask(r.Context(), taskID)
	if err != nil {
		errorJSON(w, http.StatusNotFound, err)
		return
	}
	if err := s.requireMember(r.Context(), task.TeamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	history, err := s.repo.TaskHistory(r.Context(), taskID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

// taskComments godoc
// @Summary List task comments
// @Tags Comments
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID"
// @Success 200 {array} repository.TaskComment
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id}/comments [get]
func (s *Server) taskComments(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	task, err := s.repo.GetTask(r.Context(), taskID)
	if err != nil {
		errorJSON(w, http.StatusNotFound, err)
		return
	}
	if err := s.requireMember(r.Context(), task.TeamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	comments, err := s.repo.ListTaskComments(r.Context(), taskID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, comments)
}

// addTaskComment godoc
// @Summary Add task comment
// @Tags Comments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID"
// @Param payload body commentRequest true "Comment data"
// @Success 201 {object} repository.TaskComment
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/v1/tasks/{id}/comments [post]
func (s *Server) addTaskComment(w http.ResponseWriter, r *http.Request) {
	taskID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	task, err := s.repo.GetTask(r.Context(), taskID)
	if err != nil {
		errorJSON(w, http.StatusNotFound, err)
		return
	}
	if err := s.requireMember(r.Context(), task.TeamID, userID(r)); err != nil {
		errorJSON(w, http.StatusForbidden, err)
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Body) == "" {
		errorJSON(w, http.StatusBadRequest, errors.New("comment body is required"))
		return
	}
	comment, err := s.repo.AddTaskComment(r.Context(), taskID, userID(r), in.Body)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

// teamSummary godoc
// @Summary Team summary report
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Success 200 {array} repository.TeamSummary
// @Failure 401 {object} errorResponse
// @Router /api/v1/reports/team-summary [get]
func (s *Server) teamSummary(w http.ResponseWriter, r *http.Request) {
	out, err := s.repo.TeamSummary(r.Context())
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// topCreators godoc
// @Summary Top task creators per team
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Success 200 {array} repository.TopCreator
// @Failure 401 {object} errorResponse
// @Router /api/v1/reports/top-creators [get]
func (s *Server) topCreators(w http.ResponseWriter, r *http.Request) {
	out, err := s.repo.TopCreators(r.Context())
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// invalidAssignees godoc
// @Summary Find tasks assigned to users outside their team
// @Tags Reports
// @Produce json
// @Security BearerAuth
// @Success 200 {array} repository.InvalidAssignee
// @Failure 401 {object} errorResponse
// @Router /api/v1/reports/invalid-assignees [get]
func (s *Server) invalidAssignees(w http.ResponseWriter, r *http.Request) {
	out, err := s.repo.InvalidAssignees(r.Context())
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) requireMember(ctx context.Context, teamID, actorID int64) error {
	if _, err := s.repo.TeamRole(ctx, teamID, actorID); err != nil {
		return errors.New("team membership required")
	}
	return nil
}

func (s *Server) invalidateTasksCache(ctx context.Context, teamID int64) {
	iter := s.redis.Scan(ctx, 0, fmt.Sprintf("team:%d:tasks:*", teamID), 100).Iterator()
	for iter.Next(ctx) {
		_ = s.redis.Del(ctx, iter.Val()).Err()
	}
}

func verificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func normalizeSkill(skill string) string {
	skill = strings.ToLower(strings.TrimSpace(skill))
	if skill == "" {
		return "general"
	}
	return skill
}

func normalizeSkills(skills []string, fallback string) []string {
	if len(skills) == 0 && fallback != "" {
		skills = strings.Split(fallback, ",")
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(skills))
	for _, skill := range skills {
		normalized := normalizeSkill(skill)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return []string{"general"}
	}
	return out
}
