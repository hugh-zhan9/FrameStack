package tasks_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"idea/internal/tasks"
)

func TestPostgresQueueStoreLeaseNextJobReturnsJob(t *testing.T) {
	now := time.Date(2026, 4, 9, 16, 0, 0, 0, time.UTC)
	queryer := &recordingJobCreateQueryer{
		row: staticJobCreateRow{
			item: jobRow{
				ID:           31,
				JobType:      "scan_volume",
				Status:       "leased",
				TargetType:   "volume",
				TargetID:     2,
				AttemptCount: 1,
				MaxAttempts:  3,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
	store := tasks.PostgresQueueStore{Queryer: queryer}

	item, ok, err := store.LeaseNextJob(context.Background(), "local-server")
	if err != nil {
		t.Fatalf("expected lease to succeed: %v", err)
	}
	if !ok {
		t.Fatal("expected a leased job")
	}
	if item.ID != 31 || item.Status != "leased" {
		t.Fatalf("unexpected leased job: %#v", item)
	}
	if len(queryer.args) != 1 || queryer.args[0] != "local-server" {
		t.Fatalf("unexpected lease args: %#v", queryer.args)
	}
	if !strings.Contains(normalizeSQL(queryer.query), normalizeSQL("update jobs")) {
		t.Fatalf("unexpected lease query: %s", queryer.query)
	}
}

func TestPostgresQueueStoreLeaseNextJobReturnsEmptyWhenNoJob(t *testing.T) {
	store := tasks.PostgresQueueStore{
		Queryer: &recordingJobCreateQueryer{
			row: staticJobCreateRow{err: sql.ErrNoRows},
		},
	}

	_, ok, err := store.LeaseNextJob(context.Background(), "local-server")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ok {
		t.Fatal("expected no leased job")
	}
}

func TestPostgresQueueStoreMarkJobRunningReturnsUpdatedJob(t *testing.T) {
	now := time.Date(2026, 4, 9, 16, 5, 0, 0, time.UTC)
	queryer := &recordingJobCreateQueryer{
		row: staticJobCreateRow{
			item: jobRow{
				ID:           31,
				JobType:      "scan_volume",
				Status:       "running",
				TargetType:   "volume",
				TargetID:     2,
				AttemptCount: 1,
				MaxAttempts:  3,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
	store := tasks.PostgresQueueStore{Queryer: queryer}

	item, err := store.MarkJobRunning(context.Background(), 31, "local-server", "dispatch")
	if err != nil {
		t.Fatalf("expected mark running to succeed: %v", err)
	}
	if item.Status != "running" {
		t.Fatalf("unexpected running job: %#v", item)
	}
}

func TestPostgresQueueStoreMarkJobFailedReturnsUpdatedJob(t *testing.T) {
	now := time.Date(2026, 4, 9, 16, 10, 0, 0, time.UTC)
	queryer := &recordingJobCreateQueryer{
		row: staticJobCreateRow{
			item: jobRow{
				ID:           31,
				JobType:      "scan_volume",
				Status:       "failed",
				TargetType:   "volume",
				TargetID:     2,
				AttemptCount: 1,
				MaxAttempts:  3,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
	store := tasks.PostgresQueueStore{Queryer: queryer}

	item, err := store.MarkJobFailed(context.Background(), 31, "boom")
	if err != nil {
		t.Fatalf("expected mark failed to succeed: %v", err)
	}
	if item.Status != "failed" {
		t.Fatalf("unexpected failed job: %#v", item)
	}
}

func TestPostgresQueueStoreMarkJobSucceededReturnsUpdatedJob(t *testing.T) {
	now := time.Date(2026, 4, 9, 16, 15, 0, 0, time.UTC)
	queryer := &recordingJobCreateQueryer{
		row: staticJobCreateRow{
			item: jobRow{
				ID:           31,
				JobType:      "scan_volume",
				Status:       "succeeded",
				TargetType:   "volume",
				TargetID:     2,
				AttemptCount: 1,
				MaxAttempts:  3,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
	store := tasks.PostgresQueueStore{Queryer: queryer}

	item, err := store.MarkJobSucceeded(context.Background(), 31)
	if err != nil {
		t.Fatalf("expected mark succeeded to succeed: %v", err)
	}
	if item.Status != "succeeded" {
		t.Fatalf("unexpected succeeded job: %#v", item)
	}
}

func TestNoopExecutorFailsUnsupportedJobType(t *testing.T) {
	executor := tasks.NoopExecutor{}

	err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: "unsupported_job"})
	if err == nil {
		t.Fatal("expected unsupported job type to fail")
	}
	if !strings.Contains(err.Error(), "no handler") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoopExecutorSucceedsForSupportedDevelopmentJobType(t *testing.T) {
	executor := tasks.NoopExecutor{}

	if err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: "scan_volume"}); err != nil {
		t.Fatalf("expected scan_volume to succeed: %v", err)
	}
	if err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: "infer_quality"}); err != nil {
		t.Fatalf("expected infer_quality to succeed: %v", err)
	}
	if err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: "hash_file"}); err != nil {
		t.Fatalf("expected hash_file to succeed: %v", err)
	}
	if err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: "cluster_same_content"}); err != nil {
		t.Fatalf("expected cluster_same_content to succeed: %v", err)
	}
	if err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: "cluster_same_series"}); err != nil {
		t.Fatalf("expected cluster_same_series to succeed: %v", err)
	}
	if err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: "cluster_same_person"}); err != nil {
		t.Fatalf("expected cluster_same_person to succeed: %v", err)
	}
}

func TestEventRecorderReturnsExecError(t *testing.T) {
	recorder := tasks.PostgresJobEventRecorder{
		Execer: eventErrorExecer{err: errors.New("write failed")},
	}

	err := recorder.RecordJobEvent(context.Background(), tasks.CreateJobEventInput{
		JobID:   1,
		Level:   "info",
		Message: "hello",
	})
	if err == nil {
		t.Fatal("expected record event to fail")
	}
}

type eventErrorExecer struct {
	err error
}

func (e eventErrorExecer) ExecContext(_ context.Context, _ string, _ ...any) error {
	return e.err
}
