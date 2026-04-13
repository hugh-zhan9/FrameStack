package understand

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
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type RowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type RowsQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error)
}

type PostgresStore struct {
	Queryer RowQueryer
	Rows    RowsQueryer
	Execer  Execer
}

func NewPostgresStore(queryer RowQueryer, execer Execer) PostgresStore {
	return PostgresStore{
		Queryer: queryer,
		Rows:    nil,
		Execer:  execer,
	}
}

func NewPostgresStoreFromDB(db SQLDB, execer Execer) PostgresStore {
	return PostgresStore{
		Queryer: sqlRowQueryer{db: db},
		Rows:    sqlRowsQueryer{db: db},
		Execer:  execer,
	}
}

func (s PostgresStore) GetFile(ctx context.Context, fileID int64) (File, error) {
	var item File
	err := s.Queryer.QueryRowContext(ctx, getFileQuery, fileID).Scan(
		&item.ID,
		&item.AbsPath,
		&item.FileName,
		&item.MediaType,
	)
	if err != nil {
		return File{}, err
	}
	if s.Rows != nil {
		rows, err := s.Rows.QueryContext(ctx, listUnderstandingFramesQuery, fileID)
		if err != nil {
			return File{}, err
		}
		defer rows.Close()
		for rows.Next() {
			var framePath string
			if err := rows.Scan(&framePath); err != nil {
				return File{}, err
			}
			item.FramePaths = append(item.FramePaths, framePath)
		}
		if err := rows.Err(); err != nil {
			return File{}, err
		}
	}
	return item, nil
}

func (s PostgresStore) UpsertAnalysis(ctx context.Context, input AnalysisInput) error {
	return s.Execer.ExecContext(
		ctx,
		upsertAnalysisQuery,
		input.FileID,
		input.AnalysisType,
		input.Status,
		input.Summary,
		input.StructuredAttributes,
		input.RawModelOutput,
		input.Provider,
		input.ModelName,
		input.PromptVersion,
		input.AnalysisVersion,
	)
}

func (s PostgresStore) ReplaceAITags(ctx context.Context, fileID int64, tags []TagCandidate) error {
	if err := s.Execer.ExecContext(ctx, deleteAITagsQuery, fileID); err != nil {
		return err
	}
	for _, tag := range tags {
		if err := s.Execer.ExecContext(ctx, upsertTagQuery, tag.Namespace, tag.Name, tag.Name, tag.Namespace == "sensitive"); err != nil {
			return err
		}
		if err := s.Execer.ExecContext(ctx, upsertFileTagQuery, fileID, tag.Namespace, tag.Name, tag.Confidence, `{"source":"understanding"}`); err != nil {
			return err
		}
	}
	return nil
}

const getFileQuery = `
select
  id,
  abs_path,
  file_name,
  media_type
from files
where id = $1
`

const listUnderstandingFramesQuery = `
select frame_path
from video_frames
where file_id = $1
  and frame_role = 'understanding'
order by timestamp_ms asc, id asc
limit 6
`

const upsertAnalysisQuery = `
with inserted as (
  insert into analysis_results (
    file_id,
    analysis_type,
    status,
    summary,
    structured_attributes,
    raw_model_output,
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
    $5::jsonb,
    $6::jsonb,
    $7,
    $8,
    $9,
    $10
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

const deleteAITagsQuery = `
delete from file_tags
where file_id = $1 and source = 'ai'
`

const upsertTagQuery = `
insert into tags (
  namespace,
  name,
  display_name,
  is_system,
  is_sensitive
)
values (
  $1,
  $2,
  $3,
  true,
  $4
)
on conflict (namespace, name) do update
set
  display_name = excluded.display_name,
  is_sensitive = excluded.is_sensitive
`

const upsertFileTagQuery = `
insert into file_tags (
  file_id,
  tag_id,
  source,
  confidence,
  evidence
)
select
  $1,
  t.id,
  'ai',
  $4,
  $5::jsonb
from tags t
where t.namespace = $2 and t.name = $3
on conflict (file_id, tag_id, source) do update
set
  confidence = excluded.confidence,
  evidence = excluded.evidence
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

type sqlRowsQueryer struct {
	db SQLDB
}

func (q sqlRowsQueryer) QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error) {
	return q.db.QueryContext(ctx, query, args...)
}
