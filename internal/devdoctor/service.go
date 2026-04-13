package devdoctor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"idea/internal/config"
)

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Report struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

type Service struct {
	Config         config.Config
	WorkerProvider string
	LookPath       func(string) (string, error)
	Stat           func(string) (os.FileInfo, error)
}

func (s Service) Run(_ context.Context) Report {
	lookPath := s.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	stat := s.Stat
	if stat == nil {
		stat = os.Stat
	}

	checks := []Check{
		checkCommand(lookPath, "go", true),
		checkCommand(lookPath, "python3", true),
		checkCommand(lookPath, "node", true),
		checkCommand(lookPath, "ffprobe", false),
		checkPath(stat, "worker_script", s.Config.WorkerScript, true),
		checkPath(stat, "migrations_dir", s.Config.MigrationsDir, true),
		s.checkDatabase(),
		s.checkProvider(),
	}

	status := "ready"
	hasWarning := false
	for _, item := range checks {
		switch item.Status {
		case "not_ready":
			status = "not_ready"
		case "warning":
			hasWarning = true
		}
	}
	if status != "not_ready" && hasWarning {
		status = "warning"
	}

	return Report{
		Status: status,
		Checks: checks,
	}
}

func (s Service) checkDatabase() Check {
	if !s.Config.EnableDatabase {
		return Check{Name: "database", Status: "disabled", Message: "database disabled"}
	}
	if strings.TrimSpace(s.Config.DatabaseURL) == "" {
		return Check{Name: "database", Status: "not_ready", Message: "database url is empty"}
	}
	return Check{Name: "database", Status: "ready", Message: "database url configured"}
}

func (s Service) checkProvider() Check {
	provider := strings.TrimSpace(s.WorkerProvider)
	if provider == "" {
		provider = "placeholder"
	}
	switch provider {
	case "placeholder":
		return Check{Name: "worker_provider", Status: "ready", Message: "placeholder provider enabled"}
	case "ollama":
		return Check{Name: "worker_provider", Status: "ready", Message: "ollama provider configured"}
	case "lm_studio":
		return Check{Name: "worker_provider", Status: "ready", Message: "lm_studio provider configured"}
	default:
		return Check{Name: "worker_provider", Status: "warning", Message: "unknown worker provider: " + provider}
	}
}

func checkCommand(lookPath func(string) (string, error), name string, required bool) Check {
	_, err := lookPath(name)
	if err == nil {
		return Check{Name: name, Status: "ready", Message: name + " found"}
	}
	if required {
		return Check{Name: name, Status: "not_ready", Message: name + " not found"}
	}
	return Check{Name: name, Status: "warning", Message: name + " not found"}
}

func checkPath(stat func(string) (os.FileInfo, error), name string, path string, required bool) Check {
	if strings.TrimSpace(path) == "" {
		if required {
			return Check{Name: name, Status: "not_ready", Message: name + " is empty"}
		}
		return Check{Name: name, Status: "warning", Message: name + " is empty"}
	}
	_, err := stat(path)
	if err == nil {
		return Check{Name: name, Status: "ready", Message: path}
	}
	if required {
		return Check{Name: name, Status: "not_ready", Message: path + " not found"}
	}
	if errors.Is(err, os.ErrNotExist) {
		return Check{Name: name, Status: "warning", Message: path + " not found"}
	}
	return Check{Name: name, Status: "warning", Message: err.Error()}
}
