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

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
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

type SQLExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type PostgresStore struct {
	Queryer RowQueryer
	Rows    RowsQueryer
	Execer  Execer
}

func NewPostgresStoreFromDB(db SQLDB, rows SQLRowsDB, exec SQLExecDB) PostgresStore {
	return PostgresStore{
		Queryer: sqlRowQueryer{db: db},
		Rows:    sqlRowsQueryer{db: rows},
		Execer:  sqlExecer{db: exec},
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

func (s PostgresStore) DeleteVolume(ctx context.Context, volumeID int64) error {
	if volumeID <= 0 {
		return errors.New("volume id is required")
	}
	if s.Execer == nil {
		return errors.New("execer is required")
	}
	return s.Execer.ExecContext(ctx, deleteVolumeQuery, volumeID)
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

const deleteVolumeQuery = `
delete from files
where volume_id = $1;

delete from volumes
where id = $1
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

type sqlExecer struct {
	db SQLExecDB
}

func (q sqlExecer) ExecContext(ctx context.Context, query string, args ...any) error {
	_, err := q.db.ExecContext(ctx, query, args...)
	return err
}
