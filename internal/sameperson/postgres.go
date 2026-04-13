package sameperson

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
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
		&item.HasFace,
		&item.SubjectCount,
		&item.CaptureType,
		&item.ImageEmbedding,
		&item.ImageEmbeddingType,
		&item.ImageEmbeddingModel,
	)
	if err != nil {
		return FileContext{}, err
	}
	if item.MediaType == "video" && s.RowsQueryer != nil {
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

func (s PostgresStore) ListPersonTags(ctx context.Context, fileID int64) ([]PersonTag, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listPersonTagsQuery, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PersonTag
	for rows.Next() {
		var item PersonTag
		if err := rows.Scan(&item.Name, &item.Source); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) ListAutoPersonTags(ctx context.Context, fileID int64) ([]PersonTag, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listAutoPersonTagsQuery, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PersonTag
	for rows.Next() {
		var item PersonTag
		if err := rows.Scan(&item.Name, &item.Source); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) ListFilesWithPersonTag(ctx context.Context, tagName string) ([]PersonCandidateFile, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listFilesWithPersonTagQuery, tagName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PersonCandidateFile
	for rows.Next() {
		var item PersonCandidateFile
		if err := rows.Scan(&item.FileID, &item.ParentPath, &item.FileName, &item.MediaType, &item.ModTime, &item.DurationMS, &item.Width, &item.Height, &item.HasFace, &item.SubjectCount, &item.CaptureType, &item.ImageEmbedding, &item.ImageEmbeddingType, &item.ImageEmbeddingModel); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	videoIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item.MediaType == "video" {
			videoIDs = append(videoIDs, item.FileID)
		}
	}
	if len(videoIDs) > 0 {
		frameEmbeddings, frameTypes, frameModels, err := s.listVideoFrameEmbeddings(ctx, videoIDs)
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

func (s PostgresStore) ListFilesWithAutoPersonTag(ctx context.Context, tagName string) ([]PersonCandidateFile, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listFilesWithAutoPersonTagQuery, tagName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PersonCandidateFile
	for rows.Next() {
		var item PersonCandidateFile
		if err := rows.Scan(&item.FileID, &item.ParentPath, &item.FileName, &item.MediaType, &item.ModTime, &item.DurationMS, &item.Width, &item.Height, &item.HasFace, &item.SubjectCount, &item.CaptureType, &item.ImageEmbedding, &item.ImageEmbeddingType, &item.ImageEmbeddingModel); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	videoIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item.MediaType == "video" {
			videoIDs = append(videoIDs, item.FileID)
		}
	}
	if len(videoIDs) > 0 {
		frameEmbeddings, frameTypes, frameModels, err := s.listVideoFrameEmbeddings(ctx, videoIDs)
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

func (s PostgresStore) UpsertSamePersonCluster(ctx context.Context, title string, files []PersonCandidateFile) error {
	clusterID, err := s.lookupOrCreateCluster(ctx, title)
	if err != nil {
		return err
	}
	coverFileID := files[0].FileID
	if err := s.Execer.ExecContext(ctx, updateClusterQuery, clusterID, title, coverFileID); err != nil {
		return err
	}
	if err := s.Execer.ExecContext(ctx, deleteClusterMembersQuery, clusterID); err != nil {
		return err
	}
	for index, file := range files {
		role := "member"
		if index == 0 {
			role = "cover"
		}
		if err := s.Execer.ExecContext(ctx, insertClusterMemberQuery, clusterID, file.FileID, file.Score, role); err != nil {
			return err
		}
	}
	return nil
}

func (s PostgresStore) DeactivateSamePersonCluster(ctx context.Context, title string) error {
	clusterID, err := s.lookupCluster(ctx, title)
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

func (s PostgresStore) lookupOrCreateCluster(ctx context.Context, title string) (int64, error) {
	clusterID, err := s.lookupCluster(ctx, title)
	if err == nil {
		return clusterID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	if err := s.RowQueryer.QueryRowContext(ctx, createClusterQuery, title).Scan(&clusterID); err != nil {
		return 0, err
	}
	return clusterID, nil
}

func (s PostgresStore) lookupCluster(ctx context.Context, title string) (int64, error) {
	var clusterID int64
	err := s.RowQueryer.QueryRowContext(ctx, findClusterQuery, title).Scan(&clusterID)
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
  lower(coalesce(fa.structured_attributes->>'has_face', '')) in ('true', '1', 'yes') as has_face,
  lower(coalesce(fa.structured_attributes->>'subject_count', '')) as subject_count,
  lower(coalesce(fa.structured_attributes->>'capture_type', '')) as capture_type,
  coalesce(ie.vector::text, '') as image_embedding,
  coalesce(ie.embedding_type, '') as image_embedding_type,
  coalesce(ie.model_name, '') as image_embedding_model
from files
left join image_assets ia on ia.file_id = files.id
left join video_assets va on va.file_id = files.id
left join lateral (
  select ar.structured_attributes
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = files.id
    and fca.analysis_type = 'understanding'
  order by ar.created_at desc, ar.id desc
  limit 1
) fa on true
left join lateral (
  select e.vector, e.embedding_type, e.model_name
  from embeddings e
  where e.file_id = files.id
    and e.embedding_type in ('person_visual', 'image_visual')
  order by
    case when e.embedding_type = 'person_visual' then 0 else 1 end asc,
    e.id desc
  limit 1
) ie on true
where id = $1
`

const listPersonTagsQuery = `
select distinct
  t.name,
  ft.source
from file_tags ft
join tags t on t.id = ft.tag_id
where ft.file_id = $1
  and t.namespace = 'person'
order by t.name asc, ft.source asc
`

const listAutoPersonTagsQuery = `
with auto_signals as (
  select distinct
    t.name as signal_name
  from file_tags ft
  join tags t on t.id = ft.tag_id
  where ft.file_id = $1
    and t.namespace in ('content', 'sensitive')
    and (
      t.name ilike '%单人%'
      or t.name ilike '%多人%'
      or t.name ilike '%情侣%'
      or t.name ilike '%写真%'
      or t.name ilike '%自拍%'
      or t.name ilike '%portrait%'
      or t.name ilike '%selfie%'
      or t.name ilike '%做爱%'
      or t.name ilike '%口交%'
      or t.name ilike '%av%'
    )
  union
  select '__auto:subject_count:single' as signal_name
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = $1
    and fca.analysis_type = 'understanding'
    and lower(coalesce(ar.structured_attributes->>'subject_count', '')) in ('single', '单人', '1', 'one')
  union
  select '__auto:has_face:true' as signal_name
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = $1
    and fca.analysis_type = 'understanding'
    and lower(coalesce(ar.structured_attributes->>'has_face', '')) in ('true', '1', 'yes')
  union
  select '__auto:capture_type:selfie' as signal_name
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = $1
    and fca.analysis_type = 'understanding'
    and lower(coalesce(ar.structured_attributes->>'capture_type', '')) in ('selfie', '自拍')
)
select
  signal_name,
  'auto' as source
from auto_signals
order by signal_name asc
`

const listFilesWithPersonTagQuery = `
select distinct
  ft.file_id,
  coalesce(f.parent_path, '') as parent_path,
  coalesce(f.file_name, '') as file_name,
  coalesce(f.media_type, '') as media_type,
  f.mtime,
  coalesce(va.duration_ms, 0) as duration_ms,
  coalesce(ia.width, 0) as width,
  coalesce(ia.height, 0) as height,
  lower(coalesce(ar.structured_attributes->>'has_face', '')) in ('true', '1', 'yes') as has_face,
  lower(coalesce(ar.structured_attributes->>'subject_count', '')) as subject_count,
  lower(coalesce(ar.structured_attributes->>'capture_type', '')) as capture_type,
  coalesce(ie.vector::text, '') as image_embedding,
  coalesce(ie.embedding_type, '') as image_embedding_type,
  coalesce(ie.model_name, '') as image_embedding_model
from file_tags ft
join tags t on t.id = ft.tag_id
join files f on f.id = ft.file_id
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select e.vector, e.embedding_type, e.model_name
  from embeddings e
  where e.file_id = f.id
    and e.embedding_type in ('person_visual', 'image_visual')
  order by
    case when e.embedding_type = 'person_visual' then 0 else 1 end asc,
    e.id desc
  limit 1
) ie on true
where t.namespace = 'person'
  and t.name = $1
  and f.status = 'active'
order by ft.file_id asc
`

const listFilesWithAutoPersonTagQuery = `
select distinct
  f.id as file_id,
  coalesce(f.parent_path, '') as parent_path,
  coalesce(f.file_name, '') as file_name,
  coalesce(f.media_type, '') as media_type,
  f.mtime,
  coalesce(va.duration_ms, 0) as duration_ms,
  coalesce(ia.width, 0) as width,
  coalesce(ia.height, 0) as height,
  lower(coalesce(ar.structured_attributes->>'has_face', '')) in ('true', '1', 'yes') as has_face,
  lower(coalesce(ar.structured_attributes->>'subject_count', '')) as subject_count,
  lower(coalesce(ar.structured_attributes->>'capture_type', '')) as capture_type,
  coalesce(ie.vector::text, '') as image_embedding,
  coalesce(ie.embedding_type, '') as image_embedding_type,
  coalesce(ie.model_name, '') as image_embedding_model
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select e.vector, e.embedding_type, e.model_name
  from embeddings e
  where e.file_id = f.id
    and e.embedding_type in ('person_visual', 'image_visual')
  order by
    case when e.embedding_type = 'person_visual' then 0 else 1 end asc,
    e.id desc
  limit 1
) ie on true
left join file_tags ft on ft.file_id = f.id
left join tags t on t.id = ft.tag_id
left join file_current_analysis fca on fca.file_id = f.id and fca.analysis_type = 'understanding'
left join analysis_results ar on ar.id = fca.analysis_result_id
where f.status = 'active'
  and (
    (
      $1 not like '__auto:%'
      and t.name = $1
      and t.namespace in ('content', 'sensitive')
    )
    or (
      $1 = '__auto:subject_count:single'
      and lower(coalesce(ar.structured_attributes->>'subject_count', '')) in ('single', '单人', '1', 'one')
    )
    or (
      $1 = '__auto:has_face:true'
      and lower(coalesce(ar.structured_attributes->>'has_face', '')) in ('true', '1', 'yes')
    )
    or (
      $1 = '__auto:capture_type:selfie'
      and lower(coalesce(ar.structured_attributes->>'capture_type', '')) in ('selfie', '自拍')
    )
  )
order by f.id asc
`

const findClusterQuery = `
select id
from clusters
where cluster_type = 'same_person'
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
  'same_person',
  $1,
  0.55,
  'candidate'
)
returning id
`

const updateClusterQuery = `
update clusters
set
  title = $2,
  confidence = 0.55,
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
  $3,
  $4
)
`

const listPersonVideoFrameEmbeddingsBaseQuery = `
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
    and embedded.embedding_type in ('person_visual', 'video_frame_visual')
  order by
    case when embedded.embedding_type = 'person_visual' then 0 else 1 end asc,
    embedded.id desc
  limit 1
) e on true
where vf.frame_role = 'understanding'
  and vf.file_id in (%s)
order by vf.file_id asc, vf.timestamp_ms asc, vf.id asc
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

func (s PostgresStore) listVideoFrameEmbeddings(ctx context.Context, fileIDs []int64) (map[int64][]string, map[int64]string, map[int64]string, error) {
	if len(fileIDs) == 0 {
		return map[int64][]string{}, map[int64]string{}, map[int64]string{}, nil
	}
	query, args := buildPersonVideoFrameEmbeddingsQuery(fileIDs)
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

func buildPersonVideoFrameEmbeddingsQuery(fileIDs []int64) (string, []any) {
	args := make([]any, 0, len(fileIDs))
	placeholders := make([]string, 0, len(fileIDs))
	for index, fileID := range fileIDs {
		args = append(args, fileID)
		placeholders = append(placeholders, "$"+itoa(index+1))
	}
	return sprintf(listPersonVideoFrameEmbeddingsBaseQuery, placeholders), args
}

func sprintf(format string, placeholders []string) string {
	return fmt.Sprintf(format, strings.Join(placeholders, ", "))
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
