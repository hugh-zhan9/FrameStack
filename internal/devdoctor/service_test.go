package devdoctor_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"idea/internal/config"
	"idea/internal/devdoctor"
)

func TestServiceReturnsReadyForMinimalPlaceholderSetup(t *testing.T) {
	service := devdoctor.Service{
		Config:         config.Config{EnableDatabase: false, WorkerScript: "worker/main.py", MigrationsDir: "db/migrations"},
		WorkerProvider: "placeholder",
		LookPath: func(name string) (string, error) {
			switch name {
			case "go", "python3", "node", "ffprobe":
				return "/usr/bin/" + name, nil
			default:
				return "", errors.New("missing")
			}
		},
		Stat: func(path string) (os.FileInfo, error) {
			if path == "worker/main.py" || path == "db/migrations" {
				return fakeFileInfo{name: path}, nil
			}
			return nil, os.ErrNotExist
		},
	}

	report := service.Run(context.Background())
	if report.Status != "ready" {
		t.Fatalf("expected ready, got %#v", report)
	}
}

func TestServiceReturnsNotReadyWhenWorkerScriptMissing(t *testing.T) {
	service := devdoctor.Service{
		Config:         config.Config{EnableDatabase: false, WorkerScript: "worker/main.py", MigrationsDir: "db/migrations"},
		WorkerProvider: "placeholder",
		LookPath: func(name string) (string, error) {
			switch name {
			case "go", "python3", "node":
				return "/usr/bin/" + name, nil
			default:
				return "", errors.New("missing")
			}
		},
		Stat: func(path string) (os.FileInfo, error) {
			if path == "db/migrations" {
				return fakeFileInfo{name: path}, nil
			}
			return nil, os.ErrNotExist
		},
	}

	report := service.Run(context.Background())
	if report.Status != "not_ready" {
		t.Fatalf("expected not_ready, got %#v", report)
	}
}

func TestServiceReturnsWarningWhenOptionalToolMissing(t *testing.T) {
	service := devdoctor.Service{
		Config:         config.Config{EnableDatabase: false, WorkerScript: "worker/main.py", MigrationsDir: "db/migrations"},
		WorkerProvider: "placeholder",
		LookPath: func(name string) (string, error) {
			switch name {
			case "go", "python3", "node":
				return "/usr/bin/" + name, nil
			default:
				return "", errors.New("missing")
			}
		},
		Stat: func(path string) (os.FileInfo, error) {
			if path == "worker/main.py" || path == "db/migrations" {
				return fakeFileInfo{name: path}, nil
			}
			return nil, os.ErrNotExist
		},
	}

	report := service.Run(context.Background())
	if report.Status != "warning" {
		t.Fatalf("expected warning, got %#v", report)
	}
}

type fakeFileInfo struct {
	name string
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return os.ModePerm }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return true }
func (f fakeFileInfo) Sys() any           { return nil }
