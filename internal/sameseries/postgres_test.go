package sameseries

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPostgresStoreGetFileContextReturnsMetadata(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := PostgresStore{
		RowQueryer: &recordingRowQueryer{
			row: staticContextRow{
				file: FileContext{
					FileID:              7,
					ParentPath:          "/Volumes/media/set-a",
					FileName:            "model-a-001.jpg",
					MediaType:           "image",
					ModTime:             now,
					Status:              "active",
					DurationMS:          0,
					Width:               1920,
					Height:              1080,
					CaptureType:         "photo",
					ImagePHash:          "ffffffffffffffff",
					ImageEmbedding:      "[0.9,0.8,0.7]",
					ImageEmbeddingType:  "image_visual",
					ImageEmbeddingModel: "semantic-v1",
				},
			},
		},
	}

	item, err := store.GetFileContext(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get file context to succeed: %v", err)
	}
	if item.ParentPath != "/Volumes/media/set-a" || item.FileName != "model-a-001.jpg" || item.DurationMS != 0 || item.Width != 1920 || item.Height != 1080 || item.CaptureType != "photo" || item.ImagePHash != "ffffffffffffffff" || item.ImageEmbedding != "[0.9,0.8,0.7]" || item.ImageEmbeddingType != "image_visual" || item.ImageEmbeddingModel != "semantic-v1" {
		t.Fatalf("unexpected file context: %#v", item)
	}
}

func TestPostgresStoreListSeriesCandidateFilesLoadsVideoFrames(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := PostgresStore{
		RowsQueryer: &recordingRowsQueryer{
			rows: []RowsScanner{
				&staticCandidateRows{
					items: []SeriesCandidateFile{
						{FileID: 17, ParentPath: "/Volumes/media/set-v", FileName: "clip-a-001.mp4", ModTime: now, DurationMS: 60_000, Width: 0, Height: 0, CaptureType: "photo"},
						{FileID: 18, ParentPath: "/Volumes/media/set-v", FileName: "clip-a-002.mp4", ModTime: now.Add(2 * time.Minute), DurationMS: 58_000, Width: 0, Height: 0, CaptureType: "photo"},
					},
				},
				&staticFrameRows{
					items: []frameRow{
						{FileID: 17, PHash: "a1"},
						{FileID: 17, PHash: "b2"},
						{FileID: 18, PHash: "b2"},
					},
				},
					&staticVectorRows{
						items: []vectorRow{
							{FileID: 17, Vector: "[0.1,0.2]", EmbeddingType: "video_frame_visual", ModelName: "semantic-v1"},
							{FileID: 18, Vector: "[0.2,0.3]", EmbeddingType: "video_frame_visual", ModelName: "semantic-v1"},
						},
					},
			},
		},
	}

	items, err := store.ListSeriesCandidateFiles(context.Background(), FileContext{
		FileID:     17,
		ParentPath: "/Volumes/media/set-v",
		MediaType:  "video",
		ModTime:    now,
	}, 30*time.Minute)
	if err != nil {
		t.Fatalf("expected candidate listing to succeed: %v", err)
	}
	if len(items) != 2 || items[0].DurationMS != 60_000 || items[0].CaptureType != "photo" || len(items[0].VideoFramePHashes) != 2 || items[1].DurationMS != 58_000 || items[1].VideoFramePHashes[0] != "b2" || items[1].VideoFrameEmbeddings[0] != "[0.2,0.3]" || items[1].VideoFrameEmbeddingType != "video_frame_visual" || items[1].VideoFrameEmbeddingModel != "semantic-v1" {
		t.Fatalf("unexpected video series candidates: %#v", items)
	}
}

func TestPostgresStoreListNearbySeriesCandidateFiles(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := PostgresStore{
		RowsQueryer: &recordingRowsQueryer{
			rows: []RowsScanner{
				&staticCandidateRows{
					items: []SeriesCandidateFile{
						{FileID: 21, ParentPath: "/Volumes/media/set-b", FileName: "model-a-001.jpg", ModTime: now.Add(2 * time.Minute), DurationMS: 0, Width: 1920, Height: 1080, CaptureType: "photo"},
					},
				},
			},
		},
	}

	items, err := store.ListNearbySeriesCandidateFiles(context.Background(), FileContext{
		FileID:    17,
		MediaType: "image",
		ModTime:   now,
	}, 30*time.Minute, 64)
	if err != nil {
		t.Fatalf("expected nearby candidate listing to succeed: %v", err)
	}
	if len(items) != 1 || items[0].ParentPath != "/Volumes/media/set-b" || items[0].DurationMS != 0 || items[0].Width != 1920 || items[0].Height != 1080 || items[0].CaptureType != "photo" {
		t.Fatalf("unexpected nearby candidates: %#v", items)
	}
}

func TestBuildSeriesVideoFramePHashesQueryBuildsPlaceholders(t *testing.T) {
	query, args := buildSeriesVideoFramePHashesQuery([]int64{17, 18, 19})
	if !strings.Contains(query, "$1, $2, $3") {
		t.Fatalf("unexpected query: %s", query)
	}
	if len(args) != 3 || args[2] != int64(19) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBuildSeriesVideoFrameEmbeddingsQueryBuildsPlaceholders(t *testing.T) {
	query, args := buildSeriesVideoFrameEmbeddingsQuery([]int64{17, 18, 19})
	if !strings.Contains(query, "$1, $2, $3") {
		t.Fatalf("unexpected query: %s", query)
	}
	if len(args) != 3 || args[2] != int64(19) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestSameSeriesQueriesDoNotPinEmbeddingModelName(t *testing.T) {
	if strings.Contains(getFileContextQuery, "model_name = 'phash-v1'") {
		t.Fatalf("getFileContextQuery should not pin model name: %s", getFileContextQuery)
	}
	if strings.Contains(listSeriesCandidateFilesQuery, "model_name = 'phash-v1'") {
		t.Fatalf("listSeriesCandidateFilesQuery should not pin model name: %s", listSeriesCandidateFilesQuery)
	}
	if strings.Contains(listSeriesVideoFrameEmbeddingsBaseQuery, "model_name = 'phash-v1'") {
		t.Fatalf("listSeriesVideoFrameEmbeddingsBaseQuery should not pin model name: %s", listSeriesVideoFrameEmbeddingsBaseQuery)
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
	*dest[9].(*string) = r.file.CaptureType
	*dest[10].(*string) = r.file.ImagePHash
	*dest[11].(*string) = r.file.ImageEmbedding
	*dest[12].(*string) = r.file.ImageEmbeddingType
	*dest[13].(*string) = r.file.ImageEmbeddingModel
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
	items []SeriesCandidateFile
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
	*dest[3].(*time.Time) = item.ModTime
	*dest[4].(*int64) = item.DurationMS
	*dest[5].(*int64) = item.Width
	*dest[6].(*int64) = item.Height
	*dest[7].(*string) = item.CaptureType
	*dest[8].(*string) = item.ImagePHash
	*dest[9].(*string) = item.ImageEmbedding
	*dest[10].(*string) = item.ImageEmbeddingType
	*dest[11].(*string) = item.ImageEmbeddingModel
	return nil
}

func (r *staticCandidateRows) Err() error   { return nil }
func (r *staticCandidateRows) Close() error { return nil }

type frameRow struct {
	FileID int64
	PHash  string
}

type staticFrameRows struct {
	items []frameRow
	index int
}

func (r *staticFrameRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticFrameRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.FileID
	*dest[1].(*string) = item.PHash
	return nil
}

func (r *staticFrameRows) Err() error   { return nil }
func (r *staticFrameRows) Close() error { return nil }

type vectorRow struct {
	FileID        int64
	Vector        string
	EmbeddingType string
	ModelName     string
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
	*dest[2].(*string) = item.EmbeddingType
	*dest[3].(*string) = item.ModelName
	return nil
}

func (r *staticVectorRows) Err() error   { return nil }
func (r *staticVectorRows) Close() error { return nil }
