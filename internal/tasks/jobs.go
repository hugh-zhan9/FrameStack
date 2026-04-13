package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrJobNotRetryable = errors.New("job not retryable")

type Job struct {
	ID            int64     `json:"id"`
	JobType       string    `json:"job_type"`
	Status        string    `json:"status"`
	TargetType    string    `json:"target_type,omitempty"`
	TargetID      int64     `json:"target_id,omitempty"`
	ProgressStage string    `json:"progress_stage,omitempty"`
	WorkerName    string    `json:"worker_name,omitempty"`
	Provider      string    `json:"provider,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	AttemptCount  int       `json:"attempt_count"`
	MaxAttempts   int       `json:"max_attempts"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type JobListOptions struct {
	Status string
	Limit  int
}

type JobListProvider interface {
	ListJobs(ctx context.Context, options JobListOptions) ([]Job, error)
}

type CreateJobInput struct {
	JobType     string          `json:"job_type"`
	Priority    int             `json:"priority"`
	TargetType  string          `json:"target_type"`
	TargetID    int64           `json:"target_id"`
	MaxAttempts int             `json:"max_attempts"`
	Payload     json.RawMessage `json:"payload"`
}

type JobCreator interface {
	CreateJob(ctx context.Context, input CreateJobInput) (Job, error)
}

type JobEnsurer interface {
	EnsureJob(ctx context.Context, input CreateJobInput) (Job, error)
}

type JobRetrier interface {
	RetryJob(ctx context.Context, jobID int64) (Job, error)
}

type RowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type JobListQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error)
}

type PostgresJobListProvider struct {
	Queryer JobListQueryer
}

type JobCreateQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
}

type PostgresJobCreator struct {
	Queryer JobCreateQueryer
}

type PostgresJobEnsurer struct {
	Queryer JobCreateQueryer
}

type PostgresJobRetrier struct {
	Queryer JobCreateQueryer
}

func (p PostgresJobListProvider) ListJobs(ctx context.Context, options JobListOptions) ([]Job, error) {
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}

	rows, err := p.Queryer.QueryContext(ctx, jobListQuery, options.Status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Job
	for rows.Next() {
		item, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

type SQLRowsQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func NewPostgresJobListProviderFromDB(db SQLRowsQueryer) PostgresJobListProvider {
	return PostgresJobListProvider{
		Queryer: sqlJobListQueryer{db: db},
	}
}

func (p PostgresJobCreator) CreateJob(ctx context.Context, input CreateJobInput) (Job, error) {
	if input.JobType == "" {
		return Job{}, errors.New("job_type is required")
	}

	priority := input.Priority
	if priority == 0 {
		priority = 100
	}

	maxAttempts := input.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	payload := "{}"
	if len(input.Payload) > 0 {
		payload = string(input.Payload)
	}

	return queryJobRow(
		p.Queryer.QueryRowContext(
			ctx,
			createJobQuery,
			input.JobType,
			priority,
			input.TargetType,
			input.TargetID,
			payload,
			maxAttempts,
		),
	)
}

func NewPostgresJobCreatorFromDB(db SQLDB) PostgresJobCreator {
	return PostgresJobCreator{
		Queryer: sqlJobCreateQueryer{db: db},
	}
}

func (p PostgresJobEnsurer) EnsureJob(ctx context.Context, input CreateJobInput) (Job, error) {
	if input.JobType == "" {
		return Job{}, errors.New("job_type is required")
	}

	priority := input.Priority
	if priority == 0 {
		priority = 100
	}

	maxAttempts := input.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	payload := "{}"
	if len(input.Payload) > 0 {
		payload = string(input.Payload)
	}

	return queryJobRow(
		p.Queryer.QueryRowContext(
			ctx,
			ensureJobQuery,
			input.JobType,
			priority,
			input.TargetType,
			input.TargetID,
			payload,
			maxAttempts,
		),
	)
}

func NewPostgresJobEnsurerFromDB(db SQLDB) PostgresJobEnsurer {
	return PostgresJobEnsurer{
		Queryer: sqlJobCreateQueryer{db: db},
	}
}

func (p PostgresJobRetrier) RetryJob(ctx context.Context, jobID int64) (Job, error) {
	item, err := queryJobRow(p.Queryer.QueryRowContext(ctx, retryJobQuery, jobID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrJobNotRetryable) {
			return Job{}, ErrJobNotRetryable
		}
		return Job{}, err
	}
	return item, nil
}

func NewPostgresJobRetrierFromDB(db SQLDB) PostgresJobRetrier {
	return PostgresJobRetrier{
		Queryer: sqlJobCreateQueryer{db: db},
	}
}

const jobListQuery = `
select
  id,
  job_type,
  status,
  coalesce(target_type, '') as target_type,
  coalesce(target_id, 0) as target_id,
  coalesce(progress_stage, '') as progress_stage,
  coalesce(worker_name, '') as worker_name,
  coalesce(provider, '') as provider,
  coalesce(last_error, '') as last_error,
  attempt_count,
  max_attempts,
  created_at,
  updated_at
from jobs
where ($1 = '' or status = $1)
order by updated_at desc, id desc
limit $2
`

const createJobQuery = `
insert into jobs (
  job_type,
  priority,
  status,
  target_type,
  target_id,
  payload,
  max_attempts
)
values (
  $1,
  $2,
  'pending',
  nullif($3, ''),
  nullif($4, 0),
  $5::jsonb,
  $6
)
returning
  id,
  job_type,
  status,
  coalesce(target_type, '') as target_type,
  coalesce(target_id, 0) as target_id,
  coalesce(progress_stage, '') as progress_stage,
  coalesce(worker_name, '') as worker_name,
  coalesce(provider, '') as provider,
  coalesce(last_error, '') as last_error,
  attempt_count,
  max_attempts,
  created_at,
  updated_at
`

const retryJobQuery = `
update jobs
set
  status = 'pending',
  progress_percent = null,
  progress_stage = null,
  lease_owner = null,
  lease_expires_at = null,
  started_at = null,
  finished_at = null,
  last_error = null,
  worker_name = null,
  provider = null,
  updated_at = now()
where id = $1 and status in ('failed', 'dead')
returning
  id,
  job_type,
  status,
  coalesce(target_type, '') as target_type,
  coalesce(target_id, 0) as target_id,
  coalesce(progress_stage, '') as progress_stage,
  coalesce(worker_name, '') as worker_name,
  coalesce(provider, '') as provider,
  coalesce(last_error, '') as last_error,
  attempt_count,
  max_attempts,
  created_at,
  updated_at
`

const ensureJobQuery = `
with existing as (
  select
    id,
    job_type,
    status,
    coalesce(target_type, '') as target_type,
    coalesce(target_id, 0) as target_id,
    coalesce(progress_stage, '') as progress_stage,
    coalesce(worker_name, '') as worker_name,
    coalesce(provider, '') as provider,
    coalesce(last_error, '') as last_error,
    attempt_count,
    max_attempts,
    created_at,
    updated_at
  from jobs
  where
    job_type = $1
    and coalesce(target_type, '') = coalesce(nullif($3, ''), '')
    and coalesce(target_id, 0) = coalesce(nullif($4, 0), 0)
    and status in ('pending', 'leased', 'running')
  order by id desc
  limit 1
),
inserted as (
  insert into jobs (
    job_type,
    priority,
    status,
    target_type,
    target_id,
    payload,
    max_attempts
  )
  select
    $1,
    $2,
    'pending',
    nullif($3, ''),
    nullif($4, 0),
    $5::jsonb,
    $6
  where not exists (select 1 from existing)
  returning
    id,
    job_type,
    status,
    coalesce(target_type, '') as target_type,
    coalesce(target_id, 0) as target_id,
    coalesce(progress_stage, '') as progress_stage,
    coalesce(worker_name, '') as worker_name,
    coalesce(provider, '') as provider,
    coalesce(last_error, '') as last_error,
    attempt_count,
    max_attempts,
    created_at,
    updated_at
)
select * from inserted
union all
select * from existing
limit 1
`

type sqlJobListQueryer struct {
	db SQLRowsQueryer
}

func (q sqlJobListQueryer) QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error) {
	return q.db.QueryContext(ctx, query, args...)
}

type sqlJobCreateQueryer struct {
	db SQLDB
}

func (q sqlJobCreateQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

func queryJobRow(row RowScanner) (Job, error) {
	return scanJob(row)
}

func scanJob(scanner RowScanner) (Job, error) {
	var item Job
	err := scanner.Scan(
		&item.ID,
		&item.JobType,
		&item.Status,
		&item.TargetType,
		&item.TargetID,
		&item.ProgressStage,
		&item.WorkerName,
		&item.Provider,
		&item.LastError,
		&item.AttemptCount,
		&item.MaxAttempts,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Job{}, err
	}
	return item, nil
}
