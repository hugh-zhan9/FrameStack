package files_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"idea/internal/files"
)

func TestPostgresStoreListFilesReturnsItems(t *testing.T) {
	now := time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC)
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{
			items: []files.File{
				{
					ID:           1,
					VolumeID:     7,
					AbsPath:      "/Volumes/media/photo.jpg",
					FileName:     "photo.jpg",
					MediaType:    "image",
					Status:       "active",
					SizeBytes:    123,
					UpdatedAt:    now.Format(time.RFC3339),
					Width:        intPtr(320),
					Height:       intPtr(180),
					Format:       "jpg",
					QualityScore: float64Ptr(82),
					QualityTier:  "high",
					ReviewAction: "favorite",
					TagNames:     []string{"单人写真", "室内"},
					HasPreview:   true,
				},
			},
		},
	}
	store := files.PostgresStore{Rows: queryer}

	result, err := store.ListFiles(context.Background(), files.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].FileName != "photo.jpg" {
		t.Fatalf("unexpected files: %#v", result.Items)
	}
	if result.Items[0].Width == nil || *result.Items[0].Width != 320 || result.Items[0].Format != "jpg" {
		t.Fatalf("expected extracted metadata, got %#v", result.Items[0])
	}
	if result.Items[0].QualityTier != "high" {
		t.Fatalf("expected quality tier, got %#v", result.Items[0])
	}
	if result.Items[0].QualityScore == nil || *result.Items[0].QualityScore != 82 {
		t.Fatalf("expected quality score, got %#v", result.Items[0])
	}
	if result.Items[0].ReviewAction != "favorite" {
		t.Fatalf("expected latest review action, got %#v", result.Items[0])
	}
	if len(result.Items[0].TagNames) != 2 || result.Items[0].TagNames[0] != "单人写真" {
		t.Fatalf("expected tag summary, got %#v", result.Items[0].TagNames)
	}
	if !result.Items[0].HasPreview {
		t.Fatalf("expected preview flag, got %#v", result.Items[0])
	}
	if result.HasMore || result.NextCursor != "" {
		t.Fatalf("expected no extra page metadata, got %#v", result)
	}
	if !strings.Contains(normalizeSQL(queryer.query), normalizeSQL("from files")) {
		t.Fatalf("unexpected query: %s", queryer.query)
	}
}

func TestPostgresStoreListFilesReturnsQueryError(t *testing.T) {
	store := files.PostgresStore{
		Rows: &recordingRowsQueryer{err: errors.New("db down")},
	}

	_, err := store.ListFiles(context.Background(), files.ListOptions{})
	if err == nil {
		t.Fatal("expected list files to fail")
	}
}

func TestPostgresStoreListFilesSupportsSearchQuery(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}

	_, err := store.ListFiles(context.Background(), files.ListOptions{
		Limit: 10,
		Query: "poster jpg",
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	if len(queryer.args) != 13 || queryer.args[0] != "poster jpg" || queryer.args[1] != "" || queryer.args[2] != "" || queryer.args[3] != "" || queryer.args[4] != "" || queryer.args[5] != int64(0) || queryer.args[6] != "" || queryer.args[7] != "" || queryer.args[8] != "" || queryer.args[9] != "" || queryer.args[10] != "" || queryer.args[11] != 11 || queryer.args[12] != 0 {
		t.Fatalf("unexpected query args: %#v", queryer.args)
	}
	normalized := normalizeSQL(queryer.query)
	expectedFragments := []string{
		"left join search_documents sd on sd.file_id = f.id",
		"websearch_to_tsquery('simple', $1)",
		"sd.tsv @@ websearch_to_tsquery('simple', $1)",
		"f.file_name ilike '%' || $1 || '%'",
		"f.abs_path ilike '%' || $1 || '%'",
		"sd.document_text ilike '%' || $1 || '%'",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreListFilesSupportsStructuredFilters(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}

	_, err := store.ListFiles(context.Background(), files.ListOptions{
		Limit:        15,
		Query:        "poster",
		MediaType:    "image",
		QualityTier:  "high",
		ReviewAction: "favorite",
		Status:       "active",
		VolumeID:     7,
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	if len(queryer.args) != 13 {
		t.Fatalf("expected 13 query args, got %#v", queryer.args)
	}
	expectedArgs := []any{"poster", "image", "high", "favorite", "active", int64(7), "", "", "", "", "", 16, 0}
	for i, expected := range expectedArgs {
		if queryer.args[i] != expected {
			t.Fatalf("unexpected arg %d: want %#v got %#v", i, expected, queryer.args[i])
		}
	}
	normalized := normalizeSQL(queryer.query)
	expectedFragments := []string{
		"sd.tsv @@ websearch_to_tsquery('simple', $1)",
		"or f.file_name ilike '%' || $1 || '%'",
		"or f.abs_path ilike '%' || $1 || '%'",
		"or sd.document_text ilike '%' || $1 || '%'",
		"and ($2 = '' or f.media_type = $2)",
		"and ($3 = '' or quality.quality_tier = $3)",
		"and ($4 = '' or latest_review.action_type = $4)",
		"and ($5 = '' or f.status = $5)",
		"$6 = 0 or f.volume_id = $6",
		"limit $12",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreListFilesSupportsTagFilter(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}

	_, err := store.ListFiles(context.Background(), files.ListOptions{
		Limit:        10,
		ReviewAction: "favorite",
		Tag:          "单人写真",
		TagNamespace: "content",
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	if len(queryer.args) != 13 {
		t.Fatalf("expected 13 query args, got %#v", queryer.args)
	}
	if queryer.args[3] != "favorite" || queryer.args[7] != "content" || queryer.args[8] != "单人写真" {
		t.Fatalf("unexpected tag arg: %#v", queryer.args)
	}
	normalized := normalizeSQL(queryer.query)
	expectedFragments := []string{
		"or ($7 = 'true' and exists (",
		"or ($7 = 'false' and not exists (",
		"exists (",
		"from file_tags ft",
		"join tags t on t.id = ft.tag_id",
		"and ($8 = '' or t.namespace = $8)",
		"and ($9 = '' or t.name = $9 or t.display_name = $9)",
		"limit $12",
		"offset $13",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreListFilesSupportsHasTagsFilter(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}

	_, err := store.ListFiles(context.Background(), files.ListOptions{
		Limit:   10,
		HasTags: "true",
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	if len(queryer.args) != 13 || queryer.args[6] != "true" {
		t.Fatalf("unexpected query args: %#v", queryer.args)
	}
	normalized := normalizeSQL(queryer.query)
	expectedFragments := []string{
		"$7 = 'true' and exists (",
		"$7 = 'false' and not exists (",
		"from file_tags ft",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreListFilesSupportsOffsetAndSort(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}

	_, err := store.ListFiles(context.Background(), files.ListOptions{
		Limit:  25,
		Offset: 50,
		Sort:   "size_desc",
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	if len(queryer.args) != 13 {
		t.Fatalf("expected 13 query args, got %#v", queryer.args)
	}
	if queryer.args[11] != 26 || queryer.args[12] != 50 {
		t.Fatalf("unexpected paging args: %#v", queryer.args)
	}
	normalized := normalizeSQL(queryer.query)
	expectedFragments := []string{
		"order by f.size_bytes desc, f.id desc",
		"limit $12",
		"offset $13",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreListFilesSupportsQualitySort(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}

	_, err := store.ListFiles(context.Background(), files.ListOptions{
		Limit: 10,
		Sort:  "quality_desc",
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	normalized := normalizeSQL(queryer.query)
	if !strings.Contains(normalized, normalizeSQL("order by coalesce(quality.quality_score, -1) desc, f.updated_at desc, f.id desc")) {
		t.Fatalf("expected quality sort, got %q", queryer.query)
	}
}

func TestPostgresStoreListFilesSupportsCursorForUpdatedSort(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}
	cursor, err := filesTestEncodeCursor("updated_desc", 7, "2026-04-13T12:00:00Z", 0, "", nil)
	if err != nil {
		t.Fatalf("expected cursor to encode: %v", err)
	}

	_, err = store.ListFiles(context.Background(), files.ListOptions{
		Limit:  25,
		Sort:   "updated_desc",
		Cursor: cursor,
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	normalized := normalizeSQL(queryer.query)
	if !strings.Contains(normalized, normalizeSQL("and (f.updated_at, f.id) < ($12::timestamptz, $13)")) {
		t.Fatalf("expected updated cursor clause, got %q", queryer.query)
	}
	if len(queryer.args) != 15 || queryer.args[11] != "2026-04-13T12:00:00Z" || queryer.args[12] != int64(7) || queryer.args[13] != 26 || queryer.args[14] != 0 {
		t.Fatalf("unexpected cursor args: %#v", queryer.args)
	}
}

func TestPostgresStoreListFilesSupportsCursorForQualitySort(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}
	quality := 82.0
	cursor, err := filesTestEncodeCursor("quality_desc", 7, "2026-04-13T12:00:00Z", 0, "", &quality)
	if err != nil {
		t.Fatalf("expected cursor to encode: %v", err)
	}

	_, err = store.ListFiles(context.Background(), files.ListOptions{
		Limit:  10,
		Sort:   "quality_desc",
		Cursor: cursor,
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	normalized := normalizeSQL(queryer.query)
	if !strings.Contains(normalized, normalizeSQL("and (coalesce(quality.quality_score, -1), f.updated_at, f.id) < ($12, $13::timestamptz, $14)")) {
		t.Fatalf("expected quality cursor clause, got %q", queryer.query)
	}
	if len(queryer.args) != 16 || queryer.args[11] != 82.0 || queryer.args[12] != "2026-04-13T12:00:00Z" || queryer.args[13] != int64(7) || queryer.args[14] != 11 || queryer.args[15] != 0 {
		t.Fatalf("unexpected cursor args: %#v", queryer.args)
	}
}

func TestPostgresStoreListFilesSupportsClusterFilter(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticFileRows{},
	}
	store := files.PostgresStore{Rows: queryer}

	_, err := store.ListFiles(context.Background(), files.ListOptions{
		Limit:         10,
		ClusterType:   "same_series",
		ClusterStatus: "candidate",
	})
	if err != nil {
		t.Fatalf("expected list files to succeed: %v", err)
	}
	if len(queryer.args) != 13 {
		t.Fatalf("expected 13 query args, got %#v", queryer.args)
	}
	if queryer.args[9] != "same_series" || queryer.args[10] != "candidate" {
		t.Fatalf("unexpected cluster args: %#v", queryer.args)
	}
	normalized := normalizeSQL(queryer.query)
	expectedFragments := []string{
		"from cluster_members cm",
		"join clusters c on c.id = cm.cluster_id",
		"and ($10 = '' or c.cluster_type = $10)",
		"and ($11 = '' or c.status = $11)",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreGetFileDetailReturnsFileAndPathHistory(t *testing.T) {
	queryer := &recordingDetailQueryer{
		fileRow: staticFileDetailRow{
			item: files.FileDetail{
				File: files.File{
					ID:          7,
					VolumeID:    3,
					AbsPath:     "/Volumes/media/photos/poster.jpg",
					FileName:    "poster.jpg",
					MediaType:   "image",
					Status:      "active",
					SizeBytes:   1234,
					Format:      "jpg",
					QualityTier: "medium",
					FPS:         float64Ptr(29.97),
					Bitrate:     int64Ptr(8_000_000),
					VideoCodec:  "h264",
					AudioCodec:  "aac",
				},
			},
		},
		historyRows: &staticPathHistoryRows{
			items: []files.PathHistory{
				{AbsPath: "/Volumes/media/photos/poster.jpg", EventType: "discovered", SeenAt: "2026-04-09T20:00:00Z"},
				{AbsPath: "/Volumes/old/poster.jpg", EventType: "moved", SeenAt: "2026-04-08T20:00:00Z"},
			},
		},
		analysisRows: &staticCurrentAnalysisRows{
			items: []files.CurrentAnalysis{
				{AnalysisType: "quality", Status: "succeeded", Summary: "image quality high, 1920x1080 jpg.", QualityScore: float64Ptr(82), QualityTier: "high", CreatedAt: "2026-04-09T20:01:00Z"},
			},
		},
		tagRows: &staticFileTagRows{
			items: []files.FileTag{
				{Namespace: "content", Name: "单人写真", DisplayName: "单人写真", Source: "ai"},
				{Namespace: "management", Name: "待AI精标", DisplayName: "待AI精标", Source: "ai"},
			},
		},
		reviewRows: &staticReviewActionRows{
			items: []files.ReviewAction{
				{ActionType: "favorite", Note: "manual favorite", CreatedAt: "2026-04-09T20:02:00Z"},
			},
		},
		clusterRows: &staticFileClusterRows{
			items: []files.FileCluster{
				{ClusterID: 31, ClusterType: "same_content", Title: "same_content:abc", Status: "candidate"},
				{ClusterID: 32, ClusterType: "same_series", Title: "same_series:set-a", Status: "candidate"},
			},
		},
		embeddingRows: &staticEmbeddingRows{
			items: []files.EmbeddingInfo{
				{EmbeddingType: "image_visual", Provider: "semantic", ModelName: "semantic-ollama-qwen3-vl-8b-v1", VectorCount: 1},
				{EmbeddingType: "video_frame_visual", Provider: "pixel", ModelName: "ffmpeg-gray-8x8-v1", VectorCount: 5},
			},
		},
		videoFrameRows: &staticVideoFrameRows{
			items: []files.VideoFrame{
				{TimestampMS: 5_000, FrameRole: "understanding", PHash: "frame-a"},
				{TimestampMS: 45_000, FrameRole: "understanding", PHash: "frame-b"},
			},
		},
	}
	store := files.PostgresStore{
		Rows:          queryer,
		DetailQueryer: queryer,
	}

	item, err := store.GetFileDetail(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected detail lookup to succeed: %v", err)
	}
	if item.ID != 7 || item.QualityTier != "medium" || len(item.PathHistory) != 2 || len(item.CurrentAnalyses) != 1 || len(item.Tags) != 2 || len(item.ReviewActions) != 1 {
		t.Fatalf("unexpected file detail: %#v", item)
	}
	if item.FPS == nil || *item.FPS != 29.97 || item.Bitrate == nil || *item.Bitrate != 8_000_000 || item.VideoCodec != "h264" || item.AudioCodec != "aac" {
		t.Fatalf("expected video metadata in detail, got %#v", item.File)
	}
	if len(item.Clusters) != 2 || item.Clusters[0].ClusterType != "same_content" || item.Clusters[1].ClusterType != "same_series" {
		t.Fatalf("expected cluster memberships, got %#v", item.Clusters)
	}
	if item.CurrentAnalyses[0].QualityScore == nil || *item.CurrentAnalyses[0].QualityScore != 82 || item.CurrentAnalyses[0].QualityTier != "high" {
		t.Fatalf("expected quality analysis details, got %#v", item.CurrentAnalyses[0])
	}
	if item.ReviewActions[0].ActionType != "favorite" {
		t.Fatalf("expected review actions, got %#v", item.ReviewActions)
	}
	if len(item.Embeddings) != 2 || item.Embeddings[0].Provider != "semantic" || item.Embeddings[1].VectorCount != 5 {
		t.Fatalf("expected embedding detail, got %#v", item.Embeddings)
	}
	if len(item.VideoFrames) != 2 || item.VideoFrames[0].TimestampMS != 5_000 || item.VideoFrames[1].PHash != "frame-b" {
		t.Fatalf("expected video frame detail, got %#v", item.VideoFrames)
	}
}

type recordingRowsQueryer struct {
	query string
	args  []any
	rows  files.RowsScanner
	err   error
}

func (r *recordingRowsQueryer) QueryContext(_ context.Context, query string, args ...any) (files.RowsScanner, error) {
	r.query = query
	r.args = args
	if r.err != nil {
		return nil, r.err
	}
	return r.rows, nil
}

type recordingDetailQueryer struct {
	query        string
	args         []any
	fileRow      staticFileDetailRow
	historyRows  files.RowsScanner
	analysisRows files.RowsScanner
	tagRows      files.RowsScanner
	reviewRows   files.RowsScanner
	clusterRows  files.RowsScanner
	embeddingRows files.RowsScanner
	videoFrameRows files.RowsScanner
	queryCount   int
}

func (r *recordingDetailQueryer) QueryContext(_ context.Context, query string, args ...any) (files.RowsScanner, error) {
	r.query = query
	r.args = args
	r.queryCount++
	if r.queryCount == 1 {
		return r.historyRows, nil
	}
	if r.queryCount == 2 {
		return r.analysisRows, nil
	}
	if r.queryCount == 3 {
		return r.tagRows, nil
	}
	if r.queryCount == 4 {
		return r.reviewRows, nil
	}
	if r.queryCount == 5 {
		return r.clusterRows, nil
	}
	if r.queryCount == 6 {
		return r.embeddingRows, nil
	}
	return r.videoFrameRows, nil
}

func (r *recordingDetailQueryer) QueryRowContext(_ context.Context, query string, args ...any) files.RowScanner {
	r.query = query
	r.args = args
	return r.fileRow
}

type staticFileRows struct {
	items []files.File
	index int
}

type staticFileDetailRow struct {
	item files.FileDetail
	err  error
}

func (r staticFileDetailRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.item.ID
	*dest[1].(*int64) = r.item.VolumeID
	*dest[2].(*string) = r.item.AbsPath
	*dest[3].(*string) = r.item.FileName
	*dest[4].(*string) = r.item.MediaType
	*dest[5].(*string) = r.item.Status
	*dest[6].(*int64) = r.item.SizeBytes
	*dest[7].(*string) = r.item.UpdatedAt
	assignNullInt64(dest[8], r.item.Width)
	assignNullInt64(dest[9], r.item.Height)
	assignNullInt64(dest[10], r.item.DurationMS)
	assignNullString(dest[11], r.item.Format)
	assignNullString(dest[12], r.item.Container)
	assignNullString(dest[13], r.item.QualityTier)
	assignNullFloat64(dest[14], r.item.FPS)
	assignNullInt64(dest[15], r.item.Bitrate)
	assignNullString(dest[16], r.item.VideoCodec)
	assignNullString(dest[17], r.item.AudioCodec)
	return nil
}

type staticPathHistoryRows struct {
	items []files.PathHistory
	index int
}

func (r *staticPathHistoryRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticPathHistoryRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*string) = item.AbsPath
	*dest[1].(*string) = item.EventType
	*dest[2].(*string) = item.SeenAt
	return nil
}

func (r *staticPathHistoryRows) Err() error   { return nil }
func (r *staticPathHistoryRows) Close() error { return nil }

type staticCurrentAnalysisRows struct {
	items []files.CurrentAnalysis
	index int
}

func (r *staticCurrentAnalysisRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticCurrentAnalysisRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*string) = item.AnalysisType
	*dest[1].(*string) = item.Status
	*dest[2].(*string) = item.Summary
	assignNullFloat64(dest[3], item.QualityScore)
	assignNullString(dest[4], item.QualityTier)
	*dest[5].(*string) = item.CreatedAt
	return nil
}

func (r *staticCurrentAnalysisRows) Err() error   { return nil }
func (r *staticCurrentAnalysisRows) Close() error { return nil }

type staticFileTagRows struct {
	items []files.FileTag
	index int
}

func (r *staticFileTagRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticFileTagRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*string) = item.Namespace
	*dest[1].(*string) = item.Name
	*dest[2].(*string) = item.DisplayName
	*dest[3].(*string) = item.Source
	assignNullFloat64(dest[4], item.Confidence)
	return nil
}

func (r *staticFileTagRows) Err() error   { return nil }
func (r *staticFileTagRows) Close() error { return nil }

type staticReviewActionRows struct {
	items []files.ReviewAction
	index int
}

type staticFileClusterRows struct {
	items []files.FileCluster
	index int
}

func (r *staticReviewActionRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticReviewActionRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*string) = item.ActionType
	*dest[1].(*string) = item.Note
	*dest[2].(*string) = item.CreatedAt
	return nil
}

func (r *staticReviewActionRows) Err() error   { return nil }
func (r *staticReviewActionRows) Close() error { return nil }

func (r *staticFileClusterRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticFileClusterRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.ClusterID
	*dest[1].(*string) = item.ClusterType
	*dest[2].(*string) = item.Title
	*dest[3].(*string) = item.Status
	return nil
}

func (r *staticFileClusterRows) Err() error   { return nil }
func (r *staticFileClusterRows) Close() error { return nil }

type staticEmbeddingRows struct {
	items []files.EmbeddingInfo
	index int
}

func (r *staticEmbeddingRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticEmbeddingRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*string) = item.EmbeddingType
	*dest[1].(*string) = item.Provider
	*dest[2].(*string) = item.ModelName
	*dest[3].(*int64) = item.VectorCount
	return nil
}

func (r *staticEmbeddingRows) Err() error   { return nil }
func (r *staticEmbeddingRows) Close() error { return nil }

type staticVideoFrameRows struct {
	items []files.VideoFrame
	index int
}

func (r *staticVideoFrameRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticVideoFrameRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.TimestampMS
	*dest[1].(*string) = item.FrameRole
	*dest[2].(*string) = item.PHash
	return nil
}

func (r *staticVideoFrameRows) Err() error   { return nil }
func (r *staticVideoFrameRows) Close() error { return nil }

func (r *staticFileRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticFileRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.ID
	*dest[1].(*int64) = item.VolumeID
	*dest[2].(*string) = item.AbsPath
	*dest[3].(*string) = item.FileName
	*dest[4].(*string) = item.MediaType
	*dest[5].(*string) = item.Status
	*dest[6].(*int64) = item.SizeBytes
	*dest[7].(*string) = item.UpdatedAt
	assignNullInt64(dest[8], item.Width)
	assignNullInt64(dest[9], item.Height)
	assignNullInt64(dest[10], item.DurationMS)
	assignNullString(dest[11], item.Format)
	assignNullString(dest[12], item.Container)
	assignNullFloat64(dest[13], item.FPS)
	assignNullInt64(dest[14], item.Bitrate)
	assignNullString(dest[15], item.VideoCodec)
	assignNullString(dest[16], item.AudioCodec)
	assignNullFloat64(dest[17], item.QualityScore)
	assignNullString(dest[18], item.QualityTier)
	assignNullString(dest[19], item.ReviewAction)
	assignNullString(dest[20], strings.Join(item.TagNames, "||"))
	*dest[21].(*bool) = item.HasPreview
	assignNullString(dest[22], item.ThumbnailPath)
	return nil
}

func (r *staticFileRows) Err() error   { return nil }
func (r *staticFileRows) Close() error { return nil }

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func intPtr(v int) *int {
	return &v
}

func assignNullInt64(target any, value any) {
	ptr := target.(*sql.NullInt64)
	switch typed := value.(type) {
	case *int:
		if typed == nil {
			*ptr = sql.NullInt64{}
			return
		}
		*ptr = sql.NullInt64{Int64: int64(*typed), Valid: true}
	case *int64:
		if typed == nil {
			*ptr = sql.NullInt64{}
			return
		}
		*ptr = sql.NullInt64{Int64: *typed, Valid: true}
	default:
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

func assignNullFloat64(target any, value *float64) {
	ptr := target.(*sql.NullFloat64)
	if value == nil {
		*ptr = sql.NullFloat64{}
		return
	}
	*ptr = sql.NullFloat64{Float64: *value, Valid: true}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func filesTestEncodeCursor(sort string, id int64, updatedAt string, sizeBytes int64, fileName string, qualityKey *float64) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"sort":        sort,
		"id":          id,
		"updated_at":  updatedAt,
		"size_bytes":  sizeBytes,
		"file_name":   fileName,
		"quality_key": qualityKey,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
