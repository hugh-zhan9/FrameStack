package samecontent

import (
	"context"
	"strings"
	"testing"
)

func TestPostgresStoreGetFileHashReturnsEmbeddings(t *testing.T) {
	store := PostgresStore{
		RowQueryer: &recordingRowQueryer{
			row: staticHashRow{
				file: FileHash{
					FileID:              7,
					MediaType:           "image",
					DurationMS:          0,
					Width:               1920,
					Height:              1080,
					SHA256:              "abc",
					ImagePHash:          "ffffffffffffffff",
					ImageEmbedding:      "[0.9,0.8,0.7]",
					ImageEmbeddingType:  "image_visual",
					ImageEmbeddingModel: "semantic-v1",
				},
			},
		},
	}

	item, err := store.GetFileHash(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get file hash to succeed: %v", err)
	}
	if item.ImageEmbedding != "[0.9,0.8,0.7]" || item.ImageEmbeddingType != "image_visual" || item.ImageEmbeddingModel != "semantic-v1" || item.Width != 1920 || item.Height != 1080 {
		t.Fatalf("unexpected file hash: %#v", item)
	}
}

func TestBuildVideoFrameEmbeddingMatchesQueryBuildsPlaceholders(t *testing.T) {
	query, args := buildVideoFrameEmbeddingMatchesQuery([]string{"[0.1,0.2]", "[0.3,0.4]"}, "semantic-v1")
	if !strings.Contains(query, "embedding_inputs") {
		t.Fatalf("unexpected query: %s", query)
	}
	if !strings.Contains(query, "e.vector <-> iv.embedding::vector") {
		t.Fatalf("expected vector distance query: %s", query)
	}
	if !strings.Contains(query, "count(distinct iv.idx) >= $5") {
		t.Fatalf("expected minimum distinct input count: %s", query)
	}
	if !strings.Contains(query, "embedded.model_name = $3") {
		t.Fatalf("expected model filter in query: %s", query)
	}
	if len(args) != 5 || args[0] != "[0.1,0.2]" || args[2] != "semantic-v1" || args[3] != videoEmbeddingDistanceThreshold || args[4] != 2 {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestSameContentQueriesDoNotPinEmbeddingModelName(t *testing.T) {
	if strings.Contains(getFileHashQuery, "model_name = 'phash-v1'") {
		t.Fatalf("getFileHashQuery should not pin model name: %s", getFileHashQuery)
	}
	if strings.Contains(listImageEmbeddingCandidatesQuery, "model_name = 'phash-v1'") {
		t.Fatalf("listImageEmbeddingCandidatesQuery should not pin model name: %s", listImageEmbeddingCandidatesQuery)
	}
	if strings.Contains(listVideoFrameEmbeddingsQuery, "model_name = 'phash-v1'") {
		t.Fatalf("listVideoFrameEmbeddingsQuery should not pin model name: %s", listVideoFrameEmbeddingsQuery)
	}
}

func TestPostgresStoreListImageEmbeddingCandidatesScansRows(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticImageEmbeddingRows{
			items: []ImageCandidate{
				{FileID: 7, Embedding: "[0.9,0.8]", EmbeddingType: "image_visual", EmbeddingModel: "semantic-v1"},
				{FileID: 8, Embedding: "[0.9,0.7]", EmbeddingType: "image_visual", EmbeddingModel: "semantic-v1"},
			},
		},
	}
	store := PostgresStore{
		RowsQueryer: queryer,
	}

	items, err := store.ListImageEmbeddingCandidates(context.Background(), "[0.9,0.8]", "semantic-v1")
	if err != nil {
		t.Fatalf("expected list image embedding candidates to succeed: %v", err)
	}
	if len(items) != 2 || items[1].FileID != 8 {
		t.Fatalf("unexpected items: %#v", items)
	}
	if len(queryer.args) != 4 || queryer.args[1] != "semantic-v1" || queryer.args[2] != imageEmbeddingDistanceThreshold || queryer.args[3] != 24 {
		t.Fatalf("unexpected query args: %#v", queryer.args)
	}
}

type recordingRowQueryer struct {
	row staticHashRow
}

func (q *recordingRowQueryer) QueryRowContext(_ context.Context, _ string, _ ...any) RowScanner {
	return q.row
}

type staticHashRow struct {
	file FileHash
}

func (r staticHashRow) Scan(dest ...any) error {
	*dest[0].(*int64) = r.file.FileID
	*dest[1].(*string) = r.file.MediaType
	*dest[2].(*int64) = r.file.DurationMS
	*dest[3].(*int64) = r.file.Width
	*dest[4].(*int64) = r.file.Height
	*dest[5].(*string) = r.file.SHA256
	*dest[6].(*string) = r.file.ImagePHash
	*dest[7].(*string) = r.file.ImageEmbedding
	*dest[8].(*string) = r.file.ImageEmbeddingType
	*dest[9].(*string) = r.file.ImageEmbeddingModel
	return nil
}

type recordingRowsQueryer struct {
	rows RowsScanner
	args []any
}

func (q *recordingRowsQueryer) QueryContext(_ context.Context, _ string, args ...any) (RowsScanner, error) {
	q.args = append([]any(nil), args...)
	return q.rows, nil
}

type staticImageEmbeddingRows struct {
	items []ImageCandidate
	index int
}

func (r *staticImageEmbeddingRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticImageEmbeddingRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.FileID
	*dest[1].(*string) = item.Embedding
	*dest[2].(*string) = item.EmbeddingType
	*dest[3].(*string) = item.EmbeddingModel
	return nil
}

func (r *staticImageEmbeddingRows) Err() error   { return nil }
func (r *staticImageEmbeddingRows) Close() error { return nil }
