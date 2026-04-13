package jobrunner

import (
	"context"
	"time"

	"idea/internal/tasks"
)

type Store interface {
	LeaseNextJob(ctx context.Context, workerName string) (tasks.Job, bool, error)
	MarkJobRunning(ctx context.Context, jobID int64, workerName string, stage string) (tasks.Job, error)
	MarkJobSucceeded(ctx context.Context, jobID int64) (tasks.Job, error)
	MarkJobFailed(ctx context.Context, jobID int64, message string) (tasks.Job, error)
}

type Executor interface {
	ExecuteJob(ctx context.Context, job tasks.Job) error
}

type Runner struct {
	WorkerName string
	Store      Store
	Executor   Executor
	Recorder   tasks.JobEventRecorder
}

func (r Runner) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := r.PollOnce(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r Runner) PollOnce(ctx context.Context) error {
	job, ok, err := r.Store.LeaseNextJob(ctx, r.WorkerName)
	if err != nil || !ok {
		return err
	}

	_ = r.record(ctx, job.ID, "info", "job leased")

	job, err = r.Store.MarkJobRunning(ctx, job.ID, r.WorkerName, "dispatch")
	if err != nil {
		return err
	}
	_ = r.record(ctx, job.ID, "info", "job started")

	if err := r.Executor.ExecuteJob(ctx, job); err != nil {
		_, markErr := r.Store.MarkJobFailed(ctx, job.ID, err.Error())
		if markErr != nil {
			return markErr
		}
		_ = r.record(ctx, job.ID, "error", err.Error())
		return nil
	}

	if _, err := r.Store.MarkJobSucceeded(ctx, job.ID); err != nil {
		return err
	}
	_ = r.record(ctx, job.ID, "info", "job completed")
	return nil
}

func (r Runner) record(ctx context.Context, jobID int64, level string, message string) error {
	if r.Recorder == nil {
		return nil
	}
	return r.Recorder.RecordJobEvent(ctx, tasks.CreateJobEventInput{
		JobID:   jobID,
		Level:   level,
		Message: message,
	})
}
