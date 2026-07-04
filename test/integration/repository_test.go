//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"team-task-tracker-api/internal/db"
	"team-task-tracker-api/internal/repository"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRepositoryWithPostgres(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") == "0" {
		t.Skip("RUN_INTEGRATION=0 disables integration tests")
	}
	ctx := context.Background()
	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("tracker"),
		postgres.WithUsername("tracker"),
		postgres.WithPassword("tracker"),
		postgres.WithInitScripts("../../migrations/001_init.sql"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := db.NewPostgres(ctx, dsn, 4)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := repository.New(pool)
	user, err := repo.CreateUser(ctx, "owner@example.com", "hash", "Owner")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	team, err := repo.CreateTeam(ctx, "Core", user.ID)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	worker, err := repo.CreateWorker(ctx, repository.Worker{TeamID: team.ID, Name: "Go Dev", Email: "go@example.com", Skills: []string{"go", "postgres"}})
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}
	task, err := repo.CreateTask(ctx, repository.Task{TeamID: team.ID, Title: "Ship", Status: "todo", Skills: []string{"go"}, CreatedBy: user.ID, AssigneeID: &worker.ID})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, err := repo.UpdateTask(ctx, user.ID, task.ID, "Ship API", "done", "done", []string{"go", "postgres"}, &worker.ID); err != nil {
		t.Fatalf("update task: %v", err)
	}
	history, err := repo.TaskHistory(ctx, task.ID)
	if err != nil {
		t.Fatalf("task history: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("expected history rows")
	}
}
