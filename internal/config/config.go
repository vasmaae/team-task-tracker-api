package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	Database struct {
		DSN      string `yaml:"dsn"`
		MaxConns int32  `yaml:"max_conns"`
	} `yaml:"database"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	JWT struct {
		Secret     string `yaml:"secret"`
		TTLMinutes int    `yaml:"ttl_minutes"`
	} `yaml:"jwt"`
	RateLimit struct {
		RequestsPerMinute int `yaml:"requests_per_minute"`
	} `yaml:"rate_limit"`
	Email struct {
		Endpoint string `yaml:"endpoint"`
	} `yaml:"email"`
}

func Load(path string) (Config, error) {
	cfg := defaults()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, err
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	return cfg, nil
}

func defaults() Config {
	var cfg Config
	cfg.Server.Addr = ":8080"
	cfg.Database.DSN = "postgres://tracker:tracker@localhost:5432/tracker?sslmode=disable"
	cfg.Database.MaxConns = 20
	cfg.Redis.Addr = "localhost:6379"
	cfg.JWT.Secret = "dev-secret"
	cfg.JWT.TTLMinutes = 1440
	cfg.RateLimit.RequestsPerMinute = 100
	cfg.Email.Endpoint = "mock://email-service"
	return cfg
}

func (c Config) JWTTTL() time.Duration {
	return time.Duration(c.JWT.TTLMinutes) * time.Minute
}

func applyEnv(cfg *Config) {
	setString(&cfg.Server.Addr, "SERVER_ADDR")
	setString(&cfg.Database.DSN, "DATABASE_DSN")
	setString(&cfg.Redis.Addr, "REDIS_ADDR")
	setString(&cfg.Redis.Password, "REDIS_PASSWORD")
	setString(&cfg.JWT.Secret, "JWT_SECRET")
	setString(&cfg.Email.Endpoint, "EMAIL_ENDPOINT")
	setInt32(&cfg.Database.MaxConns, "DATABASE_MAX_CONNS")
	setInt(&cfg.Redis.DB, "REDIS_DB")
	setInt(&cfg.JWT.TTLMinutes, "JWT_TTL_MINUTES")
	setInt(&cfg.RateLimit.RequestsPerMinute, "RATE_LIMIT_REQUESTS_PER_MINUTE")
}

func setString(target *string, key string) {
	if v := os.Getenv(key); v != "" {
		*target = v
	}
}

func setInt(target *int, key string) {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			*target = parsed
		}
	}
}

func setInt32(target *int32, key string) {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			*target = int32(parsed)
		}
	}
}
