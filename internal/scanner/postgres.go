package scanner

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

func (s PostgresStore) GetVolume(ctx context.Context, volumeID int64) (Volume, error) {
	var volume Volume
	err := s.Queryer.QueryRowContext(ctx, getVolumeQuery, volumeID).Scan(&volume.ID, &volume.MountPath)
	if err != nil {
		return Volume{}, err
	}
	return volume, nil
}

func (s PostgresStore) TouchVolume(ctx context.Context, volumeID int64) error {
	return s.Execer.ExecContext(ctx, touchVolumeQuery, volumeID)
}

func (s PostgresStore) UpsertFile(ctx context.Context, record FileRecord) (UpsertResult, error) {
	var result UpsertResult
	err := s.Queryer.QueryRowContext(
		ctx,
		upsertFileQuery,
		record.VolumeID,
		record.AbsPath,
		record.ParentPath,
		record.FileName,
		record.Extension,
		record.MediaType,
		record.SizeBytes,
		record.ModTime,
	).Scan(&result.FileID, &result.Changed)
	if err != nil {
		return UpsertResult{}, err
	}
	return result, nil
}

func (s PostgresStore) MarkMissingFiles(ctx context.Context, volumeID int64, seenPaths []string) error {
	return s.Execer.ExecContext(ctx, markMissingFilesQuery, volumeID, seenPaths)
}

const getVolumeQuery = `
select id, mount_path
from volumes
where id = $1
`

const touchVolumeQuery = `
update volumes
set
  is_online = true,
  last_seen_at = now(),
  updated_at = now()
where id = $1
`

const upsertFileQuery = `
with existing as (
  select
    id,
    parent_path,
    file_name,
    extension,
    media_type,
    size_bytes,
    mtime,
    status
  from files
  where volume_id = $1 and abs_path = $2
),
upserted as (
  insert into files (
    volume_id,
    abs_path,
    parent_path,
    file_name,
    extension,
    media_type,
    size_bytes,
    mtime,
    status
  )
  values (
    $1, $2, $3, $4, $5, $6, $7, $8, 'active'
  )
  on conflict (volume_id, abs_path) do update
  set
    parent_path = excluded.parent_path,
    file_name = excluded.file_name,
    extension = excluded.extension,
    media_type = excluded.media_type,
    size_bytes = excluded.size_bytes,
    mtime = excluded.mtime,
    status = 'active',
    updated_at = now()
  returning id
),
history as (
  insert into file_path_history (
  file_id,
  volume_id,
  abs_path,
  event_type
)
select
  id,
  $1,
  $2,
  'discovered'
from upserted
),
result as (
  select
    id,
    case
      when not exists (select 1 from existing) then true
      when exists (
        select 1
        from existing
        where
          parent_path is distinct from $3
          or file_name is distinct from $4
          or extension is distinct from $5
          or media_type is distinct from $6
          or size_bytes is distinct from $7
          or mtime is distinct from $8
          or status is distinct from 'active'
      ) then true
      else false
    end as changed
  from upserted
)
select id, changed
from result
`

const markMissingFilesQuery = `
with updated as (
  update files
  set
    status = 'missing',
    updated_at = now()
  where
    volume_id = $1
    and status = 'active'
    and not (abs_path = any($2))
  returning id, abs_path
)
insert into file_path_history (
  file_id,
  volume_id,
  abs_path,
  event_type
)
select
  id,
  $1,
  abs_path,
  'missing'
from updated
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}
