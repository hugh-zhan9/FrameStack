package mediaextract_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"idea/internal/mediaextract"
)

func TestPostgresStoreGetFileReturnsFile(t *testing.T) {
	store := mediaextract.PostgresStore{
		Queryer: &recordingRowQueryer{
			row: staticFileRow{
				file: mediaextract.File{
					ID:        7,
					AbsPath:   "/Volumes/media/poster.png",
					Extension: ".png",
					MediaType: "image",
				},
			},
		},
	}

	item, err := store.GetFile(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get file to succeed: %v", err)
	}
	if item.AbsPath != "/Volumes/media/poster.png" || item.MediaType != "image" {
		t.Fatalf("unexpected file: %#v", item)
	}
}

func TestPostgresStoreUpsertImageAssetExecutesUpsert(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := mediaextract.PostgresStore{Execer: execer}
	width := 320
	height := 180

	err := store.UpsertImageAsset(context.Background(), mediaextract.ImageAssetInput{
		FileID:      7,
		Width:       &width,
		Height:      &height,
		Format:      "png",
		Orientation: "landscape",
		PHash:       "abc123",
		ThumbnailPath: "/tmp/thumbs/7.jpg",
	})
	if err != nil {
		t.Fatalf("expected upsert to succeed: %v", err)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("insert into image_assets")) {
		t.Fatalf("unexpected query: %s", execer.query)
	}
	if len(execer.args) != 7 || execer.args[5] != "abc123" || execer.args[6] != "/tmp/thumbs/7.jpg" {
		t.Fatalf("expected image phash arg to be persisted, got %#v", execer.args)
	}
}

func TestPostgresStoreUpsertVideoAssetExecutesUpsert(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := mediaextract.PostgresStore{Execer: execer}
	duration := int64(95_000)

	err := store.UpsertVideoAsset(context.Background(), mediaextract.VideoAssetInput{
		FileID:     9,
		DurationMS: &duration,
		Container:  "mp4",
		PosterPath: "/tmp/previews/9/poster.jpg",
	})
	if err != nil {
		t.Fatalf("expected upsert to succeed: %v", err)
	}
	if !strings.Contains(normalizeSQL(execer.query), normalizeSQL("insert into video_assets")) {
		t.Fatalf("unexpected query: %s", execer.query)
	}
}

func TestPostgresStoreReplaceVideoFramesExecutesDeleteAndInsert(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := mediaextract.PostgresStore{Execer: execer}

	err := store.ReplaceVideoFrames(context.Background(), 9, []mediaextract.VideoFrameInput{
		{TimestampMS: 5_000, FramePath: "/tmp/previews/9/frame-1.jpg", FrameRole: "understanding", PHash: "p1"},
		{TimestampMS: 45_000, FramePath: "/tmp/previews/9/frame-2.jpg", FrameRole: "understanding", PHash: "p2"},
	})
	if err != nil {
		t.Fatalf("expected replace frames to succeed: %v", err)
	}
	if len(execer.queries) != 3 {
		t.Fatalf("expected 3 exec queries, got %#v", execer.queries)
	}
	if !strings.Contains(normalizeSQL(execer.queries[0]), normalizeSQL("delete from video_frames")) {
		t.Fatalf("unexpected delete query: %s", execer.queries[0])
	}
	if !strings.Contains(normalizeSQL(execer.queries[1]), normalizeSQL("insert into video_frames")) {
		t.Fatalf("unexpected insert query: %s", execer.queries[1])
	}
	if len(execer.allArgs[1]) != 5 || execer.allArgs[1][4] != "p1" {
		t.Fatalf("expected frame phash arg to be persisted, got %#v", execer.allArgs[1])
	}
}

func TestPostgresStorePropagatesExecError(t *testing.T) {
	store := mediaextract.PostgresStore{
		Execer: &recordingExecQueryer{err: errors.New("db down")},
	}

	err := store.UpsertImageAsset(context.Background(), mediaextract.ImageAssetInput{FileID: 7})
	if err == nil {
		t.Fatal("expected upsert to fail")
	}
}

type recordingRowQueryer struct {
	row staticFileRow
}

func (q *recordingRowQueryer) QueryRowContext(_ context.Context, _ string, _ ...any) mediaextract.RowScanner {
	return q.row
}

type staticFileRow struct {
	file mediaextract.File
	err  error
}

func (r staticFileRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.file.ID
	*dest[1].(*string) = r.file.AbsPath
	*dest[2].(*string) = r.file.Extension
	*dest[3].(*string) = r.file.MediaType
	return nil
}

type recordingExecQueryer struct {
	query   string
	args    []any
	queries []string
	allArgs [][]any
	err     error
}

func (q *recordingExecQueryer) ExecContext(_ context.Context, query string, args ...any) error {
	q.query = query
	q.args = args
	q.queries = append(q.queries, query)
	q.allArgs = append(q.allArgs, args)
	return q.err
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
