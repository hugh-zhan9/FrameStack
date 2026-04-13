package reveal

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

type SQLDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type PostgresStore struct {
	Queryer RowQueryer
}

func NewPostgresStoreFromDB(db SQLDB) PostgresStore {
	return PostgresStore{
		Queryer: sqlRowQueryer{db: db},
	}
}

func (s PostgresStore) GetFile(ctx context.Context, fileID int64) (File, error) {
	var item File
	err := s.Queryer.QueryRowContext(ctx, getFileQuery, fileID).Scan(
		&item.ID,
		&item.AbsPath,
		&item.Status,
	)
	if err != nil {
		return File{}, err
	}
	return item, nil
}

const getFileQuery = `
select
  id,
  abs_path,
  status
from files
where id = $1
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}
