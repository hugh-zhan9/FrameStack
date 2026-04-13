package tasks

import (
	"context"
	"database/sql"
	"fmt"
)

type PostgresQueueStore struct {
	Queryer JobCreateQueryer
}

func NewPostgresQueueStoreFromDB(db SQLDB) PostgresQueueStore {
	return PostgresQueueStore{
		Queryer: sqlJobCreateQueryer{db: db},
	}
}

func (s PostgresQueueStore) LeaseNextJob(ctx context.Context, workerName string) (Job, bool, error) {
	item, err := s.queryJob(ctx, leaseNextJobQuery, workerName)
	if err != nil {
		if err == sql.ErrNoRows {
			return Job{}, false, nil
		}
		return Job{}, false, err
	}
	return item, true, nil
}

func (s PostgresQueueStore) MarkJobRunning(ctx context.Context, jobID int64, workerName string, stage string) (Job, error) {
	return s.queryJob(ctx, markJobRunningQuery, jobID, workerName, stage)
}

func (s PostgresQueueStore) MarkJobSucceeded(ctx context.Context, jobID int64) (Job, error) {
	return s.queryJob(ctx, markJobSucceededQuery, jobID)
}

func (s PostgresQueueStore) MarkJobFailed(ctx context.Context, jobID int64, message string) (Job, error) {
	return s.queryJob(ctx, markJobFailedQuery, jobID, message)
}

func (s PostgresQueueStore) queryJob(ctx context.Context, query string, args ...any) (Job, error) {
	return queryJobRow(s.Queryer.QueryRowContext(ctx, query, args...))
}

type NoopExecutor struct{}

func (NoopExecutor) ExecuteJob(_ context.Context, job Job) error {
	switch job.JobType {
	case "scan_volume", "extract_image_features", "extract_video_features", "recompute_search_doc", "infer_tags", "infer_quality", "hash_file", "cluster_same_content", "cluster_same_series", "cluster_same_person", "embed_image", "embed_video_frames", "embed_person_image", "embed_person_video_frames":
		return nil
	default:
		return fmt.Errorf("no handler registered for job type %s", job.JobType)
	}
}

const leaseNextJobQuery = `
with candidate as (
  select id
  from jobs
  where
    (
      status = 'pending'
      and scheduled_at <= now()
    )
    or (
      status = 'leased'
      and lease_expires_at is not null
      and lease_expires_at <= now()
    )
  order by priority asc, scheduled_at asc, id asc
  limit 1
)
update jobs
set
  status = 'leased',
  lease_owner = $1,
  lease_expires_at = now() + interval '10 minutes',
  attempt_count = attempt_count + 1,
  updated_at = now()
where id in (select id from candidate)
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

const markJobRunningQuery = `
update jobs
set
  status = 'running',
  progress_stage = nullif($3, ''),
  worker_name = nullif($2, ''),
  started_at = coalesce(started_at, now()),
  updated_at = now()
where id = $1
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

const markJobSucceededQuery = `
update jobs
set
  status = 'succeeded',
  progress_stage = null,
  progress_percent = 100,
  finished_at = now(),
  updated_at = now()
where id = $1
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

const markJobFailedQuery = `
update jobs
set
  status = 'failed',
  last_error = $2,
  finished_at = now(),
  updated_at = now()
where id = $1
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
