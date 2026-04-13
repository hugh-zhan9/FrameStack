package tasks

import (
	"context"
	"fmt"
	"time"
)

type JobEvent struct {
	ID        int64     `json:"id"`
	JobID     int64     `json:"job_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type JobEventListProvider interface {
	ListJobEvents(ctx context.Context, jobID int64, limit int) ([]JobEvent, error)
}

type CreateJobEventInput struct {
	JobID   int64
	Level   string
	Message string
}

type JobEventRecorder interface {
	RecordJobEvent(ctx context.Context, input CreateJobEventInput) error
}

type JobEventExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type PostgresJobEventListProvider struct {
	Queryer JobListQueryer
}

type PostgresJobEventRecorder struct {
	Execer JobEventExecer
}

func (p PostgresJobEventListProvider) ListJobEvents(ctx context.Context, jobID int64, limit int) ([]JobEvent, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := p.Queryer.QueryContext(ctx, jobEventsQuery, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []JobEvent
	for rows.Next() {
		var item JobEvent
		if err := rows.Scan(&item.ID, &item.JobID, &item.Level, &item.Message, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func NewPostgresJobEventListProviderFromDB(db SQLRowsQueryer) PostgresJobEventListProvider {
	return PostgresJobEventListProvider{
		Queryer: sqlJobListQueryer{db: db},
	}
}

func (p PostgresJobEventRecorder) RecordJobEvent(ctx context.Context, input CreateJobEventInput) error {
	level := input.Level
	if level == "" {
		level = "info"
	}
	return p.Execer.ExecContext(ctx, createJobEventQuery, input.JobID, level, input.Message)
}

type EventRecordingJobCreator struct {
	Creator  JobCreator
	Recorder JobEventRecorder
}

func (c EventRecordingJobCreator) CreateJob(ctx context.Context, input CreateJobInput) (Job, error) {
	item, err := c.Creator.CreateJob(ctx, input)
	if err != nil {
		return Job{}, err
	}
	if c.Recorder != nil {
		if err := c.Recorder.RecordJobEvent(ctx, CreateJobEventInput{
			JobID:   item.ID,
			Level:   "info",
			Message: fmt.Sprintf("job created: %s", item.JobType),
		}); err != nil {
			return Job{}, err
		}
	}
	return item, nil
}

type EventRecordingJobRetrier struct {
	Retrier  JobRetrier
	Recorder JobEventRecorder
}

func (r EventRecordingJobRetrier) RetryJob(ctx context.Context, jobID int64) (Job, error) {
	item, err := r.Retrier.RetryJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if r.Recorder != nil {
		if err := r.Recorder.RecordJobEvent(ctx, CreateJobEventInput{
			JobID:   item.ID,
			Level:   "info",
			Message: fmt.Sprintf("job retried: %s", item.JobType),
		}); err != nil {
			return Job{}, err
		}
	}
	return item, nil
}

type StaticJobCreator struct {
	Item Job
	Err  error
}

func (c StaticJobCreator) CreateJob(_ context.Context, _ CreateJobInput) (Job, error) {
	return c.Item, c.Err
}

type StaticJobRetrier struct {
	Item Job
	Err  error
}

func (r StaticJobRetrier) RetryJob(_ context.Context, _ int64) (Job, error) {
	return r.Item, r.Err
}

type StaticJobEventRecorder struct {
	Err error
}

func (r StaticJobEventRecorder) RecordJobEvent(_ context.Context, _ CreateJobEventInput) error {
	return r.Err
}

const jobEventsQuery = `
select
  id,
  job_id,
  level,
  message,
  created_at
from job_events
where job_id = $1
order by created_at desc, id desc
limit $2
`

const createJobEventQuery = `
insert into job_events (
  job_id,
  level,
  message
)
values ($1, $2, $3)
`
