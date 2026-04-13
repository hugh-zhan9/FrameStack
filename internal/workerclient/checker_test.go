package workerclient_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"idea/internal/workerclient"
)

func TestHealthCheckerChecksWorkerHealth(t *testing.T) {
	checker := workerclient.HealthChecker{
		Client: workerclient.Client{
			Command: "python3",
			Script:  filepath.Clean(filepath.Join("..", "..", "worker", "main.py")),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := checker.CheckWorkerHealth(ctx); err != nil {
		t.Fatalf("expected health checker to succeed: %v", err)
	}
}
