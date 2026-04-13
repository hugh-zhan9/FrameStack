package searchdoc_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"idea/internal/searchdoc"
)

func TestPostgresStoreGetFileSourceReturnsJoinedMetadata(t *testing.T) {
	store := searchdoc.PostgresStore{
		Queryer: &recordingRowQueryer{
			row: staticFileSourceRow{
				source: searchdoc.FileSource{
					FileID:      7,
					AbsPath:     "/Volumes/media/photos/poster.jpg",
					FileName:    "poster.jpg",
					Extension:   ".jpg",
					MediaType:   "image",
					Status:      "active",
					Width:       testIntPtr(320),
					Height:      testIntPtr(180),
					Format:      "jpg",
					Orientation: "landscape",
				},
			},
		},
	}

	item, err := store.GetFileSource(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get source to succeed: %v", err)
	}
	if item.FileName != "poster.jpg" || item.Format != "jpg" {
		t.Fatalf("unexpected file source: %#v", item)
	}
}

func TestPostgresStoreUpsertSearchDocumentExecutesUpsert(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := searchdoc.PostgresStore{Execer: execer}

	err := store.UpsertSearchDocument(context.Background(), searchdoc.DocumentInput{
		FileID:       7,
		DocumentText: "poster jpg landscape",
	})
	if err != nil {
		t.Fatalf("expected upsert to succeed: %v", err)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("insert into search_documents")) {
		t.Fatalf("unexpected query: %s", execer.query)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("to_tsvector('simple'")) {
		t.Fatalf("expected tsvector generation, got %s", execer.query)
	}
}

func TestPostgresStoreUpsertSearchAnalysisExecutesUpsert(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := searchdoc.PostgresStore{Execer: execer}

	err := store.UpsertSearchAnalysis(context.Background(), searchdoc.SearchAnalysisInput{
		FileID:  7,
		Summary: "poster jpg landscape",
	})
	if err != nil {
		t.Fatalf("expected analysis upsert to succeed: %v", err)
	}
	normalized := normalizeSQL(execer.query)
	expectedFragments := []string{
		"insert into analysis_results",
		"analysis_type",
		"'search_doc'",
		"insert into file_current_analysis",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %s", fragment, execer.query)
		}
	}
}

func TestPostgresStorePropagatesExecError(t *testing.T) {
	store := searchdoc.PostgresStore{Execer: &recordingExecQueryer{err: errors.New("db down")}}

	if err := store.UpsertSearchDocument(context.Background(), searchdoc.DocumentInput{FileID: 7}); err == nil {
		t.Fatal("expected upsert to fail")
	}
}

type recordingRowQueryer struct {
	row staticFileSourceRow
}

func (q *recordingRowQueryer) QueryRowContext(_ context.Context, _ string, _ ...any) searchdoc.RowScanner {
	return q.row
}

type staticFileSourceRow struct {
	source searchdoc.FileSource
	err    error
}

func (r staticFileSourceRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	assignInt64(dest[0], r.source.FileID)
	assignString(dest[1], r.source.AbsPath)
	assignString(dest[2], r.source.FileName)
	assignString(dest[3], r.source.Extension)
	assignString(dest[4], r.source.MediaType)
	assignString(dest[5], r.source.Status)
	assignNullInt64(dest[6], r.source.Width)
	assignNullInt64(dest[7], r.source.Height)
	assignNullInt64(dest[8], r.source.DurationMS)
	assignNullString(dest[9], r.source.Format)
	assignNullString(dest[10], r.source.Container)
	assignNullString(dest[11], r.source.VideoCodec)
	assignNullString(dest[12], r.source.AudioCodec)
	assignNullString(dest[13], r.source.Orientation)
	return nil
}

type recordingExecQueryer struct {
	query string
	args  []any
	err   error
}

func (q *recordingExecQueryer) ExecContext(_ context.Context, query string, args ...any) error {
	q.query = query
	q.args = args
	return q.err
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func testIntPtr(v int) *int { return &v }

func assignInt64(target any, value int64) {
	*target.(*int64) = value
}

func assignString(target any, value string) {
	*target.(*string) = value
}

func assignNullInt64(target any, value any) {
	switch ptr := target.(type) {
	case *sql.NullInt64:
		if typed, ok := value.(*int); ok && typed != nil {
			*ptr = sql.NullInt64{Int64: int64(*typed), Valid: true}
			return
		}
		if typed, ok := value.(*int64); ok && typed != nil {
			*ptr = sql.NullInt64{Int64: *typed, Valid: true}
			return
		}
		*ptr = sql.NullInt64{}
	}
}

func assignNullString(target any, value string) {
	ptr := target.(*sql.NullString)
	if value == "" {
		*ptr = sql.NullString{}
		return
	}
	*ptr = sql.NullString{String: value, Valid: true}
}
