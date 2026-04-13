package sameperson

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPostgresStoreGetFileContextReturnsEmbeddings(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := PostgresStore{
		RowQueryer: &recordingRowQueryer{
			row: staticContextRow{
				file: FileContext{
					FileID:              7,
					ParentPath:          "/Volumes/media/set-a",
					FileName:            "alice-001.jpg",
					MediaType:           "image",
					ModTime:             now,
					Status:              "active",
					DurationMS:          0,
					Width:               1920,
					Height:              1080,
					HasFace:             true,
					SubjectCount:        "single",
					CaptureType:         "selfie",
					ImageEmbedding:      "[0.9,0.8,0.7]",
					ImageEmbeddingType:  "person_visual",
					ImageEmbeddingModel: "semantic-v1",
				},
			},
		},
	}

	item, err := store.GetFileContext(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get file context to succeed: %v", err)
	}
	if item.ImageEmbedding != "[0.9,0.8,0.7]" || item.ImageEmbeddingType != "person_visual" || item.ImageEmbeddingModel != "semantic-v1" || item.MediaType != "image" || item.DurationMS != 0 || item.Width != 1920 || item.Height != 1080 || !item.HasFace || item.SubjectCount != "single" || item.CaptureType != "selfie" {
		t.Fatalf("unexpected file context: %#v", item)
	}
}

func TestPostgresStoreListFilesWithPersonTagLoadsVideoFrameEmbeddings(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := PostgresStore{
		RowsQueryer: &recordingRowsQueryer{
			rows: []RowsScanner{
				&staticCandidateRows{
					items: []PersonCandidateFile{
						{FileID: 17, ParentPath: "/Volumes/media/a", FileName: "a.jpg", MediaType: "image", ModTime: now, DurationMS: 0, Width: 1920, Height: 1080, HasFace: true, SubjectCount: "single", CaptureType: "selfie", ImageEmbedding: "[0.1,0.2]", ImageEmbeddingType: "person_visual", ImageEmbeddingModel: "semantic-v1"},
						{FileID: 18, ParentPath: "/Volumes/media/b", FileName: "b.mp4", MediaType: "video", ModTime: now.Add(time.Minute), DurationMS: 60_000},
					},
				},
				&staticVectorRows{
					items: []vectorRow{
						{FileID: 18, Vector: "[0.2,0.3]", Type: "person_visual", ModelName: "semantic-v1"},
						{FileID: 18, Vector: "[0.4,0.5]", Type: "person_visual", ModelName: "semantic-v1"},
					},
				},
			},
		},
	}

	items, err := store.ListFilesWithPersonTag(context.Background(), "alice")
	if err != nil {
		t.Fatalf("expected candidate listing to succeed: %v", err)
	}
	if len(items) != 2 || items[1].DurationMS != 60_000 || items[1].VideoFrameEmbeddings[1] != "[0.4,0.5]" || items[1].VideoFrameEmbeddingType != "person_visual" || items[1].VideoFrameEmbeddingModel != "semantic-v1" {
		t.Fatalf("unexpected person candidates: %#v", items)
	}
	if items[0].Width != 1920 || items[0].Height != 1080 || !items[0].HasFace || items[0].SubjectCount != "single" || items[0].CaptureType != "selfie" || items[0].ImageEmbeddingType != "person_visual" {
		t.Fatalf("expected structured person signals to be loaded, got %#v", items[0])
	}
}

func TestBuildPersonVideoFrameEmbeddingsQueryBuildsPlaceholders(t *testing.T) {
	query, args := buildPersonVideoFrameEmbeddingsQuery([]int64{17, 18, 19})
	if !strings.Contains(query, "$1, $2, $3") {
		t.Fatalf("unexpected query: %s", query)
	}
	if len(args) != 3 || args[2] != int64(19) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestSamePersonQueriesDoNotPinEmbeddingModelName(t *testing.T) {
	if strings.Contains(getFileContextQuery, "model_name = 'phash-v1'") {
		t.Fatalf("getFileContextQuery should not pin model name: %s", getFileContextQuery)
	}
	if strings.Contains(listFilesWithPersonTagQuery, "model_name = 'phash-v1'") {
		t.Fatalf("listFilesWithPersonTagQuery should not pin model name: %s", listFilesWithPersonTagQuery)
	}
	if strings.Contains(listPersonVideoFrameEmbeddingsBaseQuery, "model_name = 'phash-v1'") {
		t.Fatalf("listPersonVideoFrameEmbeddingsBaseQuery should not pin model name: %s", listPersonVideoFrameEmbeddingsBaseQuery)
	}
}

func TestSamePersonQueriesPreferPersonVisualEmbeddings(t *testing.T) {
	for _, query := range []string{
		getFileContextQuery,
		listFilesWithPersonTagQuery,
		listFilesWithAutoPersonTagQuery,
		listPersonVideoFrameEmbeddingsBaseQuery,
	} {
		if !strings.Contains(query, "person_visual") {
			t.Fatalf("same_person query should prefer person_visual embeddings: %s", query)
		}
	}
}

func TestAutoPersonQueriesUseStructuredAttributeSignals(t *testing.T) {
	for _, fragment := range []string{
		"structured_attributes",
		"subject_count",
		"has_face",
		"capture_type",
	} {
		if !strings.Contains(listAutoPersonTagsQuery, fragment) {
			t.Fatalf("listAutoPersonTagsQuery should include %q: %s", fragment, listAutoPersonTagsQuery)
		}
	}
	for _, fragment := range []string{
		"analysis_results",
		"subject_count",
		"has_face",
		"capture_type",
	} {
		if !strings.Contains(listFilesWithAutoPersonTagQuery, fragment) {
			t.Fatalf("listFilesWithAutoPersonTagQuery should include %q: %s", fragment, listFilesWithAutoPersonTagQuery)
		}
	}
}

type recordingRowQueryer struct {
	row staticContextRow
}

func (q *recordingRowQueryer) QueryRowContext(_ context.Context, _ string, _ ...any) RowScanner {
	return q.row
}

type staticContextRow struct {
	file FileContext
}

func (r staticContextRow) Scan(dest ...any) error {
	*dest[0].(*int64) = r.file.FileID
	*dest[1].(*string) = r.file.ParentPath
	*dest[2].(*string) = r.file.FileName
	*dest[3].(*string) = r.file.MediaType
	*dest[4].(*time.Time) = r.file.ModTime
	*dest[5].(*string) = r.file.Status
	*dest[6].(*int64) = r.file.DurationMS
	*dest[7].(*int64) = r.file.Width
	*dest[8].(*int64) = r.file.Height
	*dest[9].(*bool) = r.file.HasFace
	*dest[10].(*string) = r.file.SubjectCount
	*dest[11].(*string) = r.file.CaptureType
	*dest[12].(*string) = r.file.ImageEmbedding
	*dest[13].(*string) = r.file.ImageEmbeddingType
	*dest[14].(*string) = r.file.ImageEmbeddingModel
	return nil
}

type recordingRowsQueryer struct {
	rows []RowsScanner
}

func (q *recordingRowsQueryer) QueryContext(_ context.Context, _ string, _ ...any) (RowsScanner, error) {
	row := q.rows[0]
	q.rows = q.rows[1:]
	return row, nil
}

type staticCandidateRows struct {
	items []PersonCandidateFile
	index int
}

func (r *staticCandidateRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticCandidateRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.FileID
	*dest[1].(*string) = item.ParentPath
	*dest[2].(*string) = item.FileName
	*dest[3].(*string) = item.MediaType
	*dest[4].(*time.Time) = item.ModTime
	*dest[5].(*int64) = item.DurationMS
	*dest[6].(*int64) = item.Width
	*dest[7].(*int64) = item.Height
	*dest[8].(*bool) = item.HasFace
	*dest[9].(*string) = item.SubjectCount
	*dest[10].(*string) = item.CaptureType
	*dest[11].(*string) = item.ImageEmbedding
	*dest[12].(*string) = item.ImageEmbeddingType
	*dest[13].(*string) = item.ImageEmbeddingModel
	return nil
}

func (r *staticCandidateRows) Err() error   { return nil }
func (r *staticCandidateRows) Close() error { return nil }

type vectorRow struct {
	FileID    int64
	Vector    string
	Type      string
	ModelName string
}

type staticVectorRows struct {
	items []vectorRow
	index int
}

func (r *staticVectorRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticVectorRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.FileID
	*dest[1].(*string) = item.Vector
	*dest[2].(*string) = item.Type
	*dest[3].(*string) = item.ModelName
	return nil
}

func (r *staticVectorRows) Err() error   { return nil }
func (r *staticVectorRows) Close() error { return nil }
