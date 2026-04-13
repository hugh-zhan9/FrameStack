package config_test

import (
	"testing"
	"time"

	"idea/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("IDEA_HTTP_ADDR", "")
	t.Setenv("IDEA_DATABASE_URL", "")
	t.Setenv("IDEA_DEFAULT_PROVIDER", "")
	t.Setenv("IDEA_ENABLE_DATABASE", "")
	t.Setenv("IDEA_RUN_JOB_WORKER", "")
	t.Setenv("IDEA_JOB_WORKER_NAME", "")
	t.Setenv("IDEA_JOB_POLL_INTERVAL", "")
	t.Setenv("IDEA_WORKER_COMMAND", "")
	t.Setenv("IDEA_WORKER_SCRIPT", "")

	cfg := config.Load()

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("expected default http addr %q, got %q", ":8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://localhost:5432/idea?sslmode=disable" {
		t.Fatalf("unexpected default database url: %q", cfg.DatabaseURL)
	}
	if cfg.DefaultProvider != "ollama" {
		t.Fatalf("expected default provider %q, got %q", "ollama", cfg.DefaultProvider)
	}
	if cfg.EnableDatabase {
		t.Fatal("expected enable database to default to false")
	}
	if !cfg.RunJobWorker {
		t.Fatal("expected run job worker to default to true")
	}
	if cfg.JobWorkerName != "local-server" {
		t.Fatalf("expected default job worker name %q, got %q", "local-server", cfg.JobWorkerName)
	}
	if cfg.JobPollInterval != 2*time.Second {
		t.Fatalf("expected default job poll interval %s, got %s", 2*time.Second, cfg.JobPollInterval)
	}
	if cfg.MigrationsDir != "db/migrations" {
		t.Fatalf("expected default migrations dir %q, got %q", "db/migrations", cfg.MigrationsDir)
	}
	if cfg.RunMigrations {
		t.Fatal("expected run migrations to default to false")
	}
	if cfg.WorkerCommand != "python3" {
		t.Fatalf("expected default worker command %q, got %q", "python3", cfg.WorkerCommand)
	}
	if cfg.WorkerScript != "worker/main.py" {
		t.Fatalf("expected default worker script %q, got %q", "worker/main.py", cfg.WorkerScript)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("IDEA_HTTP_ADDR", ":9090")
	t.Setenv("IDEA_DATABASE_URL", "postgres://example/db")
	t.Setenv("IDEA_DEFAULT_PROVIDER", "lm_studio")
	t.Setenv("IDEA_ENABLE_DATABASE", "true")
	t.Setenv("IDEA_RUN_JOB_WORKER", "false")
	t.Setenv("IDEA_JOB_WORKER_NAME", "worker-a")
	t.Setenv("IDEA_JOB_POLL_INTERVAL", "5s")
	t.Setenv("IDEA_MIGRATIONS_DIR", "db/custom")
	t.Setenv("IDEA_RUN_MIGRATIONS", "true")
	t.Setenv("IDEA_WORKER_COMMAND", "uv")
	t.Setenv("IDEA_WORKER_SCRIPT", "worker/app.py")

	cfg := config.Load()

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("expected env http addr %q, got %q", ":9090", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://example/db" {
		t.Fatalf("expected env database url, got %q", cfg.DatabaseURL)
	}
	if cfg.DefaultProvider != "lm_studio" {
		t.Fatalf("expected env provider, got %q", cfg.DefaultProvider)
	}
	if !cfg.EnableDatabase {
		t.Fatal("expected enable database to be true from env")
	}
	if cfg.RunJobWorker {
		t.Fatal("expected run job worker to be false from env")
	}
	if cfg.JobWorkerName != "worker-a" {
		t.Fatalf("expected env job worker name, got %q", cfg.JobWorkerName)
	}
	if cfg.JobPollInterval != 5*time.Second {
		t.Fatalf("expected env job poll interval, got %s", cfg.JobPollInterval)
	}
	if cfg.MigrationsDir != "db/custom" {
		t.Fatalf("expected env migrations dir, got %q", cfg.MigrationsDir)
	}
	if !cfg.RunMigrations {
		t.Fatal("expected run migrations to be true from env")
	}
	if cfg.WorkerCommand != "uv" {
		t.Fatalf("expected env worker command, got %q", cfg.WorkerCommand)
	}
	if cfg.WorkerScript != "worker/app.py" {
		t.Fatalf("expected env worker script, got %q", cfg.WorkerScript)
	}
}
