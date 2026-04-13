package searchdoc

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

func NewPostgresStore(queryer RowQueryer, execer Execer) PostgresStore {
	return PostgresStore{
		Queryer: queryer,
		Execer:  execer,
	}
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
	var format sql.NullString
	var container sql.NullString
	var videoCodec sql.NullString
	var audioCodec sql.NullString
	var orientation sql.NullString

	err := s.Queryer.QueryRowContext(ctx, getFileSourceQuery, fileID).Scan(
		&item.FileID,
		&item.AbsPath,
		&item.FileName,
		&item.Extension,
		&item.MediaType,
		&item.Status,
		&width,
		&height,
		&durationMS,
		&format,
		&container,
		&videoCodec,
		&audioCodec,
		&orientation,
	)
	if err != nil {
		return FileSource{}, err
	}
	item.Width = intPtrFromNull(width)
	item.Height = intPtrFromNull(height)
	item.DurationMS = int64PtrFromNull(durationMS)
	item.Format = stringFromNull(format)
	item.Container = stringFromNull(container)
	item.VideoCodec = stringFromNull(videoCodec)
	item.AudioCodec = stringFromNull(audioCodec)
	item.Orientation = stringFromNull(orientation)
	return item, nil
}

func (s PostgresStore) UpsertSearchDocument(ctx context.Context, input DocumentInput) error {
	return s.Execer.ExecContext(ctx, upsertSearchDocumentQuery, input.FileID, input.DocumentText)
}

func (s PostgresStore) UpsertSearchAnalysis(ctx context.Context, input SearchAnalysisInput) error {
	return s.Execer.ExecContext(ctx, upsertSearchAnalysisQuery, input.FileID, input.Summary)
}

const getFileSourceQuery = `
select
  f.id,
  f.abs_path,
  f.file_name,
  f.extension,
  f.media_type,
  f.status,
  coalesce(ia.width, va.width) as width,
  coalesce(ia.height, va.height) as height,
  va.duration_ms,
  ia.format,
  va.container,
  va.video_codec,
  va.audio_codec,
  ia.orientation
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
where f.id = $1
`

const upsertSearchDocumentQuery = `
insert into search_documents (
  file_id,
  document_text,
  tsv,
  updated_at
)
values (
  $1,
  $2,
  to_tsvector('simple', $2),
  now()
)
on conflict (file_id) do update
set
  document_text = excluded.document_text,
  tsv = excluded.tsv,
  updated_at = now()
`

const upsertSearchAnalysisQuery = `
with inserted as (
  insert into analysis_results (
    file_id,
    analysis_type,
    status,
    summary,
    provider,
    model_name,
    prompt_version,
    analysis_version
  )
  values (
    $1,
    'search_doc',
    'succeeded',
    $2,
    'system',
    'searchdoc',
    'searchdoc-v1',
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
  'search_doc',
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
