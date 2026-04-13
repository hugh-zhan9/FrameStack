package tasks

import (
	"context"
	"database/sql"
)

type Task struct {
	Status string
}

type Summary struct {
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Failed    int `json:"failed"`
	Dead      int `json:"dead"`
	Succeeded int `json:"succeeded"`
}

func Summarize(items []Task) Summary {
	var summary Summary
	for _, item := range items {
		switch item.Status {
		case "pending":
			summary.Pending++
		case "leased", "running":
			summary.Running++
		case "failed":
			summary.Failed++
		case "dead":
			summary.Dead++
		case "succeeded":
			summary.Succeeded++
		}
	}
	return summary
}

type Store interface {
	ListTasks(ctx context.Context) ([]Task, error)
}

type SummaryProvider interface {
	TaskSummary(ctx context.Context) (Summary, error)
}

type RowScanner interface {
	Scan(dest ...any) error
}

type SummaryQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
}

type SummaryService struct {
	store Store
}

func NewSummaryService(store Store) SummaryService {
	return SummaryService{store: store}
}

func (s SummaryService) TaskSummary(ctx context.Context) (Summary, error) {
	items, err := s.store.ListTasks(ctx)
	if err != nil {
		return Summary{}, err
	}
	return Summarize(items), nil
}

type PostgresSummaryProvider struct {
	Queryer SummaryQueryer
}

type SQLDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (p PostgresSummaryProvider) TaskSummary(ctx context.Context) (Summary, error) {
	var summary Summary
	err := p.Queryer.QueryRowContext(ctx, taskSummaryQuery).Scan(
		&summary.Pending,
		&summary.Running,
		&summary.Failed,
		&summary.Dead,
		&summary.Succeeded,
	)
	if err != nil {
		return Summary{}, err
	}
	return summary, nil
}

func NewPostgresSummaryProviderFromDB(db SQLDB) PostgresSummaryProvider {
	return PostgresSummaryProvider{
		Queryer: sqlSummaryQueryer{db: db},
	}
}

type InMemoryStore struct {
	tasks []Task
}

func NewInMemoryStore(items []Task) InMemoryStore {
	cloned := make([]Task, len(items))
	copy(cloned, items)
	return InMemoryStore{tasks: cloned}
}

func (s InMemoryStore) ListTasks(_ context.Context) ([]Task, error) {
	cloned := make([]Task, len(s.tasks))
	copy(cloned, s.tasks)
	return cloned, nil
}

const taskSummaryQuery = `
select
  count(*) filter (where status = 'pending') as pending,
  count(*) filter (where status in ('leased', 'running')) as running,
  count(*) filter (where status = 'failed') as failed,
  count(*) filter (where status = 'dead') as dead,
  count(*) filter (where status = 'succeeded') as succeeded
from jobs
`

type sqlSummaryQueryer struct {
	db SQLDB
}

func (q sqlSummaryQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}
