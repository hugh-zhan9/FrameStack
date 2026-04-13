package quality

import (
	"context"
	"database/sql"
)

type RowScanner interface {
	Scan(dest ...any) error
}

type RowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
}

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type SQLDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type PostgresStore struct {
	Queryer RowQueryer
	Execer  Execer
}

func NewPostgresStoreFromDB(db SQLDB, execer Execer) PostgresStore {
	return PostgresStore{
		Queryer: sqlRowQueryer{db: db},
		Execer:  execer,
	}
}

func (s PostgresStore) GetFileSource(ctx context.Context, fileID int64) (FileSource, error) {
	var item FileSource
	var width sql.NullInt64
	var height sql.NullInt64
	var durationMS sql.NullInt64
	var bitrate sql.NullInt64
	var fps sql.NullFloat64
	var format sql.NullString
	var container sql.NullString
	var videoCodec sql.NullString
	var audioCodec sql.NullString

	err := s.Queryer.QueryRowContext(ctx, getFileSourceQuery, fileID).Scan(
		&item.FileID,
		&item.MediaType,
		&width,
		&height,
		&durationMS,
		&bitrate,
		&fps,
		&format,
		&container,
		&videoCodec,
		&audioCodec,
	)
	if err != nil {
		return FileSource{}, err
	}
	item.Width = intPtrFromNull(width)
	item.Height = intPtrFromNull(height)
	item.DurationMS = int64PtrFromNull(durationMS)
	item.Bitrate = int64PtrFromNull(bitrate)
	item.FPS = float64PtrFromNull(fps)
	item.Format = stringFromNull(format)
	item.Container = stringFromNull(container)
	item.VideoCodec = stringFromNull(videoCodec)
	item.AudioCodec = stringFromNull(audioCodec)
	return item, nil
}

func (s PostgresStore) UpsertQualityAnalysis(ctx context.Context, input AnalysisInput) error {
	return s.Execer.ExecContext(
		ctx,
		upsertQualityAnalysisQuery,
		input.FileID,
		input.AnalysisType,
		input.Status,
		input.Summary,
		input.QualityScore,
		input.QualityTier,
	)
}

const getFileSourceQuery = `
select
  f.id,
  f.media_type,
  coalesce(ia.width, va.width) as width,
  coalesce(ia.height, va.height) as height,
  va.duration_ms,
  va.bitrate,
  va.fps,
  ia.format,
  va.container,
  va.video_codec,
  va.audio_codec
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
where f.id = $1
`

const upsertQualityAnalysisQuery = `
with inserted as (
  insert into analysis_results (
    file_id,
    analysis_type,
    status,
    summary,
    quality_score,
    quality_tier,
    provider,
    model_name,
    prompt_version,
    analysis_version
  )
  values (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    'system',
    'quality-v1',
    'quality-v1',
    1
  )
  returning id
)
insert into file_current_analysis (
  file_id,
  analysis_type,
  analysis_result_id,
  updated_at
)
select
  $1,
  $2,
  id,
  now()
from inserted
on conflict (file_id, analysis_type) do update
set
  analysis_result_id = excluded.analysis_result_id,
  updated_at = now()
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

func intPtrFromNull(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func int64PtrFromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func stringFromNull(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func float64PtrFromNull(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}
