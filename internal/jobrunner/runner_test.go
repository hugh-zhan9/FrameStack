package jobrunner_test

import (
	"context"
	"errors"
	"testing"

	"idea/internal/jobrunner"
	"idea/internal/tasks"
)

func TestRunnerPollOnceExecutesJobAndMarksSucceeded(t *testing.T) {
	store := &recordingStore{
		leasedJob: tasks.Job{ID: 7, JobType: "scan_volume", Status: "leased"},
	}
	executor := &recordingExecutor{}
	recorder := &recordingEventRecorder{}
	runner := jobrunner.Runner{
		WorkerName: "local-server",
		Store:      store,
		Executor:   executor,
		Recorder:   recorder,
	}

	if err := runner.PollOnce(context.Background()); err != nil {
		t.Fatalf("expected poll once to succeed: %v", err)
	}
	if !store.leased {
		t.Fatal("expected job to be leased")
	}
	if !store.running {
		t.Fatal("expected job to be marked running")
	}
	if !store.succeeded {
		t.Fatal("expected job to be marked succeeded")
	}
	if executor.jobID != 7 {
		t.Fatalf("expected executor to receive job 7, got %d", executor.jobID)
	}
	if len(recorder.events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(recorder.events))
	}
}

func TestRunnerPollOnceMarksJobFailedWhenExecutorFails(t *testing.T) {
	store := &recordingStore{
		leasedJob: tasks.Job{ID: 8, JobType: "infer_tags", Status: "leased"},
	}
	executor := &recordingExecutor{err: errors.New("boom")}
	recorder := &recordingEventRecorder{}
	runner := jobrunner.Runner{
		WorkerName: "local-server",
		Store:      store,
		Executor:   executor,
		Recorder:   recorder,
	}

	if err := runner.PollOnce(context.Background()); err != nil {
		t.Fatalf("expected poll once to keep succeeding after job failure: %v", err)
	}
	if !store.failed {
		t.Fatal("expected job to be marked failed")
	}
	if store.lastError != "boom" {
		t.Fatalf("expected failure message to be recorded, got %q", store.lastError)
	}
	if len(recorder.events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(recorder.events))
	}
}

func TestRunnerPollOnceDoesNothingWhenQueueIsEmpty(t *testing.T) {
	runner := jobrunner.Runner{
		WorkerName: "local-server",
		Store:      &recordingStore{},
		Executor:   &recordingExecutor{},
		Recorder:   &recordingEventRecorder{},
	}

	if err := runner.PollOnce(context.Background()); err != nil {
		t.Fatalf("expected poll once to succeed: %v", err)
	}
}

type recordingStore struct {
	leasedJob  tasks.Job
	leased     bool
	running    bool
	succeeded  bool
	failed     bool
	lastError  string
}

func (s *recordingStore) LeaseNextJob(_ context.Context, _ string) (tasks.Job, bool, error) {
	if s.leasedJob.ID == 0 {
		return tasks.Job{}, false, nil
	}
	s.leased = true
	return s.leasedJob, true, nil
}

func (s *recordingStore) MarkJobRunning(_ context.Context, jobID int64, _ string, _ string) (tasks.Job, error) {
	s.running = true
	return tasks.Job{ID: jobID, JobType: s.leasedJob.JobType, Status: "running"}, nil
}

func (s *recordingStore) MarkJobSucceeded(_ context.Context, jobID int64) (tasks.Job, error) {
	s.succeeded = true
	return tasks.Job{ID: jobID, JobType: s.leasedJob.JobType, Status: "succeeded"}, nil
}

func (s *recordingStore) MarkJobFailed(_ context.Context, jobID int64, message string) (tasks.Job, error) {
	s.failed = true
	s.lastError = message
	return tasks.Job{ID: jobID, JobType: s.leasedJob.JobType, Status: "failed"}, nil
}

type recordingExecutor struct {
	jobID int64
	err   error
}

func (e *recordingExecutor) ExecuteJob(_ context.Context, job tasks.Job) error {
	e.jobID = job.ID
	return e.err
}

type recordingEventRecorder struct {
	events []tasks.CreateJobEventInput
}

func (r *recordingEventRecorder) RecordJobEvent(_ context.Context, input tasks.CreateJobEventInput) error {
	r.events = append(r.events, input)
	return nil
}
