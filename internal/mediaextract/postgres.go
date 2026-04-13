package mediaextract

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

func (s PostgresStore) GetFile(ctx context.Context, fileID int64) (File, error) {
	var item File
	err := s.Queryer.QueryRowContext(ctx, getFileQuery, fileID).Scan(
		&item.ID,
		&item.AbsPath,
		&item.Extension,
		&item.MediaType,
	)
	if err != nil {
		return File{}, err
	}
	return item, nil
}

func (s PostgresStore) UpsertImageAsset(ctx context.Context, input ImageAssetInput) error {
	return s.Execer.ExecContext(
		ctx,
		upsertImageAssetQuery,
		input.FileID,
		input.Width,
		input.Height,
		nullIfEmpty(input.Format),
		nullIfEmpty(input.Orientation),
		nullIfEmpty(input.PHash),
		nullIfEmpty(input.ThumbnailPath),
	)
}

func (s PostgresStore) UpsertVideoAsset(ctx context.Context, input VideoAssetInput) error {
	return s.Execer.ExecContext(
		ctx,
		upsertVideoAssetQuery,
		input.FileID,
		input.DurationMS,
		input.Width,
		input.Height,
		input.FPS,
		nullIfEmpty(input.Container),
		nullIfEmpty(input.VideoCodec),
		nullIfEmpty(input.AudioCodec),
		input.Bitrate,
		nullIfEmpty(input.PosterPath),
	)
}

func (s PostgresStore) ReplaceVideoFrames(ctx context.Context, fileID int64, frames []VideoFrameInput) error {
	if err := s.Execer.ExecContext(ctx, deleteVideoFramesQuery, fileID); err != nil {
		return err
	}
	for _, frame := range frames {
		if err := s.Execer.ExecContext(ctx, insertVideoFrameQuery, fileID, frame.TimestampMS, frame.FramePath, frame.FrameRole, nullIfEmpty(frame.PHash)); err != nil {
			return err
		}
	}
	return nil
}

const getFileQuery = `
select id, abs_path, extension, media_type
from files
where id = $1
`

const upsertImageAssetQuery = `
insert into image_assets (
  file_id,
  width,
  height,
  format,
  orientation,
  phash,
  thumbnail_path,
  updated_at
)
values (
  $1, $2, $3, $4, $5, $6, $7, now()
)
on conflict (file_id) do update
set
  width = excluded.width,
  height = excluded.height,
  format = excluded.format,
  orientation = excluded.orientation,
  phash = excluded.phash,
  thumbnail_path = excluded.thumbnail_path,
  updated_at = now()
`

const upsertVideoAssetQuery = `
insert into video_assets (
  file_id,
  duration_ms,
  width,
  height,
  fps,
  container,
  video_codec,
  audio_codec,
  bitrate,
  poster_path,
  updated_at
)
values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now()
)
on conflict (file_id) do update
set
  duration_ms = excluded.duration_ms,
  width = excluded.width,
  height = excluded.height,
  fps = excluded.fps,
  container = excluded.container,
  video_codec = excluded.video_codec,
  audio_codec = excluded.audio_codec,
  bitrate = excluded.bitrate,
  poster_path = excluded.poster_path,
  updated_at = now()
`

const deleteVideoFramesQuery = `
delete from video_frames
where file_id = $1
`

const insertVideoFrameQuery = `
insert into video_frames (
  file_id,
  timestamp_ms,
  frame_path,
  frame_role,
  phash
)
values (
  $1, $2, $3, $4, $5
)
`

type sqlRowQueryer struct {
	db SQLDB
}

func (q sqlRowQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
