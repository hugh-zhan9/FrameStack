package trash

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

func (s PostgresStore) GetFile(ctx context.Context, fileID int64) (File, error) {
	var item File
	err := s.Queryer.QueryRowContext(ctx, getFileQuery, fileID).Scan(
		&item.ID,
		&item.AbsPath,
		&item.Status,
		&item.VolumeID,
	)
	if err != nil {
		return File{}, err
	}
	return item, nil
}

func (s PostgresStore) MarkFileTrashed(ctx context.Context, fileID int64) error {
	return s.Execer.ExecContext(ctx, markFileTrashedQuery, fileID)
}

const getFileQuery = `
select
  id,
  abs_path,
  status,
  volume_id
from files
where id = $1
`

const markFileTrashedQuery = `
with updated as (
  update files
  set
    status = 'trashed',
    updated_at = now()
  where id = $1
  returning id
)
insert into review_actions (
  file_id,
  action_type,
  note
)
select
  id,
  'deleted_to_trash',
  'moved to macOS trash'
from updated
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}
