package embeddings_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"idea/internal/embeddings"
)

func TestPostgresStoreUpsertFileEmbeddingExecutesUpsert(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := embeddings.PostgresStore{Execer: execer}

	err := store.UpsertFileEmbedding(context.Background(), embeddings.FileEmbeddingInput{
		FileID:        7,
		EmbeddingType: "image_visual",
		ModelName:     "phash-v1",
		Vector:        "[0.0,1.0]",
	})
	if err != nil {
		t.Fatalf("expected upsert to succeed: %v", err)
	}
	if len(execer.queries) != 2 {
		t.Fatalf("expected delete then insert, got %#v", execer.queries)
	}
	if !strings.Contains(normalizeSQL(execer.queries[0]), normalizeSQL("delete from embeddings")) {
		t.Fatalf("unexpected delete query: %s", execer.queries[0])
	}
	if !strings.Contains(normalizeSQL(execer.queries[1]), normalizeSQL("insert into embeddings")) {
		t.Fatalf("unexpected insert query: %s", execer.queries[1])
	}
	if len(execer.allArgs[0]) != 3 || execer.allArgs[0][0] != int64(7) || execer.allArgs[0][1] != "image_visual" || execer.allArgs[0][2] != "phash-v1" {
		t.Fatalf("unexpected delete args: %#v", execer.allArgs[0])
	}
	if len(execer.allArgs[1]) != 4 || execer.allArgs[1][3] != "[0.0,1.0]" {
		t.Fatalf("unexpected insert args: %#v", execer.allArgs[1])
	}
}

func TestPostgresStoreReplaceFrameEmbeddingsDeletesThenInserts(t *testing.T) {
	execer := &recordingExecQueryer{}
	store := embeddings.PostgresStore{Execer: execer}

	err := store.ReplaceFrameEmbeddings(context.Background(), 9, "video_frame_visual", []embeddings.FrameEmbeddingInput{
		{FrameID: 11, EmbeddingType: "video_frame_visual", ModelName: "phash-v1", Vector: "[0.1,0.2]"},
		{FrameID: 12, EmbeddingType: "video_frame_visual", ModelName: "phash-v1", Vector: "[0.3,0.4]"},
	})
	if err != nil {
		t.Fatalf("expected replace to succeed: %v", err)
	}
	if len(execer.queries) != 3 {
		t.Fatalf("expected 3 exec queries, got %#v", execer.queries)
	}
	if !strings.Contains(normalizeSQL(execer.queries[0]), normalizeSQL("delete from embeddings")) {
		t.Fatalf("unexpected delete query: %s", execer.queries[0])
	}
	if execer.allArgs[0][0] != int64(9) {
		t.Fatalf("unexpected delete args: %#v", execer.allArgs[0])
	}
	if len(execer.allArgs[0]) != 2 || execer.allArgs[0][1] != "video_frame_visual" {
		t.Fatalf("expected delete to target all frame embeddings for the type, got %#v", execer.allArgs[0])
	}
	if execer.allArgs[1][0] != int64(11) || execer.allArgs[2][0] != int64(12) {
		t.Fatalf("unexpected insert args: %#v", execer.allArgs)
	}
}

func TestPostgresStorePropagatesExecError(t *testing.T) {
	store := embeddings.PostgresStore{Execer: &recordingExecQueryer{err: errors.New("db down")}}
	err := store.UpsertFileEmbedding(context.Background(), embeddings.FileEmbeddingInput{
		FileID:        7,
		EmbeddingType: "image_visual",
		ModelName:     "phash-v1",
		Vector:        "[0.0,1.0]",
	})
	if err == nil {
		t.Fatal("expected exec error")
	}
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
