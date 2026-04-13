package filehash_test

import (
	"context"
	"strings"
	"testing"

	"idea/internal/filehash"
)

func TestPostgresStoreUpdateHashesExecutesExpectedSQL(t *testing.T) {
	execer := &recordingExecer{}
	store := filehash.PostgresStore{Execer: execer}

	if err := store.UpdateHashes(context.Background(), filehash.HashInput{
		FileID:    7,
		SHA256:    "abc",
		QuickHash: "def",
	}); err != nil {
		t.Fatalf("expected update hashes to succeed: %v", err)
	}
	normalized := normalizeSQL(execer.query)
	for _, fragment := range []string{"update files", "set", "sha256", "quick_hash"} {
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
