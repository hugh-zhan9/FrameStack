package trash_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"idea/internal/trash"
)

func TestPostgresStoreMarkFileTrashedExecutesExpectedSQL(t *testing.T) {
	execer := &recordingExecer{}
	store := trash.PostgresStore{
		Execer: execer,
	}

	if err := store.MarkFileTrashed(context.Background(), 7); err != nil {
		t.Fatalf("expected mark trashed to succeed: %v", err)
	}
	if len(execer.queries) != 1 {
		t.Fatalf("expected one query, got %#v", execer.queries)
	}
	normalized := normalizeSQL(execer.queries[0])
	for _, fragment := range []string{"update files", "status = 'trashed'", "insert into review_actions", "deleted_to_trash"} {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, execer.queries[0])
		}
	}
}

func TestPostgresStorePropagatesExecError(t *testing.T) {
	store := trash.PostgresStore{
		Execer: &recordingExecer{err: errors.New("db down")},
	}

	if err := store.MarkFileTrashed(context.Background(), 7); err == nil {
		t.Fatal("expected mark trashed to fail")
	}
}

type recordingExecer struct {
	queries []string
	err     error
}

func (e *recordingExecer) ExecContext(_ context.Context, query string, _ ...any) error {
	e.queries = append(e.queries, query)
	return e.err
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
