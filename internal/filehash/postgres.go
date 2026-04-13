package filehash

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
	err := s.Queryer.QueryRowContext(ctx, getFileQuery, fileID).Scan(&item.ID, &item.AbsPath)
	if err != nil {
		return File{}, err
	}
	return item, nil
}

func (s PostgresStore) UpdateHashes(ctx context.Context, input HashInput) error {
	return s.Execer.ExecContext(ctx, updateHashesQuery, input.FileID, input.SHA256, input.QuickHash)
}

const getFileQuery = `
select
  id,
  abs_path
from files
where id = $1
`

const updateHashesQuery = `
update files
set
  sha256 = nullif($2, ''),
  quick_hash = nullif($3, ''),
  updated_at = now()
where id = $1
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}
