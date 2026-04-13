package volumes

import (
	"context"
	"database/sql"
	"errors"
)

type Volume struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	MountPath   string `json:"mount_path"`
	IsOnline    bool   `json:"is_online"`
}

type CreateVolumeInput struct {
	DisplayName string `json:"display_name"`
	MountPath   string `json:"mount_path"`
}

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

type SQLDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type SQLRowsDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type PostgresStore struct {
	Queryer RowQueryer
	Rows    RowsQueryer
}

func NewPostgresStoreFromDB(db SQLDB, rows SQLRowsDB) PostgresStore {
	return PostgresStore{
		Queryer: sqlRowQueryer{db: db},
		Rows:    sqlRowsQueryer{db: rows},
	}
}

func (s PostgresStore) ListVolumes(ctx context.Context) ([]Volume, error) {
	rows, err := s.Rows.QueryContext(ctx, listVolumesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Volume
	for rows.Next() {
		var item Volume
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.MountPath, &item.IsOnline); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) CreateVolume(ctx context.Context, input CreateVolumeInput) (Volume, error) {
	if input.DisplayName == "" || input.MountPath == "" {
		return Volume{}, errors.New("display_name and mount_path are required")
	}

	var item Volume
	err := s.Queryer.QueryRowContext(ctx, createVolumeQuery, input.DisplayName, input.MountPath).Scan(
		&item.ID,
		&item.DisplayName,
		&item.MountPath,
		&item.IsOnline,
	)
	if err != nil {
		return Volume{}, err
	}
	return item, nil
}

const listVolumesQuery = `
select
  id,
  display_name,
  mount_path,
  is_online
from volumes
order by display_name asc, id asc
`

const createVolumeQuery = `
insert into volumes (
  display_name,
  mount_path,
  is_online
)
values ($1, $2, true)
returning
  id,
  display_name,
  mount_path,
  is_online
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

type sqlRowsQueryer struct {
	db SQLRowsDB
}

func (q sqlRowsQueryer) QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error) {
	return q.db.QueryContext(ctx, query, args...)
}
