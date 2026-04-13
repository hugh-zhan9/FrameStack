package quality_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"idea/internal/quality"
)

func TestPostgresStoreGetFileSourceReturnsMetadata(t *testing.T) {
	store := quality.PostgresStore{
		Queryer: &recordingRowQueryer{
			row: staticFileSourceRow{
				source: quality.FileSource{
					FileID:     7,
					MediaType:  "video",
					Width:      intPtr(1920),
					Height:     intPtr(1080),
					Bitrate:    int64Ptr(8_000_000),
					FPS:        float64Ptr(29.97),
					Format:     "jpg",
					Container:  "mp4",
					VideoCodec: "h264",
					AudioCodec: "aac",
				},
			},
		},
	}

	item, err := store.GetFileSource(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected get source to succeed: %v", err)
	}
	if item.FileID != 7 || item.Format != "jpg" || item.FPS == nil || *item.FPS != 29.97 || item.VideoCodec != "h264" || item.AudioCodec != "aac" {
		t.Fatalf("unexpected file source: %#v", item)
	}
}

func TestPostgresStoreUpsertQualityAnalysisExecutesExpectedSQL(t *testing.T) {
	execer := &recordingExecer{}
	store := quality.PostgresStore{Execer: execer}

	if err := store.UpsertQualityAnalysis(context.Background(), quality.AnalysisInput{
		FileID:       7,
		AnalysisType: "quality",
		Status:       "succeeded",
		Summary:      "图片质量 high，1920x1080。",
		QualityScore: 82,
		QualityTier:  "high",
	}); err != nil {
		t.Fatalf("expected upsert to succeed: %v", err)
	}
	normalized := normalizeSQL(execer.query)
	for _, fragment := range []string{"insert into analysis_results", "quality_score", "quality_tier", "insert into file_current_analysis"} {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, execer.query)
		}
	}
}

type recordingRowQueryer struct {
	row staticFileSourceRow
}

func (q *recordingRowQueryer) QueryRowContext(_ context.Context, _ string, _ ...any) quality.RowScanner {
	return q.row
}

type staticFileSourceRow struct {
	source quality.FileSource
}

func (r staticFileSourceRow) Scan(dest ...any) error {
	*dest[0].(*int64) = r.source.FileID
	*dest[1].(*string) = r.source.MediaType
	assignNullInt64(dest[2], r.source.Width)
	assignNullInt64(dest[3], r.source.Height)
	assignNullInt64(dest[4], r.source.DurationMS)
	assignNullInt64(dest[5], r.source.Bitrate)
	assignNullFloat64(dest[6], r.source.FPS)
	assignNullString(dest[7], r.source.Format)
	assignNullString(dest[8], r.source.Container)
	assignNullString(dest[9], r.source.VideoCodec)
	assignNullString(dest[10], r.source.AudioCodec)
	return nil
}

type recordingExecer struct {
	query string
}

func (e *recordingExecer) ExecContext(_ context.Context, query string, _ ...any) error {
	e.query = query
	return nil
}

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
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
