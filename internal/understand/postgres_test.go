package understand_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"idea/internal/understand"
)

func TestPostgresStoreGetFileReturnsFile(t *testing.T) {
	store := understand.PostgresStore{
		Queryer: &recordingRowQueryer{
			row: staticFileRow{
				file: understand.File{
					ID:        7,
					AbsPath:   "/Volumes/media/photos/poster.jpg",
					FileName:  "poster.jpg",
					MediaType: "image",
				},
			},
		},
		Rows: &recordingRowsQueryer{rows: &staticFrameRows{}},
	}

	item, err := store.GetFile(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get file to succeed: %v", err)
	}
	if item.ID != 7 || item.FileName != "poster.jpg" {
		t.Fatalf("unexpected file: %#v", item)
	}
}

func TestPostgresStoreGetFileLoadsUnderstandingFramePaths(t *testing.T) {
	store := understand.PostgresStore{
		Queryer: &recordingRowQueryer{
			row: staticFileRow{
				file: understand.File{
					ID:        9,
					AbsPath:   "/Volumes/media/videos/clip.mp4",
					FileName:  "clip.mp4",
					MediaType: "video",
				},
			},
		},
		Rows: &recordingRowsQueryer{
			rows: &staticFrameRows{
				items: []string{
					"/tmp/previews/9/frame-1.jpg",
					"/tmp/previews/9/frame-2.jpg",
				},
			},
		},
	}

	item, err := store.GetFile(context.Background(), 9)
	if err != nil {
		t.Fatalf("expected get file to succeed: %v", err)
	}
	if len(item.FramePaths) != 2 {
		t.Fatalf("expected 2 frame paths, got %#v", item)
	}
}

func TestPostgresStoreUpsertAnalysisExecutesExpectedSQL(t *testing.T) {
	execer := &recordingExecer{}
	store := understand.PostgresStore{Execer: execer}

	err := store.UpsertAnalysis(context.Background(), understand.AnalysisInput{
		FileID:               7,
		AnalysisType:         "understanding",
		Status:               "succeeded",
		Summary:              "单人室内写真，画面清晰。",
		StructuredAttributes: []byte(`{"orientation":"portrait"}`),
		RawModelOutput:       []byte(`{"raw_tags":["单人写真"]}`),
		Provider:             "placeholder",
		ModelName:            "placeholder-v1",
		PromptVersion:        "understand-v1",
		AnalysisVersion:      1,
	})
	if err != nil {
		t.Fatalf("expected upsert analysis to succeed: %v", err)
	}
	if len(execer.queries) != 1 {
		t.Fatalf("expected one exec query, got %#v", execer.queries)
	}
	normalized := normalizeSQL(execer.queries[0])
	for _, fragment := range []string{"insert into analysis_results", "insert into file_current_analysis"} {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %s", fragment, execer.queries[0])
		}
	}
	if len(execer.args) != 1 || len(execer.args[0]) < 2 || execer.args[0][1] != "understanding" {
		t.Fatalf("expected analysis_type argument understanding, got %#v", execer.args)
	}
}

func TestPostgresStoreReplaceAITagsReplacesExistingTags(t *testing.T) {
	execer := &recordingExecer{}
	store := understand.PostgresStore{Execer: execer}

	err := store.ReplaceAITags(context.Background(), 7, []understand.TagCandidate{
		{Namespace: "content", Name: "单人写真", Confidence: 0.92},
		{Namespace: "management", Name: "待AI精标", Confidence: 0.60},
	})
	if err != nil {
		t.Fatalf("expected replace ai tags to succeed: %v", err)
	}
	if len(execer.queries) != 5 {
		t.Fatalf("expected 5 exec queries, got %#v", execer.queries)
	}
	if !strings.Contains(normalizeSQL(execer.queries[0]), normalizeSQL("delete from file_tags")) {
		t.Fatalf("expected first query to delete ai tags, got %s", execer.queries[0])
	}
	if !strings.Contains(normalizeSQL(execer.queries[1]), normalizeSQL("insert into tags")) {
		t.Fatalf("expected second query to upsert tags, got %s", execer.queries[1])
	}
	if !strings.Contains(normalizeSQL(execer.queries[2]), normalizeSQL("insert into file_tags")) {
		t.Fatalf("expected third query to upsert file_tags, got %s", execer.queries[2])
	}
}

func TestPostgresStorePropagatesExecError(t *testing.T) {
	store := understand.PostgresStore{
		Execer: &recordingExecer{err: errors.New("db down")},
	}

	if err := store.ReplaceAITags(context.Background(), 7, []understand.TagCandidate{{Namespace: "content", Name: "图片"}}); err == nil {
		t.Fatal("expected replace ai tags to fail")
	}
}

type recordingRowQueryer struct {
	row staticFileRow
}

func (q *recordingRowQueryer) QueryRowContext(_ context.Context, _ string, _ ...any) understand.RowScanner {
	return q.row
}

type staticFileRow struct {
	file understand.File
	err  error
}

func (r staticFileRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.file.ID
	*dest[1].(*string) = r.file.AbsPath
	*dest[2].(*string) = r.file.FileName
	*dest[3].(*string) = r.file.MediaType
	return nil
}

type recordingRowsQueryer struct {
	rows *staticFrameRows
}

func (q *recordingRowsQueryer) QueryContext(_ context.Context, _ string, _ ...any) (understand.RowsScanner, error) {
	return q.rows, nil
}

type staticFrameRows struct {
	items []string
	index int
}

func (r *staticFrameRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticFrameRows) Scan(dest ...any) error {
	*dest[0].(*string) = r.items[r.index]
	r.index++
	return nil
}

func (r *staticFrameRows) Err() error   { return nil }
func (r *staticFrameRows) Close() error { return nil }

type recordingExecer struct {
	queries []string
	args    [][]any
	err     error
}

func (e *recordingExecer) ExecContext(_ context.Context, query string, args ...any) error {
	e.queries = append(e.queries, query)
	e.args = append(e.args, args)
	return e.err
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func assignNullString(target any, value string) {
	ptr := target.(*sql.NullString)
	if value == "" {
		*ptr = sql.NullString{}
		return
	}
	*ptr = sql.NullString{String: value, Valid: true}
}
