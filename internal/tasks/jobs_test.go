package tasks_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"idea/internal/tasks"
)

func TestPostgresJobListProviderReturnsJobs(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	queryer := &recordingJobListQueryer{
		rows: &staticJobRows{
			items: []jobRow{
				{
					ID:            7,
					JobType:       "infer_tags",
					Status:        "running",
					TargetType:    "file",
					TargetID:      101,
					ProgressStage: "infer",
					WorkerName:    "worker-1",
					Provider:      "ollama",
					LastError:     "",
					AttemptCount:  1,
					MaxAttempts:   3,
					CreatedAt:     now,
					UpdatedAt:     now,
				},
			},
		},
	}
	provider := tasks.PostgresJobListProvider{Queryer: queryer}

	items, err := provider.ListJobs(context.Background(), tasks.JobListOptions{
		Status: "running",
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("expected provider to succeed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].JobType != "infer_tags" || items[0].Status != "running" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
	if items[0].LastError != "" {
		t.Fatalf("expected empty last error, got %q", items[0].LastError)
	}
	if len(queryer.args) != 2 || queryer.args[0] != "running" || queryer.args[1] != 20 {
		t.Fatalf("unexpected query args: %#v", queryer.args)
	}
	expectedFragments := []string{
		"from jobs",
		"where ($1 = '' or status = $1)",
		"order by updated_at desc, id desc",
		"limit $2",
	}
	normalized := normalizeSQL(queryer.query)
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresJobListProviderReturnsRowsError(t *testing.T) {
	provider := tasks.PostgresJobListProvider{
		Queryer: &recordingJobListQueryer{
			err: errors.New("query failed"),
		},
	}

	_, err := provider.ListJobs(context.Background(), tasks.JobListOptions{})
	if err == nil {
		t.Fatal("expected provider to return query error")
	}
}

func TestPostgresJobListProviderReturnsScanError(t *testing.T) {
	provider := tasks.PostgresJobListProvider{
		Queryer: &recordingJobListQueryer{
			rows: &staticJobRows{
				items:   []jobRow{{}},
				scanErr: errors.New("scan failed"),
			},
		},
	}

	_, err := provider.ListJobs(context.Background(), tasks.JobListOptions{})
	if err == nil {
		t.Fatal("expected provider to return scan error")
	}
}

func TestPostgresJobCreatorCreatesPendingJob(t *testing.T) {
	now := time.Date(2026, 4, 9, 13, 0, 0, 0, time.UTC)
	queryer := &recordingJobCreateQueryer{
		row: staticJobCreateRow{
			item: jobRow{
				ID:           11,
				JobType:      "scan_volume",
				Status:       "pending",
				TargetType:   "volume",
				TargetID:     4,
				AttemptCount: 0,
				MaxAttempts:  5,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
	creator := tasks.PostgresJobCreator{Queryer: queryer}

	payload := json.RawMessage(`{"volume_id":4}`)
	item, err := creator.CreateJob(context.Background(), tasks.CreateJobInput{
		JobType:     "scan_volume",
		Priority:    90,
		TargetType:  "volume",
		TargetID:    4,
		MaxAttempts: 5,
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("expected creator to succeed: %v", err)
	}
	if item.ID != 11 || item.JobType != "scan_volume" || item.Status != "pending" {
		t.Fatalf("unexpected created job: %#v", item)
	}
	expectedArgs := []any{"scan_volume", 90, "volume", int64(4), `{"volume_id":4}`, 5}
	for i, expected := range expectedArgs {
		if queryer.args[i] != expected {
			t.Fatalf("unexpected arg %d: want %#v got %#v", i, expected, queryer.args[i])
		}
	}
}

func TestPostgresJobCreatorRejectsMissingJobType(t *testing.T) {
	creator := tasks.PostgresJobCreator{}

	_, err := creator.CreateJob(context.Background(), tasks.CreateJobInput{})
	if err == nil {
		t.Fatal("expected creator to reject missing job type")
	}
}

func TestPostgresJobCreatorReturnsScanError(t *testing.T) {
	creator := tasks.PostgresJobCreator{
		Queryer: &recordingJobCreateQueryer{
			row: staticJobCreateRow{err: errors.New("scan failed")},
		},
	}

	_, err := creator.CreateJob(context.Background(), tasks.CreateJobInput{
		JobType: "scan_volume",
	})
	if err == nil {
		t.Fatal("expected creator to return scan error")
	}
}

func TestPostgresJobEnsurerEnsuresPendingJob(t *testing.T) {
	now := time.Date(2026, 4, 9, 13, 30, 0, 0, time.UTC)
	queryer := &recordingJobCreateQueryer{
		row: staticJobCreateRow{
			item: jobRow{
				ID:           17,
				JobType:      "extract_image_features",
				Status:       "pending",
				TargetType:   "file",
				TargetID:     42,
				AttemptCount: 0,
				MaxAttempts:  3,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
	ensurer := tasks.PostgresJobEnsurer{Queryer: queryer}

	item, err := ensurer.EnsureJob(context.Background(), tasks.CreateJobInput{
		JobType:    "extract_image_features",
		Priority:   80,
		TargetType: "file",
		TargetID:   42,
		Payload:    json.RawMessage(`{"file_id":42}`),
	})
	if err != nil {
		t.Fatalf("expected ensurer to succeed: %v", err)
	}
	if item.ID != 17 || item.JobType != "extract_image_features" || item.TargetID != 42 {
		t.Fatalf("unexpected ensured job: %#v", item)
	}
	expectedFragments := []string{
		"with existing as",
		"status in ('pending', 'leased', 'running')",
		"insert into jobs",
	}
	normalized := normalizeSQL(queryer.query)
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresJobRetrierRequeuesJob(t *testing.T) {
	now := time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC)
	queryer := &recordingJobCreateQueryer{
		row: staticJobCreateRow{
			item: jobRow{
				ID:           21,
				JobType:      "infer_tags",
				Status:       "pending",
				TargetType:   "file",
				TargetID:     1001,
				AttemptCount: 2,
				MaxAttempts:  3,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
	}
	retrier := tasks.PostgresJobRetrier{Queryer: queryer}

	item, err := retrier.RetryJob(context.Background(), 21)
	if err != nil {
		t.Fatalf("expected retrier to succeed: %v", err)
	}
	if item.ID != 21 || item.Status != "pending" {
		t.Fatalf("unexpected retried job: %#v", item)
	}
	if len(queryer.args) != 1 || queryer.args[0] != int64(21) {
		t.Fatalf("unexpected retry args: %#v", queryer.args)
	}
}

func TestPostgresJobRetrierReturnsNotRetryableError(t *testing.T) {
	retrier := tasks.PostgresJobRetrier{
		Queryer: &recordingJobCreateQueryer{
			row: staticJobCreateRow{err: tasks.ErrJobNotRetryable},
		},
	}

	_, err := retrier.RetryJob(context.Background(), 99)
	if !errors.Is(err, tasks.ErrJobNotRetryable) {
		t.Fatalf("expected ErrJobNotRetryable, got %v", err)
	}
}

type recordingJobListQueryer struct {
	query string
	args  []any
	rows  tasks.RowsScanner
	err   error
}

func (r *recordingJobListQueryer) QueryContext(_ context.Context, query string, args ...any) (tasks.RowsScanner, error) {
	r.query = query
	r.args = args
	if r.err != nil {
		return nil, r.err
	}
	return r.rows, nil
}

type recordingJobCreateQueryer struct {
	query string
	args  []any
	row   staticJobCreateRow
}

func (r *recordingJobCreateQueryer) QueryRowContext(_ context.Context, query string, args ...any) tasks.RowScanner {
	r.query = query
	r.args = args
	return r.row
}

type jobRow struct {
	ID            int64
	JobType       string
	Status        string
	TargetType    string
	TargetID      int64
	ProgressStage string
	WorkerName    string
	Provider      string
	LastError     string
	AttemptCount  int
	MaxAttempts   int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type staticJobRows struct {
	items   []jobRow
	index   int
	scanErr error
}

type staticJobCreateRow struct {
	item jobRow
	err  error
}

func (r staticJobCreateRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	assignInt64(dest[0], r.item.ID)
	assignString(dest[1], r.item.JobType)
	assignString(dest[2], r.item.Status)
	assignNullString(dest[3], r.item.TargetType)
	assignNullInt64(dest[4], r.item.TargetID)
	assignNullString(dest[5], r.item.ProgressStage)
	assignNullString(dest[6], r.item.WorkerName)
	assignNullString(dest[7], r.item.Provider)
	assignNullString(dest[8], r.item.LastError)
	assignInt(dest[9], r.item.AttemptCount)
	assignInt(dest[10], r.item.MaxAttempts)
	assignTime(dest[11], r.item.CreatedAt)
	assignTime(dest[12], r.item.UpdatedAt)
	return nil
}

func (r *staticJobRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticJobRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	item := r.items[r.index]
	r.index++

	assignInt64(dest[0], item.ID)
	assignString(dest[1], item.JobType)
	assignString(dest[2], item.Status)
	assignNullString(dest[3], item.TargetType)
	assignNullInt64(dest[4], item.TargetID)
	assignNullString(dest[5], item.ProgressStage)
	assignNullString(dest[6], item.WorkerName)
	assignNullString(dest[7], item.Provider)
	assignNullString(dest[8], item.LastError)
	assignInt(dest[9], item.AttemptCount)
	assignInt(dest[10], item.MaxAttempts)
	assignTime(dest[11], item.CreatedAt)
	assignTime(dest[12], item.UpdatedAt)
	return nil
}

func (r *staticJobRows) Err() error {
	return nil
}

func (r *staticJobRows) Close() error {
	return nil
}

func assignInt64(target any, value int64) {
	ptr := target.(*int64)
	*ptr = value
}

func assignInt(target any, value int) {
	ptr := target.(*int)
	*ptr = value
}

func assignString(target any, value string) {
	ptr := target.(*string)
	*ptr = value
}

func assignTime(target any, value time.Time) {
	ptr := target.(*time.Time)
	*ptr = value
}

func assignNullString(target any, value string) {
	ptr := target.(*string)
	*ptr = value
}

func assignNullInt64(target any, value int64) {
	ptr := target.(*int64)
	*ptr = value
}
