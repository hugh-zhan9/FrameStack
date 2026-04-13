package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"idea/internal/database"
)

func TestDiscoverMigrationFilesReturnsSortedSQLFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "0002_second.sql"), "select 2;")
	mustWriteFile(t, filepath.Join(dir, "0001_first.sql"), "select 1;")
	mustWriteFile(t, filepath.Join(dir, "README.md"), "ignored")

	files, err := database.DiscoverMigrationFiles(dir)
	if err != nil {
		t.Fatalf("expected no error discovering files: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 sql files, got %d", len(files))
	}
	if filepath.Base(files[0].Path) != "0001_first.sql" {
		t.Fatalf("expected first migration to be 0001_first.sql, got %q", filepath.Base(files[0].Path))
	}
	if filepath.Base(files[1].Path) != "0002_second.sql" {
		t.Fatalf("expected second migration to be 0002_second.sql, got %q", filepath.Base(files[1].Path))
	}
}

func TestRunnerExecutesMigrationsInOrder(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "0002_second.sql"), "select 2;")
	mustWriteFile(t, filepath.Join(dir, "0001_first.sql"), "select 1;")

	rec := &recordingExecer{}
	runner := database.Runner{Execer: rec}

	if err := runner.Run(context.Background(), dir); err != nil {
		t.Fatalf("expected runner to succeed: %v", err)
	}

	if len(rec.statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(rec.statements))
	}
	if rec.statements[0] != "select 1;" {
		t.Fatalf("expected first statement to be select 1;, got %q", rec.statements[0])
	}
	if rec.statements[1] != "select 2;" {
		t.Fatalf("expected second statement to be select 2;, got %q", rec.statements[1])
	}
}

type recordingExecer struct {
	statements []string
}

func (r *recordingExecer) ExecContext(_ context.Context, query string, _ ...any) error {
	r.statements = append(r.statements, query)
	return nil
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
