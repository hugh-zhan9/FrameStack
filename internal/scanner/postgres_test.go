package scanner_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"idea/internal/scanner"
)

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func TestPostgresStoreGetVolumeReturnsVolume(t *testing.T) {
	queryer := &recordingRowQueryer{
		row: staticVolumeRow{
			volume: scanner.Volume{ID: 7, MountPath: "/Volumes/media"},
		},
	}
	store := scanner.PostgresStore{Queryer: queryer}

	volume, err := store.GetVolume(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get volume to succeed: %v", err)
	}
	if volume.MountPath != "/Volumes/media" {
		t.Fatalf("unexpected volume: %#v", volume)
	}
	if len(queryer.args) != 1 || queryer.args[0] != int64(7) {
		t.Fatalf("unexpected query args: %#v", queryer.args)
	}
}

func TestPostgresStoreTouchVolumeExecutesUpdate(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := scanner.PostgresStore{Execer: execer}

	if err := store.TouchVolume(context.Background(), 7); err != nil {
		t.Fatalf("expected touch volume to succeed: %v", err)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("update volumes")) {
		t.Fatalf("unexpected query: %s", execer.query)
	}
}

func TestPostgresStoreUpsertFileExecutesInsertOrUpdate(t *testing.T) {
	queryer := &recordingUpsertRowQueryer{
		row: staticUpsertRow{
			result: scanner.UpsertResult{FileID: 22, Changed: true},
		},
	}
	store := scanner.PostgresStore{Queryer: queryer}

	record := scanner.FileRecord{
		VolumeID:   7,
		AbsPath:    "/Volumes/media/photo.jpg",
		ParentPath: "/Volumes/media",
		FileName:   "photo.jpg",
		Extension:  ".jpg",
		MediaType:  "image",
		SizeBytes:  128,
		ModTime:    time.Date(2026, 4, 9, 19, 0, 0, 0, time.UTC),
	}
	result, err := store.UpsertFile(context.Background(), record)
	if err != nil {
		t.Fatalf("expected upsert to succeed: %v", err)
	}
	if result.FileID != 22 || !result.Changed {
		t.Fatalf("unexpected upsert result: %#v", result)
	}
	if !strings.Contains(normalizeSQL(queryer.query), normalizeSQL("insert into files")) {
		t.Fatalf("unexpected query: %s", queryer.query)
	}
	if !strings.Contains(normalizeSQL(queryer.query), normalizeSQL("insert into file_path_history")) {
		t.Fatalf("expected file path history insert, got %s", queryer.query)
	}
}

func TestPostgresStoreMarkMissingFilesExecutesUpdate(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := scanner.PostgresStore{Execer: execer}

	err := store.MarkMissingFiles(context.Background(), 7, []string{
		"/Volumes/media/photo.jpg",
		"/Volumes/media/clip.mp4",
	})
	if err != nil {
		t.Fatalf("expected mark missing to succeed: %v", err)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("update files")) {
		t.Fatalf("unexpected query: %s", execer.query)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("insert into file_path_history")) {
		t.Fatalf("expected file path history insert, got %s", execer.query)
	}
}

func TestPostgresStorePropagatesExecError(t *testing.T) {
	store := scanner.PostgresStore{
		Execer: &recordingExecQueryer{err: errors.New("db down")},
	}

	err := store.TouchVolume(context.Background(), 7)
	if err == nil {
		t.Fatal("expected touch volume to fail")
	}
}

type recordingRowQueryer struct {
	query string
	args  []any
	row   staticVolumeRow
}

func (r *recordingRowQueryer) QueryRowContext(_ context.Context, query string, args ...any) scanner.RowScanner {
	r.query = query
	r.args = args
	return r.row
}

type staticVolumeRow struct {
	volume scanner.Volume
	err    error
}

func (r staticVolumeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.volume.ID
	*dest[1].(*string) = r.volume.MountPath
	return nil
}

type recordingExecQueryer struct {
	query string
	args  []any
	err   error
}

func (r *recordingExecQueryer) ExecContext(_ context.Context, query string, args ...any) error {
	r.query = query
	r.args = args
	return r.err
}

type recordingUpsertRowQueryer struct {
	query string
	args  []any
	row   staticUpsertRow
}

func (r *recordingUpsertRowQueryer) QueryRowContext(_ context.Context, query string, args ...any) scanner.RowScanner {
	r.query = query
	r.args = args
	return r.row
}

type staticUpsertRow struct {
	result scanner.UpsertResult
	err    error
}

func (r staticUpsertRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.result.FileID
	*dest[1].(*bool) = r.result.Changed
	return nil
}
