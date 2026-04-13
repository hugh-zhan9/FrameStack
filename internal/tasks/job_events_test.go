package tasks_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"idea/internal/tasks"
)

func TestPostgresJobEventListProviderReturnsEvents(t *testing.T) {
	now := time.Date(2026, 4, 9, 15, 0, 0, 0, time.UTC)
	queryer := &recordingEventListQueryer{
		rows: &staticEventRows{
			items: []eventRow{
				{ID: 1, JobID: 7, Level: "info", Message: "job created", CreatedAt: now},
			},
		},
	}
	provider := tasks.PostgresJobEventListProvider{Queryer: queryer}

	items, err := provider.ListJobEvents(context.Background(), 7, 10)
	if err != nil {
		t.Fatalf("expected provider to succeed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 event, got %d", len(items))
	}
	if items[0].Message != "job created" || items[0].JobID != 7 {
		t.Fatalf("unexpected event: %#v", items[0])
	}
	expectedFragments := []string{
		"from job_events",
		"where job_id = $1",
		"order by created_at desc, id desc",
		"limit $2",
	}
	normalized := normalizeSQL(queryer.query)
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresJobEventRecorderInsertsEvent(t *testing.T) {
	execer := &recordingEventExecer{}
	recorder := tasks.PostgresJobEventRecorder{Execer: execer}

	err := recorder.RecordJobEvent(context.Background(), tasks.CreateJobEventInput{
		JobID:   7,
		Level:   "info",
		Message: "job created",
	})
	if err != nil {
		t.Fatalf("expected recorder to succeed: %v", err)
	}
	if len(execer.args) != 3 || execer.args[0] != int64(7) || execer.args[1] != "info" || execer.args[2] != "job created" {
		t.Fatalf("unexpected recorder args: %#v", execer.args)
	}
}

func TestEventRecordingJobCreatorCreatesJobAndEvent(t *testing.T) {
	creator := tasks.EventRecordingJobCreator{
		Creator: tasks.StaticJobCreator{
			Item: tasks.Job{ID: 8, JobType: "scan_volume", Status: "pending"},
		},
		Recorder: tasks.StaticJobEventRecorder{},
	}

	item, err := creator.CreateJob(context.Background(), tasks.CreateJobInput{JobType: "scan_volume"})
	if err != nil {
		t.Fatalf("expected creator to succeed: %v", err)
	}
	if item.ID != 8 {
		t.Fatalf("unexpected job: %#v", item)
	}
}

func TestEventRecordingJobRetrierRetriesJobAndEvent(t *testing.T) {
	retrier := tasks.EventRecordingJobRetrier{
		Retrier: tasks.StaticJobRetrier{
			Item: tasks.Job{ID: 9, JobType: "infer_tags", Status: "pending"},
		},
		Recorder: tasks.StaticJobEventRecorder{},
	}

	item, err := retrier.RetryJob(context.Background(), 9)
	if err != nil {
		t.Fatalf("expected retrier to succeed: %v", err)
	}
	if item.ID != 9 {
		t.Fatalf("unexpected job: %#v", item)
	}
}

func TestEventRecordingJobCreatorReturnsRecorderError(t *testing.T) {
	creator := tasks.EventRecordingJobCreator{
		Creator: tasks.StaticJobCreator{
			Item: tasks.Job{ID: 8, JobType: "scan_volume", Status: "pending"},
		},
		Recorder: tasks.StaticJobEventRecorder{Err: errors.New("write failed")},
	}

	_, err := creator.CreateJob(context.Background(), tasks.CreateJobInput{JobType: "scan_volume"})
	if err == nil {
		t.Fatal("expected creator to return recorder error")
	}
}

type eventRow struct {
	ID        int64
	JobID     int64
	Level     string
	Message   string
	CreatedAt time.Time
}

type recordingEventListQueryer struct {
	query string
	rows  tasks.RowsScanner
}

func (r *recordingEventListQueryer) QueryContext(_ context.Context, query string, _ ...any) (tasks.RowsScanner, error) {
	r.query = query
	return r.rows, nil
}

type staticEventRows struct {
	items []eventRow
	index int
}

func (r *staticEventRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticEventRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	assignInt64(dest[0], item.ID)
	assignInt64(dest[1], item.JobID)
	assignString(dest[2], item.Level)
	assignString(dest[3], item.Message)
	assignTime(dest[4], item.CreatedAt)
	return nil
}

func (r *staticEventRows) Err() error   { return nil }
func (r *staticEventRows) Close() error { return nil }

type recordingEventExecer struct {
	query string
	args  []any
}

func (r *recordingEventExecer) ExecContext(_ context.Context, query string, args ...any) error {
	r.query = query
	r.args = args
	return nil
}
