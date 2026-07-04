package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"team-task-tracker-api/internal/config"
	"team-task-tracker-api/internal/db"
	"team-task-tracker-api/internal/httpapi"
	"team-task-tracker-api/internal/metrics"
	"team-task-tracker-api/internal/ratelimit"
	"team-task-tracker-api/internal/repository"
	"team-task-tracker-api/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	pg, err := connectPostgres(ctx, cfg.Database.DSN, cfg.Database.MaxConns)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()
	if err := db.Migrate(ctx, pg, "migrations"); err != nil {
		if err := db.Migrate(ctx, pg, "/app/migrations"); err != nil {
			slog.Error("migrate postgres", "error", err)
			os.Exit(1)
		}
	}

	redisClient := db.NewRedis(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Warn("redis unavailable; cache operations will fail open", "error", err)
	}
	defer redisClient.Close()

	reg := prometheus.NewRegistry()
	rec := metrics.New(reg)
	api := httpapi.New(
		repository.New(pg),
		redisClient,
		service.NewEmailService(cfg.Email.Endpoint),
		cfg.JWT.Secret,
		cfg.JWTTTL(),
		ratelimit.New(cfg.RateLimit.RequestsPerMinute),
	)

	root := http.NewServeMux()
	root.Handle("/", rec.Middleware(api.Handler()))
	root.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("api started", "addr", cfg.Server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
}

func connectPostgres(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err := db.NewPostgres(ctx, dsn, maxConns)
		if err == nil {
			return pool, nil
		}
		lastErr = err
		slog.Warn("postgres not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, lastErr
}
