package review_test

import (
	"context"
	"strings"
	"testing"

	"idea/internal/review"
)

func TestPostgresStoreCreateFileActionExecutesExpectedSQL(t *testing.T) {
	execer := &recordingExecer{}
	store := review.PostgresStore{Execer: execer}

	if err := store.CreateFileAction(context.Background(), 7, review.FileActionInput{
		ActionType: "favorite",
		Note:       "manual favorite",
	}); err != nil {
		t.Fatalf("expected create review action to succeed: %v", err)
	}
	normalized := normalizeSQL(execer.query)
	for _, fragment := range []string{"insert into review_actions", "file_id", "action_type", "note"} {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, execer.query)
		}
	}
}

type recordingExecer struct {
	query string
}

func (e *recordingExecer) ExecContext(_ context.Context, query string, _ ...any) error {
	e.query = query
	return nil
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
