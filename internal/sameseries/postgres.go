package sameseries

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type RowScanner interface {
	Scan(dest ...any) error
}

type RowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
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

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type SQLDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type PostgresStore struct {
	RowQueryer  RowQueryer
	RowsQueryer RowsQueryer
	Execer      Execer
}

func NewPostgresStoreFromDB(db SQLDB, execer Execer) PostgresStore {
	return PostgresStore{
		RowQueryer:  sqlRowQueryer{db: db},
		RowsQueryer: sqlRowsQueryer{db: db},
		Execer:      execer,
	}
}

func (s PostgresStore) GetFileContext(ctx context.Context, fileID int64) (FileContext, error) {
	var item FileContext
	err := s.RowQueryer.QueryRowContext(ctx, getFileContextQuery, fileID).Scan(
		&item.FileID,
		&item.ParentPath,
		&item.FileName,
		&item.MediaType,
		&item.ModTime,
		&item.Status,
		&item.DurationMS,
		&item.Width,
		&item.Height,
		&item.CaptureType,
		&item.ImagePHash,
		&item.ImageEmbedding,
		&item.ImageEmbeddingType,
		&item.ImageEmbeddingModel,
	)
	if err != nil {
		return FileContext{}, err
	}
	if item.MediaType == "video" && s.RowsQueryer != nil {
		framePHashes, err := s.listVideoFramePHashes(ctx, []int64{fileID})
		if err != nil {
			return FileContext{}, err
		}
		item.VideoFramePHashes = framePHashes[fileID]
		frameEmbeddings, frameTypes, frameModels, err := s.listVideoFrameEmbeddings(ctx, []int64{fileID})
		if err != nil {
			return FileContext{}, err
		}
		item.VideoFrameEmbeddings = frameEmbeddings[fileID]
		item.VideoFrameEmbeddingType = frameTypes[fileID]
		item.VideoFrameEmbeddingModel = frameModels[fileID]
	}
	return item, nil
}

func (s PostgresStore) ListSeriesCandidateFiles(ctx context.Context, file FileContext, window time.Duration) ([]SeriesCandidateFile, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listSeriesCandidateFilesQuery, file.ParentPath, file.MediaType, file.ModTime.Add(-window), file.ModTime.Add(window))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SeriesCandidateFile
	fileIDs := make([]int64, 0, 8)
	for rows.Next() {
		var item SeriesCandidateFile
		if err := rows.Scan(&item.FileID, &item.ParentPath, &item.FileName, &item.ModTime, &item.DurationMS, &item.Width, &item.Height, &item.CaptureType, &item.ImagePHash, &item.ImageEmbedding, &item.ImageEmbeddingType, &item.ImageEmbeddingModel); err != nil {
			return nil, err
		}
		items = append(items, item)
		fileIDs = append(fileIDs, item.FileID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if file.MediaType == "video" && len(fileIDs) > 0 {
		framePHashes, err := s.listVideoFramePHashes(ctx, fileIDs)
		if err != nil {
			return nil, err
		}
		for index := range items {
			items[index].VideoFramePHashes = framePHashes[items[index].FileID]
		}
		frameEmbeddings, frameTypes, frameModels, err := s.listVideoFrameEmbeddings(ctx, fileIDs)
		if err != nil {
			return nil, err
		}
		for index := range items {
			items[index].VideoFrameEmbeddings = frameEmbeddings[items[index].FileID]
			items[index].VideoFrameEmbeddingType = frameTypes[items[index].FileID]
			items[index].VideoFrameEmbeddingModel = frameModels[items[index].FileID]
		}
	}
	return items, nil
}

func (s PostgresStore) ListNearbySeriesCandidateFiles(ctx context.Context, file FileContext, window time.Duration, limit int) ([]SeriesCandidateFile, error) {
	if limit <= 0 {
		limit = 64
	}
	rows, err := s.RowsQueryer.QueryContext(ctx, listNearbySeriesCandidateFilesQuery, file.MediaType, file.ModTime.Add(-window), file.ModTime.Add(window), file.FileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SeriesCandidateFile
	fileIDs := make([]int64, 0, 8)
	for rows.Next() {
		var item SeriesCandidateFile
		if err := rows.Scan(&item.FileID, &item.ParentPath, &item.FileName, &item.ModTime, &item.DurationMS, &item.Width, &item.Height, &item.CaptureType, &item.ImagePHash, &item.ImageEmbedding, &item.ImageEmbeddingType, &item.ImageEmbeddingModel); err != nil {
			return nil, err
		}
		items = append(items, item)
		fileIDs = append(fileIDs, item.FileID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if file.MediaType == "video" && len(fileIDs) > 0 {
		framePHashes, err := s.listVideoFramePHashes(ctx, fileIDs)
		if err != nil {
			return nil, err
		}
		for index := range items {
			items[index].VideoFramePHashes = framePHashes[items[index].FileID]
		}
		frameEmbeddings, frameTypes, frameModels, err := s.listVideoFrameEmbeddings(ctx, fileIDs)
		if err != nil {
			return nil, err
		}
		for index := range items {
			items[index].VideoFrameEmbeddings = frameEmbeddings[items[index].FileID]
			items[index].VideoFrameEmbeddingType = frameTypes[items[index].FileID]
			items[index].VideoFrameEmbeddingModel = frameModels[items[index].FileID]
		}
	}
	return items, nil
}

func (s PostgresStore) UpsertSameSeriesCluster(ctx context.Context, key string, files []SeriesCandidateFile) error {
	clusterID, err := s.lookupOrCreateCluster(ctx, key)
	if err != nil {
		return err
	}
	coverFileID := files[0].FileID
	if err := s.Execer.ExecContext(ctx, updateClusterQuery, clusterID, key, coverFileID); err != nil {
		return err
	}
	if err := s.Execer.ExecContext(ctx, deleteClusterMembersQuery, clusterID); err != nil {
		return err
	}
	for index, file := range files {
		role := file.Role
		if role == "" {
			role = "member"
			if index == len(files)/2 {
				role = "series_focus"
			}
		}
		if err := s.Execer.ExecContext(ctx, insertClusterMemberQuery, clusterID, file.FileID, role); err != nil {
			return err
		}
	}
	return nil
}

func (s PostgresStore) DeactivateSameSeriesCluster(ctx context.Context, key string) error {
	clusterID, err := s.lookupCluster(ctx, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if err := s.Execer.ExecContext(ctx, deactivateClusterQuery, clusterID); err != nil {
		return err
	}
	return s.Execer.ExecContext(ctx, deleteClusterMembersQuery, clusterID)
}

func (s PostgresStore) lookupOrCreateCluster(ctx context.Context, key string) (int64, error) {
	clusterID, err := s.lookupCluster(ctx, key)
	if err == nil {
		return clusterID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	if err := s.RowQueryer.QueryRowContext(ctx, createClusterQuery, key).Scan(&clusterID); err != nil {
		return 0, err
	}
	return clusterID, nil
}

func (s PostgresStore) lookupCluster(ctx context.Context, key string) (int64, error) {
	var clusterID int64
	err := s.RowQueryer.QueryRowContext(ctx, findClusterQuery, key).Scan(&clusterID)
	return clusterID, err
}

const getFileContextQuery = `
select
  id,
  coalesce(parent_path, '') as parent_path,
  coalesce(file_name, '') as file_name,
  coalesce(media_type, '') as media_type,
  mtime,
  coalesce(status, '') as status,
  coalesce(va.duration_ms, 0) as duration_ms,
  coalesce(ia.width, 0) as width,
  coalesce(ia.height, 0) as height,
  coalesce(understanding.capture_type, '') as capture_type,
  coalesce(ia.phash, '') as image_phash,
  coalesce(ie.vector::text, '') as image_embedding,
  coalesce(ie.embedding_type, '') as image_embedding_type,
  coalesce(ie.model_name, '') as image_embedding_model
from files
left join image_assets ia on ia.file_id = files.id
left join video_assets va on va.file_id = files.id
left join lateral (
  select coalesce(ar.structured_attributes->>'capture_type', '') as capture_type
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = files.id
    and fca.analysis_type = 'understanding'
  order by ar.created_at desc, ar.id desc
  limit 1
) understanding on true
left join lateral (
  select e.vector, e.embedding_type, e.model_name
  from embeddings e
  where e.file_id = files.id
    and e.embedding_type = 'image_visual'
  order by e.id desc
  limit 1
) ie on true
where id = $1
`

const listSeriesCandidateFilesQuery = `
select
  f.id,
  coalesce(f.parent_path, '') as parent_path,
  coalesce(f.file_name, '') as file_name,
  f.mtime,
  coalesce(va.duration_ms, 0) as duration_ms,
  coalesce(ia.width, 0) as width,
  coalesce(ia.height, 0) as height,
  coalesce(understanding.capture_type, '') as capture_type,
  coalesce(ia.phash, '') as image_phash,
  coalesce(ie.vector::text, '') as image_embedding,
  coalesce(ie.embedding_type, '') as image_embedding_type,
  coalesce(ie.model_name, '') as image_embedding_model
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select coalesce(ar.structured_attributes->>'capture_type', '') as capture_type
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'understanding'
  order by ar.created_at desc, ar.id desc
  limit 1
) understanding on true
left join lateral (
  select e.vector, e.embedding_type, e.model_name
  from embeddings e
  where e.file_id = f.id
    and e.embedding_type = 'image_visual'
  order by e.id desc
  limit 1
) ie on true
where f.parent_path = $1
  and f.media_type = $2
  and f.status = 'active'
  and f.mtime between $3 and $4
order by f.mtime asc, f.id asc
`

const listNearbySeriesCandidateFilesQuery = `
select
  f.id,
  coalesce(f.parent_path, '') as parent_path,
  coalesce(f.file_name, '') as file_name,
  f.mtime,
  coalesce(va.duration_ms, 0) as duration_ms,
  coalesce(ia.width, 0) as width,
  coalesce(ia.height, 0) as height,
  coalesce(understanding.capture_type, '') as capture_type,
  coalesce(ia.phash, '') as image_phash,
  coalesce(ie.vector::text, '') as image_embedding,
  coalesce(ie.embedding_type, '') as image_embedding_type,
  coalesce(ie.model_name, '') as image_embedding_model
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select coalesce(ar.structured_attributes->>'capture_type', '') as capture_type
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'understanding'
  order by ar.created_at desc, ar.id desc
  limit 1
) understanding on true
left join lateral (
  select e.vector, e.embedding_type, e.model_name
  from embeddings e
  where e.file_id = f.id
    and e.embedding_type = 'image_visual'
  order by e.id desc
  limit 1
) ie on true
where f.media_type = $1
  and f.status = 'active'
  and f.mtime between $2 and $3
  and f.id <> $4
order by f.mtime asc, f.id asc
limit $5
`

const listSeriesVideoFramePHashesBaseQuery = `
select
  file_id,
  phash
from video_frames
where frame_role = 'understanding'
  and phash is not null
  and phash <> ''
  and file_id in (%s)
order by file_id asc, timestamp_ms asc, id asc
`

const listSeriesVideoFrameEmbeddingsBaseQuery = `
select
  vf.file_id,
  e.vector::text as vector,
  e.embedding_type,
  e.model_name
from video_frames vf
join lateral (
  select embedded.vector, embedded.embedding_type, embedded.model_name
  from embeddings embedded
  where embedded.frame_id = vf.id
    and embedded.embedding_type = 'video_frame_visual'
  order by embedded.id desc
  limit 1
) e on true
where vf.frame_role = 'understanding'
  and vf.file_id in (%s)
order by vf.file_id asc, vf.timestamp_ms asc, vf.id asc
`

const findClusterQuery = `
select id
from clusters
where cluster_type = 'same_series'
  and title = $1
order by id desc
limit 1
`

const createClusterQuery = `
insert into clusters (
  cluster_type,
  title,
  confidence,
  status
)
values (
  'same_series',
  $1,
  0.65,
  'candidate'
)
returning id
`

const updateClusterQuery = `
update clusters
set
  title = $2,
  confidence = 0.65,
  status = 'candidate',
  cover_file_id = $3,
  updated_at = now()
where id = $1
`

const deactivateClusterQuery = `
update clusters
set
  status = 'ignored',
  updated_at = now()
where id = $1
`

const deleteClusterMembersQuery = `
delete from cluster_members
where cluster_id = $1
`

const insertClusterMemberQuery = `
insert into cluster_members (
  cluster_id,
  file_id,
  score,
  role
)
values (
  $1,
  $2,
  0.65,
  $3
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

func (s PostgresStore) listVideoFramePHashes(ctx context.Context, fileIDs []int64) (map[int64][]string, error) {
	if len(fileIDs) == 0 {
		return map[int64][]string{}, nil
	}
	query, args := buildSeriesVideoFramePHashesQuery(fileIDs)
	rows, err := s.RowsQueryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]string, len(fileIDs))
	for rows.Next() {
		var fileID int64
		var phash string
		if err := rows.Scan(&fileID, &phash); err != nil {
			return nil, err
		}
		if phash != "" {
			result[fileID] = append(result[fileID], phash)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s PostgresStore) listVideoFrameEmbeddings(ctx context.Context, fileIDs []int64) (map[int64][]string, map[int64]string, map[int64]string, error) {
	if len(fileIDs) == 0 {
		return map[int64][]string{}, map[int64]string{}, map[int64]string{}, nil
	}
	query, args := buildSeriesVideoFrameEmbeddingsQuery(fileIDs)
	rows, err := s.RowsQueryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	result := make(map[int64][]string, len(fileIDs))
	types := make(map[int64]string, len(fileIDs))
	models := make(map[int64]string, len(fileIDs))
	for rows.Next() {
		var fileID int64
		var vector string
		var embeddingType string
		var modelName string
		if err := rows.Scan(&fileID, &vector, &embeddingType, &modelName); err != nil {
			return nil, nil, nil, err
		}
		if vector != "" {
			result[fileID] = append(result[fileID], vector)
		}
		if types[fileID] == "" && embeddingType != "" {
			types[fileID] = embeddingType
		}
		if models[fileID] == "" && modelName != "" {
			models[fileID] = modelName
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	return result, types, models, nil
}

func buildSeriesVideoFramePHashesQuery(fileIDs []int64) (string, []any) {
	args := make([]any, 0, len(fileIDs))
	placeholders := make([]string, 0, len(fileIDs))
	for index, fileID := range fileIDs {
		args = append(args, fileID)
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
	}
	return fmt.Sprintf(listSeriesVideoFramePHashesBaseQuery, strings.Join(placeholders, ", ")), args
}

func buildSeriesVideoFrameEmbeddingsQuery(fileIDs []int64) (string, []any) {
	args := make([]any, 0, len(fileIDs))
	placeholders := make([]string, 0, len(fileIDs))
	for index, fileID := range fileIDs {
		args = append(args, fileID)
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
	}
	return fmt.Sprintf(listSeriesVideoFrameEmbeddingsBaseQuery, strings.Join(placeholders, ", ")), args
}
