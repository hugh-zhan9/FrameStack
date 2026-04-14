package samecontent

import (
	"context"
	"database/sql"
	"fmt"
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

func (s PostgresStore) GetFileHash(ctx context.Context, fileID int64) (FileHash, error) {
	var item FileHash
	err := s.RowQueryer.QueryRowContext(ctx, getFileHashQuery, fileID).Scan(
		&item.FileID,
		&item.MediaType,
		&item.DurationMS,
		&item.Width,
		&item.Height,
		&item.SHA256,
		&item.ImagePHash,
		&item.ImageEmbedding,
		&item.ImageEmbeddingType,
		&item.ImageEmbeddingModel,
	)
	if err != nil {
		return FileHash{}, err
	}
	if item.MediaType == "video" && s.RowsQueryer != nil {
		framePHashes, err := s.listVideoFramePHashes(ctx, fileID)
		if err != nil {
			return FileHash{}, err
		}
		item.VideoFramePHashes = framePHashes
		frameEmbeddings, frameType, frameModel, err := s.listVideoFrameEmbeddings(ctx, fileID)
		if err != nil {
			return FileHash{}, err
		}
		item.VideoFrameEmbeddings = frameEmbeddings
		item.VideoFrameEmbeddingType = frameType
		item.VideoFrameEmbeddingModel = frameModel
	}
	return item, nil
}

func (s PostgresStore) ListDuplicateFiles(ctx context.Context, sha256 string) ([]DuplicateFile, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listDuplicateFilesQuery, sha256)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DuplicateFile
	for rows.Next() {
		var item DuplicateFile
		if err := rows.Scan(&item.FileID, &item.QualityScore, &item.QualityTier, &item.Width, &item.Height, &item.DurationMS, &item.SizeBytes, &item.Bitrate, &item.FPS, &item.Container); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) ListImagePHashCandidates(ctx context.Context, prefix string) ([]ImageCandidate, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listImagePHashCandidatesQuery, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ImageCandidate
	for rows.Next() {
		var item ImageCandidate
		if err := rows.Scan(&item.FileID, &item.PHash, &item.QualityScore, &item.QualityTier, &item.Width, &item.Height, &item.SizeBytes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) ListImageEmbeddingCandidates(ctx context.Context, prefix string, model string) ([]ImageCandidate, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listImageEmbeddingCandidatesQuery, prefix, model, imageEmbeddingDistanceThreshold, 24)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ImageCandidate
	for rows.Next() {
		var item ImageCandidate
		if err := rows.Scan(&item.FileID, &item.Embedding, &item.EmbeddingType, &item.EmbeddingModel, &item.QualityScore, &item.QualityTier, &item.Width, &item.Height, &item.SizeBytes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) ListVideoFramePHashMatches(ctx context.Context, phashes []string) ([]DuplicateFile, error) {
	normalized := uniqueSortedPHashes(phashes)
	if len(normalized) < 2 {
		return nil, nil
	}

	query, args := buildVideoFrameMatchesQuery(normalized)
	rows, err := s.RowsQueryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DuplicateFile
	for rows.Next() {
		var item DuplicateFile
		if err := rows.Scan(&item.FileID, &item.QualityScore, &item.QualityTier, &item.Width, &item.Height, &item.DurationMS, &item.SizeBytes, &item.Bitrate, &item.FPS, &item.Container); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) ListVideoFrameEmbeddingMatches(ctx context.Context, embeddings []string, model string) ([]DuplicateFile, error) {
	normalized := uniqueSortedEmbeddings(embeddings)
	if len(normalized) < 2 {
		return nil, nil
	}

	query, args := buildVideoFrameEmbeddingMatchesQuery(normalized, model)
	rows, err := s.RowsQueryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DuplicateFile
	for rows.Next() {
		var item DuplicateFile
		if err := rows.Scan(&item.FileID, &item.QualityScore, &item.QualityTier, &item.Width, &item.Height, &item.DurationMS, &item.SizeBytes, &item.Bitrate, &item.FPS, &item.Container); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) UpsertSameContentCluster(ctx context.Context, sha256 string, files []DuplicateFile) error {
	clusterID, err := s.lookupOrCreateCluster(ctx, sha256)
	if err != nil {
		return err
	}
	coverFileID := files[0].FileID
	if err := s.Execer.ExecContext(ctx, updateClusterQuery, clusterID, ClusterTitle(sha256), coverFileID); err != nil {
		return err
	}
	if err := s.Execer.ExecContext(ctx, deleteClusterMembersQuery, clusterID); err != nil {
		return err
	}
	for index, file := range files {
		role := file.Role
		if role == "" {
			role = "duplicate_candidate"
			if index == 0 {
				role = "best_quality"
			}
		}
		score := file.Score
		if score <= 0 {
			score = 1
			if role != "best_quality" {
				score = 0.5
			}
		}
		if err := s.Execer.ExecContext(ctx, insertClusterMemberQuery, clusterID, file.FileID, score, role); err != nil {
			return err
		}
	}
	return nil
}

func (s PostgresStore) DeactivateSameContentCluster(ctx context.Context, sha256 string) error {
	clusterID, err := s.lookupCluster(ctx, sha256)
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

func (s PostgresStore) lookupOrCreateCluster(ctx context.Context, sha256 string) (int64, error) {
	clusterID, err := s.lookupCluster(ctx, sha256)
	if err == nil {
		return clusterID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	if err := s.RowQueryer.QueryRowContext(ctx, createClusterQuery, ClusterTitle(sha256)).Scan(&clusterID); err != nil {
		return 0, err
	}
	return clusterID, nil
}

func (s PostgresStore) lookupCluster(ctx context.Context, sha256 string) (int64, error) {
	var clusterID int64
	err := s.RowQueryer.QueryRowContext(ctx, findClusterQuery, ClusterTitle(sha256)).Scan(&clusterID)
	return clusterID, err
}

const getFileHashQuery = `
select
  id,
  media_type,
  coalesce(va.duration_ms, 0) as duration_ms,
  coalesce(ia.width, va.width, 0) as width,
  coalesce(ia.height, va.height, 0) as height,
  coalesce(sha256, '') as sha256,
  coalesce(ia.phash, '') as image_phash,
  coalesce(ie.vector::text, '') as image_embedding,
  coalesce(ie.embedding_type, '') as image_embedding_type,
  coalesce(ie.model_name, '') as image_embedding_model
from files
left join image_assets ia on ia.file_id = files.id
left join video_assets va on va.file_id = files.id
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

const listDuplicateFilesQuery = `
select
  f.id,
  coalesce(quality.quality_score, 0),
  coalesce(quality.quality_tier, ''),
  coalesce(ia.width, va.width, 0),
  coalesce(ia.height, va.height, 0),
  coalesce(va.duration_ms, 0),
  coalesce(f.size_bytes, 0),
  coalesce(va.bitrate, 0),
  coalesce(va.fps, 0),
  coalesce(va.container, '')
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select ar.quality_score, ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
where f.sha256 = $1
  and f.status <> 'trashed'
order by f.id asc
`

const listImagePHashCandidatesQuery = `
select
  f.id,
  ia.phash,
  coalesce(quality.quality_score, 0),
  coalesce(quality.quality_tier, ''),
  coalesce(ia.width, 0),
  coalesce(ia.height, 0),
  coalesce(f.size_bytes, 0)
from files f
join image_assets ia on ia.file_id = f.id
left join lateral (
  select ar.quality_score, ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
where left(ia.phash, 4) = $1
  and f.media_type = 'image'
  and f.status <> 'trashed'
order by f.id asc
`

const listImageEmbeddingCandidatesQuery = `
select
  f.id,
  ie.vector::text as embedding,
  ie.embedding_type,
  ie.model_name,
  coalesce(quality.quality_score, 0),
  coalesce(quality.quality_tier, ''),
  coalesce(ia.width, 0),
  coalesce(ia.height, 0),
  coalesce(f.size_bytes, 0)
from files f
left join image_assets ia on ia.file_id = f.id
left join lateral (
  select ar.quality_score, ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
join lateral (
  select e.vector, e.embedding_type, e.model_name
  from embeddings e
  where e.file_id = f.id
    and e.embedding_type = 'image_visual'
    and e.model_name = $2
  order by e.id desc
  limit 1
) ie on true
where f.media_type = 'image'
  and f.status <> 'trashed'
  and (ie.vector <-> $1::vector) <= $3
order by ie.vector <-> $1::vector asc, f.id asc
limit $4
`

const listVideoFramePHashesQuery = `
select
  phash
from video_frames
where file_id = $1
  and frame_role = 'understanding'
  and phash is not null
  and phash <> ''
order by timestamp_ms asc, id asc
`

const listVideoFrameEmbeddingsQuery = `
select
  e.vector::text as embedding,
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
where vf.file_id = $1
  and vf.frame_role = 'understanding'
order by vf.timestamp_ms asc, vf.id asc
`

const findClusterQuery = `
select id
from clusters
where cluster_type = 'same_content'
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
  'same_content',
  $1,
  1.0,
  'candidate'
)
returning id
`

const updateClusterQuery = `
update clusters
set
  title = $2,
  confidence = 1.0,
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

func (s PostgresStore) listVideoFramePHashes(ctx context.Context, fileID int64) ([]string, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listVideoFramePHashesQuery, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value != "" {
			items = append(items, value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return uniqueSortedPHashes(items), nil
}

func (s PostgresStore) listVideoFrameEmbeddings(ctx context.Context, fileID int64) ([]string, string, string, error) {
	rows, err := s.RowsQueryer.QueryContext(ctx, listVideoFrameEmbeddingsQuery, fileID)
	if err != nil {
		return nil, "", "", err
	}
	defer rows.Close()

	var items []string
	embeddingType := ""
	modelName := ""
	for rows.Next() {
		var value string
		var rowType string
		var rowModel string
		if err := rows.Scan(&value, &rowType, &rowModel); err != nil {
			return nil, "", "", err
		}
		if value != "" {
			items = append(items, value)
		}
		if embeddingType == "" && rowType != "" {
			embeddingType = rowType
		}
		if modelName == "" && rowModel != "" {
			modelName = rowModel
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}
	return uniqueSortedEmbeddings(items), embeddingType, modelName, nil
}

func buildVideoFrameMatchesQuery(phashes []string) (string, []any) {
	args := make([]any, 0, len(phashes)+1)
	placeholders := make([]string, 0, len(phashes))
	for index, phash := range phashes {
		args = append(args, phash)
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
	}
	args = append(args, 2)
	query := fmt.Sprintf(`
select
  f.id,
  coalesce(quality.quality_score, 0),
  coalesce(quality.quality_tier, ''),
  coalesce(va.width, 0),
  coalesce(va.height, 0),
  coalesce(va.duration_ms, 0),
  coalesce(f.size_bytes, 0),
  coalesce(va.bitrate, 0),
  coalesce(va.fps, 0),
  coalesce(va.container, '')
from files f
join video_frames vf on vf.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select ar.quality_score, ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
where f.media_type = 'video'
  and f.status <> 'trashed'
  and vf.frame_role = 'understanding'
  and vf.phash in (%s)
group by f.id
  , quality.quality_score
  , quality.quality_tier
  , va.width
  , va.height
  , va.duration_ms
  , f.size_bytes
  , va.bitrate
  , va.fps
  , va.container
having count(distinct vf.phash) >= $%d
order by f.id asc
`, strings.Join(placeholders, ", "), len(args))
	return query, args
}

func buildVideoFrameEmbeddingMatchesQuery(embeddings []string, model string) (string, []any) {
	args := make([]any, 0, len(embeddings)+3)
	valueRows := make([]string, 0, len(embeddings))
	for index, embedding := range embeddings {
		args = append(args, embedding)
		valueRows = append(valueRows, fmt.Sprintf("($%d::text, %d)", index+1, index+1))
	}
	args = append(args, model, videoEmbeddingDistanceThreshold, 2)
	query := fmt.Sprintf(`
with embedding_inputs(embedding, idx) as (
  values %s
)
select
  f.id,
  coalesce(quality.quality_score, 0),
  coalesce(quality.quality_tier, ''),
  coalesce(va.width, 0),
  coalesce(va.height, 0),
  coalesce(va.duration_ms, 0),
  coalesce(f.size_bytes, 0),
  coalesce(va.bitrate, 0),
  coalesce(va.fps, 0),
  coalesce(va.container, '')
from files f
join video_frames vf on vf.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select ar.quality_score, ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
join lateral (
  select embedded.vector
  from embeddings embedded
  where embedded.frame_id = vf.id
    and embedded.embedding_type = 'video_frame_visual'
    and embedded.model_name = $%d
  order by embedded.id desc
  limit 1
) e on true
join embedding_inputs iv on (e.vector <-> iv.embedding::vector) <= $%d
where f.media_type = 'video'
  and f.status <> 'trashed'
  and vf.frame_role = 'understanding'
group by f.id
  , quality.quality_score
  , quality.quality_tier
  , va.width
  , va.height
  , va.duration_ms
  , f.size_bytes
  , va.bitrate
  , va.fps
  , va.container
having count(distinct iv.idx) >= $%d
order by f.id asc
`, strings.Join(valueRows, ", "), len(embeddings)+1, len(embeddings)+2, len(args))
	return query, args
}
