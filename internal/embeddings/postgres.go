package embeddings

import (
	"context"
	"database/sql"
)

type RowScanner interface {
	Scan(dest ...any) error
}

type RowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type RowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
}

type RowsQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error)
}

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type SQLDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type PostgresStore struct {
	Queryer RowQueryer
	Rows    RowsQueryer
	Execer  Execer
}

func NewPostgresStoreFromDB(db SQLDB, execer Execer) PostgresStore {
	return PostgresStore{
		Queryer: sqlRowQueryer{db: db},
		Rows:    sqlRowsQueryer{db: db},
		Execer:  execer,
	}
}

func (s PostgresStore) GetImageSource(ctx context.Context, fileID int64) (ImageSource, error) {
	var item ImageSource
	err := s.Queryer.QueryRowContext(ctx, getImageSourceQuery, fileID).Scan(
		&item.FileID,
		&item.FilePath,
		&item.PHash,
	)
	if err != nil {
		return ImageSource{}, err
	}
	return item, nil
}

func (s PostgresStore) ListVideoFrameSources(ctx context.Context, fileID int64) ([]VideoFrameSource, error) {
	rows, err := s.Rows.QueryContext(ctx, listVideoFrameSourcesQuery, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []VideoFrameSource
	for rows.Next() {
		var item VideoFrameSource
		if err := rows.Scan(&item.FrameID, &item.FileID, &item.FramePath, &item.PHash); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) UpsertFileEmbedding(ctx context.Context, input FileEmbeddingInput) error {
	return s.Execer.ExecContext(
		ctx,
		upsertFileEmbeddingQuery,
		input.FileID,
		input.EmbeddingType,
		input.ModelName,
		input.Vector,
	)
}

func (s PostgresStore) ReplaceFrameEmbeddings(ctx context.Context, fileID int64, embeddingType string, inputs []FrameEmbeddingInput) error {
	if err := s.Execer.ExecContext(ctx, deleteFrameEmbeddingsQuery, fileID, embeddingType); err != nil {
		return err
	}
	for _, input := range inputs {
		if err := s.Execer.ExecContext(
			ctx,
			insertFrameEmbeddingQuery,
			input.FrameID,
			input.EmbeddingType,
			input.ModelName,
			input.Vector,
		); err != nil {
			return err
		}
	}
	return nil
}

const getImageSourceQuery = `
select f.id, coalesce(f.abs_path, '') as file_path, coalesce(ia.phash, '') as phash
from files f
left join image_assets ia on ia.file_id = f.id
where f.id = $1
`

const listVideoFrameSourcesQuery = `
select id, file_id, coalesce(frame_path, '') as frame_path, coalesce(phash, '') as phash
from video_frames
where file_id = $1 and frame_role = 'understanding'
order by timestamp_ms asc, id asc
`

const upsertFileEmbeddingQuery = `
delete from embeddings
where
  file_id = $1
  and embedding_type = $2
  and model_name = $3;

insert into embeddings (
  file_id,
  embedding_type,
  model_name,
  vector
)
values (
  $1,
  $2,
  $3,
  $4::vector
)
`

const deleteFrameEmbeddingsQuery = `
delete from embeddings e
using video_frames vf
where
  e.frame_id = vf.id
  and vf.file_id = $1
  and e.embedding_type = $2
`

const insertFrameEmbeddingQuery = `
insert into embeddings (
  frame_id,
  embedding_type,
  model_name,
  vector
)
values (
  $1,
  $2,
  $3,
  $4::vector
)
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
