package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	DefaultProvider string
	EnableDatabase  bool
	RunJobWorker    bool
	JobWorkerName   string
	JobPollInterval time.Duration
	MigrationsDir   string
	RunMigrations   bool
	WorkerCommand   string
	WorkerScript    string
}

func Load() Config {
	return Config{
		HTTPAddr:        getenv("IDEA_HTTP_ADDR", ":8080"),
		DatabaseURL:     getenv("IDEA_DATABASE_URL", "postgres://localhost:5432/idea?sslmode=disable"),
		DefaultProvider: getenv("IDEA_DEFAULT_PROVIDER", "ollama"),
		EnableDatabase:  getenvBool("IDEA_ENABLE_DATABASE", false),
		RunJobWorker:    getenvBool("IDEA_RUN_JOB_WORKER", true),
		JobWorkerName:   getenv("IDEA_JOB_WORKER_NAME", "local-server"),
		JobPollInterval: getenvDuration("IDEA_JOB_POLL_INTERVAL", 2*time.Second),
		MigrationsDir:   getenv("IDEA_MIGRATIONS_DIR", "db/migrations"),
		RunMigrations:   getenvBool("IDEA_RUN_MIGRATIONS", false),
		WorkerCommand:   getenv("IDEA_WORKER_COMMAND", "python3"),
		WorkerScript:    getenv("IDEA_WORKER_SCRIPT", "worker/main.py"),
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
