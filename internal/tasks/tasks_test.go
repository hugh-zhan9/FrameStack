package tasks_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"idea/internal/tasks"
)

func TestSummarizeByStatus(t *testing.T) {
	summary := tasks.Summarize([]tasks.Task{
		{Status: "pending"},
		{Status: "leased"},
		{Status: "running"},
		{Status: "running"},
		{Status: "failed"},
		{Status: "dead"},
		{Status: "succeeded"},
	})

	if summary.Pending != 1 {
		t.Fatalf("expected pending to be 1, got %d", summary.Pending)
	}
	if summary.Running != 3 {
		t.Fatalf("expected running to be 3, got %d", summary.Running)
	}
	if summary.Failed != 1 {
		t.Fatalf("expected failed to be 1, got %d", summary.Failed)
	}
	if summary.Dead != 1 {
		t.Fatalf("expected dead to be 1, got %d", summary.Dead)
	}
	if summary.Succeeded != 1 {
		t.Fatalf("expected succeeded to be 1, got %d", summary.Succeeded)
	}
}

func TestPostgresSummaryProviderReturnsAggregateSummary(t *testing.T) {
	queryer := &recordingSummaryQueryer{
		row: staticSummaryRow{
			values: []int{4, 3, 2, 1, 9},
		},
	}
	provider := tasks.PostgresSummaryProvider{Queryer: queryer}

	summary, err := provider.TaskSummary(context.Background())
	if err != nil {
		t.Fatalf("expected provider to succeed: %v", err)
	}
	if summary.Pending != 4 || summary.Running != 3 || summary.Failed != 2 || summary.Dead != 1 || summary.Succeeded != 9 {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	expected := "select count(*) filter (where status = 'pending') as pending, count(*) filter (where status in ('leased', 'running')) as running, count(*) filter (where status = 'failed') as failed, count(*) filter (where status = 'dead') as dead, count(*) filter (where status = 'succeeded') as succeeded from jobs"
	if normalizeSQL(queryer.query) != normalizeSQL(expected) {
		t.Fatalf("unexpected query:\nwant: %s\n got: %s", expected, queryer.query)
	}
}

func TestPostgresSummaryProviderReturnsScanError(t *testing.T) {
	provider := tasks.PostgresSummaryProvider{
		Queryer: &recordingSummaryQueryer{
			row: staticSummaryRow{err: errors.New("scan failed")},
		},
	}

	_, err := provider.TaskSummary(context.Background())
	if err == nil {
		t.Fatal("expected provider to return scan error")
	}
}

func TestSummaryServiceBuildsSummaryFromStore(t *testing.T) {
	store := tasks.NewInMemoryStore([]tasks.Task{
		{Status: "pending"},
		{Status: "running"},
		{Status: "dead"},
	})

	service := tasks.NewSummaryService(store)

	summary, err := service.TaskSummary(context.Background())
	if err != nil {
		t.Fatalf("expected summary service to succeed: %v", err)
	}
	if summary.Pending != 1 || summary.Running != 1 || summary.Dead != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestSummaryServiceReturnsStoreError(t *testing.T) {
	service := tasks.NewSummaryService(errorStore{err: errors.New("boom")})

	_, err := service.TaskSummary(context.Background())
	if err == nil {
		t.Fatal("expected summary service to return store error")
	}
}

func TestNoopExecutorAcceptsKnownPipelineJobs(t *testing.T) {
	executor := tasks.NoopExecutor{}
	jobTypes := []string{
		"scan_volume",
		"extract_image_features",
		"extract_video_features",
		"recompute_search_doc",
		"embed_image",
		"embed_video_frames",
	}

	for _, jobType := range jobTypes {
		err := executor.ExecuteJob(context.Background(), tasks.Job{JobType: jobType})
		if err != nil {
			t.Fatalf("expected %s to be accepted, got %v", jobType, err)
		}
	}
}

type errorStore struct {
	err error
}

func (e errorStore) ListTasks(_ context.Context) ([]tasks.Task, error) {
	return nil, e.err
}

type recordingSummaryQueryer struct {
	query string
	row   staticSummaryRow
}

func (r *recordingSummaryQueryer) QueryRowContext(_ context.Context, query string, _ ...any) tasks.RowScanner {
	r.query = query
	return r.row
}

type staticSummaryRow struct {
	values []int
	err    error
}

func (r staticSummaryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, target := range dest {
		ptr, ok := target.(*int)
		if !ok {
			return errors.New("expected *int target")
		}
		*ptr = r.values[i]
	}
	return nil
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
