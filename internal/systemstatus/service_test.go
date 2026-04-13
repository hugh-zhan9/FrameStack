package systemstatus_test

import (
	"context"
	"errors"
	"testing"

	"idea/internal/systemstatus"
)

func TestServiceReturnsReadyWhenAllEnabledChecksPass(t *testing.T) {
	service := systemstatus.Service{
		EnableDatabase: true,
		RequireWorker:  true,
		Database:       staticChecker{},
		Worker:         staticChecker{},
	}

	snapshot, err := service.SystemStatus(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot.Status != "ready" {
		t.Fatalf("expected ready, got %#v", snapshot)
	}
	if len(snapshot.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %#v", snapshot)
	}
}

func TestServiceReturnsNotReadyWhenWorkerFails(t *testing.T) {
	service := systemstatus.Service{
		EnableDatabase: true,
		RequireWorker:  true,
		Database:       staticChecker{},
		Worker:         staticChecker{err: errors.New("worker unavailable")},
	}

	snapshot, err := service.SystemStatus(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot.Status != "not_ready" {
		t.Fatalf("expected not_ready, got %#v", snapshot)
	}
	if snapshot.Checks[1].Status != "not_ready" {
		t.Fatalf("expected worker not_ready, got %#v", snapshot.Checks)
	}
}

func TestServiceMarksDisabledChecksExplicitly(t *testing.T) {
	service := systemstatus.Service{}

	snapshot, err := service.SystemStatus(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot.Status != "ready" {
		t.Fatalf("expected ready, got %#v", snapshot)
	}
	if len(snapshot.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %#v", snapshot)
	}
	for _, item := range snapshot.Checks {
		if item.Status != "disabled" {
			t.Fatalf("expected disabled checks, got %#v", snapshot.Checks)
		}
	}
}

type staticChecker struct {
	err error
}

func (s staticChecker) CheckHealth(_ context.Context) error {
	return s.err
}
