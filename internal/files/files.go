package files

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type File struct {
	ID           int64
	VolumeID     int64
	AbsPath      string
	FileName     string
	MediaType    string
	Status       string
	SizeBytes    int64
	UpdatedAt    string
	Width        *int
	Height       *int
	DurationMS   *int64
	Format       string
	Container    string
	FPS          *float64
	Bitrate      *int64
	VideoCodec   string
	AudioCodec   string
	QualityScore *float64
	QualityTier  string
	ReviewAction string
	TagNames     []string
	HasPreview   bool
	ThumbnailPath string
}

type PathHistory struct {
	AbsPath   string
	EventType string
	SeenAt    string
}

type CurrentAnalysis struct {
	AnalysisType string
	Status       string
	Summary      string
	QualityScore *float64
	QualityTier  string
	CreatedAt    string
}

type FileTag struct {
	Namespace   string
	Name        string
	DisplayName string
	Source      string
	Confidence  *float64
}

type ReviewAction struct {
	ActionType string
	Note       string
	CreatedAt  string
}

type EmbeddingInfo struct {
	EmbeddingType string
	Provider      string
	ModelName     string
	VectorCount   int64
}

type VideoFrame struct {
	TimestampMS int64
	FrameRole   string
	PHash       string
}

type FileCluster struct {
	ClusterID   int64
	ClusterType string
	Title       string
	Status      string
}

type FileContent struct {
	AbsPath   string
	FileName  string
	MediaType string
	UpdatedAt time.Time
}

type FileDetail struct {
	File
	PathHistory     []PathHistory
	CurrentAnalyses []CurrentAnalysis
	Tags            []FileTag
	ReviewActions   []ReviewAction
	Clusters        []FileCluster
	Embeddings      []EmbeddingInfo
	VideoFrames     []VideoFrame
}

type ListOptions struct {
	Limit         int
	Offset        int
	Query         string
	MediaType     string
	QualityTier   string
	ReviewAction  string
	Status        string
	VolumeID      int64
	TagNamespace  string
	Tag           string
	ClusterType   string
	ClusterStatus string
	Sort          string
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

type RowScanner interface {
	Scan(dest ...any) error
}

type DetailQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
}

type SQLRowsDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type PostgresStore struct {
	Rows          RowsQueryer
	DetailQueryer DetailQueryer
}

func NewPostgresStoreFromDB(db SQLRowsDB) PostgresStore {
	return PostgresStore{
		Rows:          sqlRowsQueryer{db: db},
		DetailQueryer: sqlDetailQueryer{db: db},
	}
}

func (s PostgresStore) ListFiles(ctx context.Context, options ListOptions) ([]File, error) {
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := options.Offset
	if offset < 0 {
		offset = 0
	}
	orderBy, err := resolveOrderBy(options.Sort)
	if err != nil {
		return nil, err
	}

	rows, err := s.Rows.QueryContext(ctx, fmt.Sprintf(listFilesQuery, orderBy), options.Query, options.MediaType, options.QualityTier, options.ReviewAction, options.Status, options.VolumeID, options.TagNamespace, options.Tag, options.ClusterType, options.ClusterStatus, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []File
	for rows.Next() {
		var item File
		var width sql.NullInt64
		var height sql.NullInt64
	var durationMS sql.NullInt64
	var format sql.NullString
	var container sql.NullString
	var fps sql.NullFloat64
	var bitrate sql.NullInt64
	var videoCodec sql.NullString
	var audioCodec sql.NullString
	var qualityScore sql.NullFloat64
		var qualityTier sql.NullString
		var reviewAction sql.NullString
		var tagsText sql.NullString
		var hasPreview bool
		var thumbnailPath sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.VolumeID,
			&item.AbsPath,
			&item.FileName,
			&item.MediaType,
			&item.Status,
			&item.SizeBytes,
			&item.UpdatedAt,
			&width,
			&height,
			&durationMS,
			&format,
			&container,
			&fps,
			&bitrate,
			&videoCodec,
			&audioCodec,
			&qualityScore,
			&qualityTier,
			&reviewAction,
			&tagsText,
			&hasPreview,
			&thumbnailPath,
		); err != nil {
			return nil, err
		}
		item.Width = intPtrFromNull(width)
		item.Height = intPtrFromNull(height)
		item.DurationMS = int64PtrFromNull(durationMS)
		item.Format = stringFromNull(format)
		item.Container = stringFromNull(container)
		item.FPS = float64PtrFromNull(fps)
		item.Bitrate = int64PtrFromNull(bitrate)
		item.VideoCodec = stringFromNull(videoCodec)
		item.AudioCodec = stringFromNull(audioCodec)
		item.QualityScore = float64PtrFromNull(qualityScore)
		item.QualityTier = stringFromNull(qualityTier)
		item.ReviewAction = stringFromNull(reviewAction)
		item.TagNames = splitTagSummary(tagsText)
		item.HasPreview = hasPreview
		item.ThumbnailPath = stringFromNull(thumbnailPath)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) GetFileDetail(ctx context.Context, fileID int64) (FileDetail, error) {
	var item FileDetail
	var width sql.NullInt64
	var height sql.NullInt64
	var durationMS sql.NullInt64
	var format sql.NullString
	var container sql.NullString
	var qualityTier sql.NullString
	var fps sql.NullFloat64
	var bitrate sql.NullInt64
	var videoCodec sql.NullString
	var audioCodec sql.NullString

	err := s.DetailQueryer.QueryRowContext(ctx, fileDetailQuery, fileID).Scan(
		&item.ID,
		&item.VolumeID,
		&item.AbsPath,
		&item.FileName,
		&item.MediaType,
		&item.Status,
		&item.SizeBytes,
		&item.UpdatedAt,
		&width,
		&height,
		&durationMS,
		&format,
		&container,
		&qualityTier,
		&fps,
		&bitrate,
		&videoCodec,
		&audioCodec,
	)
	if err != nil {
		return FileDetail{}, err
	}
	item.Width = intPtrFromNull(width)
	item.Height = intPtrFromNull(height)
	item.DurationMS = int64PtrFromNull(durationMS)
	item.Format = stringFromNull(format)
	item.Container = stringFromNull(container)
	item.QualityTier = stringFromNull(qualityTier)
	item.FPS = float64PtrFromNull(fps)
	item.Bitrate = int64PtrFromNull(bitrate)
	item.VideoCodec = stringFromNull(videoCodec)
	item.AudioCodec = stringFromNull(audioCodec)

	rows, err := s.Rows.QueryContext(ctx, filePathHistoryQuery, fileID)
	if err != nil {
		return FileDetail{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var history PathHistory
		if err := rows.Scan(&history.AbsPath, &history.EventType, &history.SeenAt); err != nil {
			return FileDetail{}, err
		}
		item.PathHistory = append(item.PathHistory, history)
	}
	if err := rows.Err(); err != nil {
		return FileDetail{}, err
	}

	analysisRows, err := s.Rows.QueryContext(ctx, fileCurrentAnalysisQuery, fileID)
	if err != nil {
		return FileDetail{}, err
	}
	defer analysisRows.Close()

	for analysisRows.Next() {
		var analysis CurrentAnalysis
		var qualityScore sql.NullFloat64
		var qualityTier sql.NullString
		if err := analysisRows.Scan(&analysis.AnalysisType, &analysis.Status, &analysis.Summary, &qualityScore, &qualityTier, &analysis.CreatedAt); err != nil {
			return FileDetail{}, err
		}
		analysis.QualityScore = float64PtrFromNull(qualityScore)
		analysis.QualityTier = stringFromNull(qualityTier)
		item.CurrentAnalyses = append(item.CurrentAnalyses, analysis)
	}
	if err := analysisRows.Err(); err != nil {
		return FileDetail{}, err
	}

	tagRows, err := s.Rows.QueryContext(ctx, fileTagsQuery, fileID)
	if err != nil {
		return FileDetail{}, err
	}
	defer tagRows.Close()

	for tagRows.Next() {
		var tag FileTag
		var confidence sql.NullFloat64
		if err := tagRows.Scan(&tag.Namespace, &tag.Name, &tag.DisplayName, &tag.Source, &confidence); err != nil {
			return FileDetail{}, err
		}
		tag.Confidence = float64PtrFromNull(confidence)
		item.Tags = append(item.Tags, tag)
	}
	if err := tagRows.Err(); err != nil {
		return FileDetail{}, err
	}

	reviewRows, err := s.Rows.QueryContext(ctx, fileReviewActionsQuery, fileID)
	if err != nil {
		return FileDetail{}, err
	}
	defer reviewRows.Close()

	for reviewRows.Next() {
		var action ReviewAction
		if err := reviewRows.Scan(&action.ActionType, &action.Note, &action.CreatedAt); err != nil {
			return FileDetail{}, err
		}
		item.ReviewActions = append(item.ReviewActions, action)
	}
	if err := reviewRows.Err(); err != nil {
		return FileDetail{}, err
	}

	clusterRows, err := s.Rows.QueryContext(ctx, fileClustersQuery, fileID)
	if err != nil {
		return FileDetail{}, err
	}
	defer clusterRows.Close()

	for clusterRows.Next() {
		var cluster FileCluster
		if err := clusterRows.Scan(&cluster.ClusterID, &cluster.ClusterType, &cluster.Title, &cluster.Status); err != nil {
			return FileDetail{}, err
		}
		item.Clusters = append(item.Clusters, cluster)
	}
	if err := clusterRows.Err(); err != nil {
		return FileDetail{}, err
	}

	embeddingRows, err := s.Rows.QueryContext(ctx, fileEmbeddingsQuery, fileID)
	if err != nil {
		return FileDetail{}, err
	}
	defer embeddingRows.Close()

	for embeddingRows.Next() {
		var embedding EmbeddingInfo
		if err := embeddingRows.Scan(&embedding.EmbeddingType, &embedding.Provider, &embedding.ModelName, &embedding.VectorCount); err != nil {
			return FileDetail{}, err
		}
		item.Embeddings = append(item.Embeddings, embedding)
	}
	if err := embeddingRows.Err(); err != nil {
		return FileDetail{}, err
	}

	videoFrameRows, err := s.Rows.QueryContext(ctx, fileVideoFramesQuery, fileID)
	if err != nil {
		return FileDetail{}, err
	}
	defer videoFrameRows.Close()

	for videoFrameRows.Next() {
		var frame VideoFrame
		if err := videoFrameRows.Scan(&frame.TimestampMS, &frame.FrameRole, &frame.PHash); err != nil {
			return FileDetail{}, err
		}
		item.VideoFrames = append(item.VideoFrames, frame)
	}
	if err := videoFrameRows.Err(); err != nil {
		return FileDetail{}, err
	}
	return item, nil
}

func (s PostgresStore) GetFileContent(ctx context.Context, fileID int64) (FileContent, error) {
	var item FileContent
	err := s.DetailQueryer.QueryRowContext(ctx, fileContentQuery, fileID).Scan(
		&item.AbsPath,
		&item.FileName,
		&item.MediaType,
		&item.UpdatedAt,
	)
	if err != nil {
		return FileContent{}, err
	}
	return item, nil
}

func (s PostgresStore) GetFilePreview(ctx context.Context, fileID int64) (FileContent, error) {
	var item FileContent
	err := s.DetailQueryer.QueryRowContext(ctx, filePreviewQuery, fileID).Scan(
		&item.AbsPath,
		&item.FileName,
		&item.MediaType,
		&item.UpdatedAt,
	)
	if err != nil {
		return FileContent{}, err
	}
	return item, nil
}

func (s PostgresStore) GetVideoFramePreview(ctx context.Context, fileID int64, frameIndex int) (FileContent, error) {
	var item FileContent
	err := s.DetailQueryer.QueryRowContext(ctx, fileVideoFramePreviewQuery, fileID, frameIndex).Scan(
		&item.AbsPath,
		&item.FileName,
		&item.MediaType,
		&item.UpdatedAt,
	)
	if err != nil {
		return FileContent{}, err
	}
	return item, nil
}

const listFilesQuery = `
select
  f.id,
  f.volume_id,
  f.abs_path,
  f.file_name,
  f.media_type,
  f.status,
  f.size_bytes,
  f.updated_at,
  coalesce(ia.width, va.width) as width,
  coalesce(ia.height, va.height) as height,
  va.duration_ms,
  ia.format,
  va.container,
  va.fps,
  va.bitrate,
  va.video_codec,
  va.audio_codec,
  quality.quality_score,
  quality.quality_tier,
  latest_review.action_type,
  coalesce(tag_summary.tag_names, '') as tag_names,
  case
    when f.media_type = 'image' then true
    when coalesce(va.poster_path, '') <> '' then true
    else false
  end as has_preview,
  coalesce(ia.thumbnail_path, '') as thumbnail_path
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
left join search_documents sd on sd.file_id = f.id
left join lateral (
  select
    ar.quality_score,
    ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
left join lateral (
  select ra.action_type
  from review_actions ra
  where ra.file_id = f.id
  order by ra.created_at desc, ra.id desc
  limit 1
) latest_review on true
left join lateral (
  select string_agg(tag_name, '||') as tag_names
  from (
    select t.display_name as tag_name
    from file_tags ft
    join tags t on t.id = ft.tag_id
    where ft.file_id = f.id
    order by
      case ft.source
        when 'human' then 0
        when 'ai' then 1
        else 2
      end,
      t.display_name asc
    limit 3
  ) top_tags
) tag_summary on true
where ($1 = '' or sd.tsv @@ websearch_to_tsquery('simple', $1))
  and ($2 = '' or f.media_type = $2)
  and ($3 = '' or quality.quality_tier = $3)
  and ($4 = '' or latest_review.action_type = $4)
  and ($5 = '' or f.status = $5)
  and (
    $6 = 0 or f.volume_id = $6
  )
  and (
    $8 = ''
    or exists (
      select 1
      from file_tags ft
      join tags t on t.id = ft.tag_id
      where ft.file_id = f.id
        and ($7 = '' or t.namespace = $7)
        and ($8 = '' or t.name = $8 or t.display_name = $8)
    )
  )
  and (
    $9 = ''
    or exists (
      select 1
      from cluster_members cm
      join clusters c on c.id = cm.cluster_id
      where cm.file_id = f.id
        and ($9 = '' or c.cluster_type = $9)
        and ($10 = '' or c.status = $10)
    )
  )
order by %s
limit $11
offset $12
`

const fileDetailQuery = `
select
  f.id,
  f.volume_id,
  f.abs_path,
  f.file_name,
  f.media_type,
  f.status,
  f.size_bytes,
  f.updated_at,
  coalesce(ia.width, va.width) as width,
  coalesce(ia.height, va.height) as height,
  va.duration_ms,
  ia.format,
  va.container,
  quality.quality_tier,
  va.fps,
  va.bitrate,
  va.video_codec,
  va.audio_codec
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
left join lateral (
  select ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
where f.id = $1
`

const fileContentQuery = `
select
  abs_path,
  file_name,
  media_type,
  coalesce(updated_at, now()) as updated_at
from files
where id = $1
`

const filePreviewQuery = `
select
  case
    when f.media_type = 'image' then coalesce(nullif(ia.thumbnail_path, ''), f.abs_path)
    else va.poster_path
  end as abs_path,
  f.file_name,
  case
    when f.media_type = 'image' then f.media_type
    else 'image'
  end as media_type,
  coalesce(f.updated_at, now()) as updated_at
from files f
left join image_assets ia on ia.file_id = f.id
left join video_assets va on va.file_id = f.id
where f.id = $1
  and (
    f.media_type = 'image'
    or coalesce(va.poster_path, '') <> ''
  )
`

const fileVideoFramePreviewQuery = `
select
  vf.frame_path as abs_path,
  f.file_name,
  'image' as media_type,
  coalesce(f.updated_at, now()) as updated_at
from video_frames vf
join files f on f.id = vf.file_id
where vf.file_id = $1
order by
  case vf.frame_role
    when 'understanding' then 0
    else 1
  end,
  vf.timestamp_ms asc,
  vf.id asc
offset $2
limit 1
`

const filePathHistoryQuery = `
select
  abs_path,
  event_type,
  seen_at
from file_path_history
where file_id = $1
order by seen_at desc, id desc
`

const fileCurrentAnalysisQuery = `
select
  ar.analysis_type,
  ar.status,
  coalesce(ar.summary, '') as summary,
  ar.quality_score,
  coalesce(ar.quality_tier, '') as quality_tier,
  ar.created_at
from file_current_analysis fca
join analysis_results ar on ar.id = fca.analysis_result_id
where fca.file_id = $1
order by ar.created_at desc, ar.id desc
`

const fileTagsQuery = `
select
  t.namespace,
  t.name,
  t.display_name,
  ft.source,
  ft.confidence
from file_tags ft
join tags t on t.id = ft.tag_id
where ft.file_id = $1
order by
  case ft.source
    when 'human' then 0
    when 'ai' then 1
    else 2
  end,
  t.namespace asc,
  t.display_name asc
`

const fileReviewActionsQuery = `
select
  action_type,
  coalesce(note, '') as note,
  created_at
from review_actions
where file_id = $1
order by created_at desc, id desc
limit 10
`

const fileClustersQuery = `
select
  c.id,
  c.cluster_type,
  coalesce(c.title, '') as title,
  c.status
from cluster_members cm
join clusters c on c.id = cm.cluster_id
where cm.file_id = $1
order by c.created_at desc, c.id desc
`

const fileEmbeddingsQuery = `
select
  embedding_type,
  case
    when model_name like 'semantic-%' then 'semantic'
    when model_name like 'ffmpeg-%' then 'pixel'
    when model_name like 'placeholder%' then 'placeholder'
    else 'unknown'
  end as provider,
  model_name,
  count(*) as vector_count
from embeddings
where
  (file_id = $1 and embedding_type = 'image_visual')
  or (
    frame_id in (
      select id
      from video_frames
      where file_id = $1
    )
    and embedding_type = 'video_frame_visual'
  )
group by embedding_type, provider, model_name
order by embedding_type asc, model_name desc
`

const fileVideoFramesQuery = `
select
  coalesce(timestamp_ms, 0) as timestamp_ms,
  coalesce(frame_role, '') as frame_role,
  coalesce(phash, '') as phash
from video_frames
where file_id = $1
order by
  case frame_role
    when 'understanding' then 0
    else 1
  end,
  timestamp_ms asc,
  id asc
limit 6
`

type sqlRowsQueryer struct {
	db SQLRowsDB
}

func (q sqlRowsQueryer) QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error) {
	return q.db.QueryContext(ctx, query, args...)
}

type sqlDetailQueryer struct {
	db SQLRowsDB
}

func (q sqlDetailQueryer) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

func intPtrFromNull(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func int64PtrFromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func stringFromNull(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func splitTagSummary(value sql.NullString) []string {
	if !value.Valid || value.String == "" {
		return nil
	}
	parts := strings.Split(value.String, "||")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func float64PtrFromNull(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func resolveOrderBy(raw string) (string, error) {
	switch raw {
	case "", "updated_desc":
		return "f.updated_at desc, f.id desc", nil
	case "size_desc":
		return "f.size_bytes desc, f.id desc", nil
	case "size_asc":
		return "f.size_bytes asc, f.id asc", nil
	case "name_asc":
		return "f.file_name asc, f.id asc", nil
	case "quality_desc":
		return "quality.quality_score desc nulls last, f.updated_at desc, f.id desc", nil
	default:
		return "", fmt.Errorf("unsupported sort %s", raw)
	}
}
