package clusters

import (
	"context"
	"database/sql"
)

type Cluster struct {
	ID          int64
	ClusterType string
	Title       string
	Confidence  *float64
	Status      string
	CoverFileID *int64
	MemberCount int64
	StrongMemberCount int64
	TopMemberScore *float64
	PersonVisualCount int64
	GenericVisualCount int64
	TopEvidenceType string
	CreatedAt   string
}

type ClusterMember struct {
	FileID               int64
	FileName             string
	AbsPath              string
	MediaType            string
	Role                 string
	Score                *float64
	QualityTier          string
	HasFace              bool
	SubjectCount         string
	CaptureType          string
	EmbeddingType        string
	EmbeddingProvider    string
	EmbeddingModel       string
	EmbeddingVectorCount int64
}

type ClusterDetail struct {
	Cluster
	PersonVisualCount  int64
	GenericVisualCount int64
	TopEvidenceType    string
	Members            []ClusterMember
}

type ClusterSummary struct {
	ClusterType  string
	Status       string
	ClusterCount int64
	MemberCount  int64
}

type ListOptions struct {
	ClusterType string
	Status      string
	Limit       int
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

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type PostgresStore struct {
	Rows          RowsQueryer
	DetailQueryer DetailQueryer
	Execer        Execer
}

func NewPostgresStoreFromDB(db SQLRowsDB, execer Execer) PostgresStore {
	return PostgresStore{
		Rows:          sqlRowsQueryer{db: db},
		DetailQueryer: sqlDetailQueryer{db: db},
		Execer:        execer,
	}
}

func (s PostgresStore) ListClusters(ctx context.Context, options ListOptions) ([]Cluster, error) {
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.Rows.QueryContext(ctx, listClustersQuery, options.ClusterType, options.Status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Cluster
	for rows.Next() {
		var item Cluster
		var confidence sql.NullFloat64
		var topMemberScore sql.NullFloat64
		var coverFileID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ClusterType, &item.Title, &confidence, &item.Status, &coverFileID, &item.MemberCount, &item.StrongMemberCount, &topMemberScore, &item.PersonVisualCount, &item.GenericVisualCount, &item.TopEvidenceType, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Confidence = float64PtrFromNull(confidence)
		item.TopMemberScore = float64PtrFromNull(topMemberScore)
		item.CoverFileID = int64PtrFromNull(coverFileID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) GetClusterDetail(ctx context.Context, clusterID int64) (ClusterDetail, error) {
	var item ClusterDetail
	var confidence sql.NullFloat64
	var topMemberScore sql.NullFloat64
	var coverFileID sql.NullInt64
	err := s.DetailQueryer.QueryRowContext(ctx, clusterDetailQuery, clusterID).Scan(
		&item.ID,
		&item.ClusterType,
		&item.Title,
		&confidence,
		&item.Status,
		&coverFileID,
		&item.MemberCount,
		&item.StrongMemberCount,
		&topMemberScore,
		&item.CreatedAt,
	)
	if err != nil {
		return ClusterDetail{}, err
	}
	item.Confidence = float64PtrFromNull(confidence)
	item.TopMemberScore = float64PtrFromNull(topMemberScore)
	item.CoverFileID = int64PtrFromNull(coverFileID)

	rows, err := s.Rows.QueryContext(ctx, clusterMembersQuery, clusterID)
	if err != nil {
		return ClusterDetail{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var member ClusterMember
		var score sql.NullFloat64
		if err := rows.Scan(&member.FileID, &member.FileName, &member.AbsPath, &member.MediaType, &member.Role, &score, &member.QualityTier, &member.HasFace, &member.SubjectCount, &member.CaptureType, &member.EmbeddingType, &member.EmbeddingProvider, &member.EmbeddingModel, &member.EmbeddingVectorCount); err != nil {
			return ClusterDetail{}, err
		}
		member.Score = float64PtrFromNull(score)
		item.Members = append(item.Members, member)
	}
	if err := rows.Err(); err != nil {
		return ClusterDetail{}, err
	}
	item.PersonVisualCount, item.GenericVisualCount, item.TopEvidenceType = summarizeEmbeddingEvidence(item.Members)
	return item, nil
}

func summarizeEmbeddingEvidence(members []ClusterMember) (int64, int64, string) {
	var personVisualCount int64
	var genericVisualCount int64
	topEvidenceType := ""
	topEvidenceRank := -1
	for _, member := range members {
		switch member.EmbeddingType {
		case "person_visual":
			personVisualCount++
		case "image_visual", "video_frame_visual":
			genericVisualCount++
		}
		rank := embeddingEvidenceRank(member.EmbeddingType)
		if rank > topEvidenceRank {
			topEvidenceRank = rank
			topEvidenceType = member.EmbeddingType
		}
	}
	return personVisualCount, genericVisualCount, topEvidenceType
}

func embeddingEvidenceRank(embeddingType string) int {
	switch embeddingType {
	case "person_visual":
		return 2
	case "image_visual", "video_frame_visual":
		return 1
	default:
		return 0
	}
}

func (s PostgresStore) SummarizeClusters(ctx context.Context) ([]ClusterSummary, error) {
	rows, err := s.Rows.QueryContext(ctx, clusterSummaryQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ClusterSummary
	for rows.Next() {
		var item ClusterSummary
		if err := rows.Scan(&item.ClusterType, &item.Status, &item.ClusterCount, &item.MemberCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s PostgresStore) UpdateClusterStatus(ctx context.Context, clusterID int64, status string) error {
	return s.Execer.ExecContext(ctx, updateClusterStatusQuery, clusterID, status)
}

const listClustersQuery = `
select
  c.id,
  c.cluster_type,
  coalesce(c.title, '') as title,
  c.confidence,
  c.status,
  c.cover_file_id,
  count(cm.id) as member_count,
  count(cm.id) filter (where cm.score >= 0.80) as strong_member_count,
  max(cm.score) as top_member_score,
  count(distinct cm.file_id) filter (
    where exists (
      select 1
      from embeddings e
      where
        (e.file_id = cm.file_id and e.embedding_type = 'person_visual')
        or (
          e.frame_id in (
            select vf.id
            from video_frames vf
            where vf.file_id = cm.file_id
          )
          and e.embedding_type = 'person_visual'
        )
    )
  ) as person_visual_count,
  count(distinct cm.file_id) filter (
    where exists (
      select 1
      from embeddings e
      where
        (e.file_id = cm.file_id and e.embedding_type = 'image_visual')
        or (
          e.frame_id in (
            select vf.id
            from video_frames vf
            where vf.file_id = cm.file_id
          )
          and e.embedding_type = 'video_frame_visual'
        )
    )
  ) as generic_visual_count,
  case
    when count(distinct cm.file_id) filter (
      where exists (
        select 1
        from embeddings e
        where
          (e.file_id = cm.file_id and e.embedding_type = 'person_visual')
          or (
            e.frame_id in (
              select vf.id
              from video_frames vf
              where vf.file_id = cm.file_id
            )
            and e.embedding_type = 'person_visual'
          )
      )
    ) > 0 then 'person_visual'
    when count(distinct cm.file_id) filter (
      where exists (
        select 1
        from embeddings e
        where
          (e.file_id = cm.file_id and e.embedding_type = 'image_visual')
          or (
            e.frame_id in (
              select vf.id
              from video_frames vf
              where vf.file_id = cm.file_id
            )
            and e.embedding_type = 'video_frame_visual'
          )
      )
    ) > 0 then 'generic_visual'
    else ''
  end as top_evidence_type,
  c.created_at
from clusters c
left join cluster_members cm on cm.cluster_id = c.id
where ($1 = '' or c.cluster_type = $1)
  and ($2 = '' or c.status = $2)
group by c.id
order by c.created_at desc, c.id desc
limit $3
`

const clusterDetailQuery = `
select
  c.id,
  c.cluster_type,
  coalesce(c.title, '') as title,
  c.confidence,
  c.status,
  c.cover_file_id,
  count(cm.id) as member_count,
  count(cm.id) filter (where cm.score >= 0.80) as strong_member_count,
  max(cm.score) as top_member_score,
  c.created_at
from clusters c
left join cluster_members cm on cm.cluster_id = c.id
where c.id = $1
group by c.id
`

const clusterMembersQuery = `
select
  f.id,
  f.file_name,
  f.abs_path,
  f.media_type,
  cm.role,
  cm.score,
  coalesce(quality.quality_tier, '') as quality_tier,
  lower(coalesce(understanding.structured_attributes->>'has_face', '')) in ('true', '1', 'yes') as has_face,
  lower(coalesce(understanding.structured_attributes->>'subject_count', '')) as subject_count,
  lower(coalesce(understanding.structured_attributes->>'capture_type', '')) as capture_type,
  coalesce(embedding.embedding_type, '') as embedding_type,
  coalesce(embedding.provider, '') as embedding_provider,
  coalesce(embedding.model_name, '') as embedding_model,
  coalesce(embedding.vector_count, 0) as embedding_vector_count
from cluster_members cm
join files f on f.id = cm.file_id
left join lateral (
  select ar.quality_tier
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'quality'
  order by ar.created_at desc, ar.id desc
  limit 1
) quality on true
left join lateral (
  select ar.structured_attributes
  from file_current_analysis fca
  join analysis_results ar on ar.id = fca.analysis_result_id
  where fca.file_id = f.id
    and fca.analysis_type = 'understanding'
  order by ar.created_at desc, ar.id desc
  limit 1
) understanding on true
left join lateral (
  select
    e.embedding_type,
    case
      when e.model_name like 'semantic-%' then 'semantic'
      when e.model_name like 'ffmpeg-%' then 'pixel'
      when e.model_name like 'placeholder%' then 'placeholder'
      else 'unknown'
    end as provider,
    e.model_name,
    count(*) as vector_count
  from embeddings e
  where
    (e.file_id = f.id and e.embedding_type in ('person_visual', 'image_visual'))
    or (
      e.frame_id in (
        select id
        from video_frames
        where file_id = f.id
      )
      and e.embedding_type in ('person_visual', 'video_frame_visual')
    )
  group by e.embedding_type, provider, e.model_name
  order by
    case when e.embedding_type = 'person_visual' then 0 else 1 end asc,
    count(*) desc,
    e.model_name desc
  limit 1
) embedding on true
where cm.cluster_id = $1
order by
  case cm.role
    when 'cover' then 0
    when 'best_quality' then 1
    else 2
  end,
  cm.score desc nulls last,
  cm.id asc
`

const clusterSummaryQuery = `
select
  c.cluster_type,
  c.status,
  count(distinct c.id) as cluster_count,
  count(cm.id) as member_count
from clusters c
left join cluster_members cm on cm.cluster_id = c.id
group by c.cluster_type, c.status
order by c.cluster_type asc, c.status asc
`

const updateClusterStatusQuery = `
update clusters
set
  status = $2,
  updated_at = now()
where id = $1
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

func float64PtrFromNull(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func int64PtrFromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
