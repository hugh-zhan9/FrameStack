package clusters_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"idea/internal/clusters"
)

func TestPostgresStoreListClustersReturnsItems(t *testing.T) {
	store := clusters.PostgresStore{
		Rows: &recordingRowsQueryer{
			rows: &staticClusterRows{
				items: []clusters.Cluster{
					{
						ID:          7,
						ClusterType: "same_person",
						Title:       "Candidate person group",
						Confidence:  float64Ptr(0.91),
						Status:      "candidate",
						CoverFileID: int64Ptr(11),
						MemberCount: 3,
						StrongMemberCount: 2,
						TopMemberScore:    float64Ptr(0.97),
						PersonVisualCount: 2,
						GenericVisualCount: 1,
						TopEvidenceType: "person_visual",
						CreatedAt:   "2026-04-09T20:00:00Z",
					},
				},
			},
		},
	}

	items, err := store.ListClusters(context.Background(), clusters.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("expected list clusters to succeed: %v", err)
	}
	if len(items) != 1 || items[0].ClusterType != "same_person" || items[0].MemberCount != 3 || items[0].StrongMemberCount != 2 {
		t.Fatalf("unexpected clusters: %#v", items)
	}
	if items[0].TopMemberScore == nil || *items[0].TopMemberScore != 0.97 {
		t.Fatalf("expected top score, got %#v", items[0])
	}
	if items[0].PersonVisualCount != 2 || items[0].GenericVisualCount != 1 || items[0].TopEvidenceType != "person_visual" {
		t.Fatalf("expected list evidence summary, got %#v", items[0])
	}
}

func TestPostgresStoreListClustersPassesFilters(t *testing.T) {
	queryer := &recordingRowsQueryer{rows: &staticClusterRows{}}
	store := clusters.PostgresStore{Rows: queryer}

	_, err := store.ListClusters(context.Background(), clusters.ListOptions{
		ClusterType: "same_content",
		Status:      "candidate",
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("expected list clusters to succeed: %v", err)
	}
	if len(queryer.args) != 3 || queryer.args[0] != "same_content" || queryer.args[1] != "candidate" || queryer.args[2] != 5 {
		t.Fatalf("unexpected query args: %#v", queryer.args)
	}
	normalized := normalizeSQL(queryer.query)
	for _, fragment := range []string{"from clusters c", "left join cluster_members cm", "where ($1 = '' or c.cluster_type = $1)", "and ($2 = '' or c.status = $2)", "limit $3"} {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreGetClusterDetailReturnsMembers(t *testing.T) {
	queryer := &recordingDetailQueryer{
		clusterRow: staticClusterDetailRow{
			item: clusters.Cluster{
				ID:          7,
				ClusterType: "same_series",
				Title:       "Series A",
				Confidence:  float64Ptr(0.88),
				Status:      "candidate",
				CoverFileID: int64Ptr(15),
				MemberCount: 2,
				StrongMemberCount: 1,
				TopMemberScore:    float64Ptr(0.99),
				CreatedAt:   "2026-04-09T20:00:00Z",
			},
		},
		memberRows: &staticClusterMemberRows{
			items: []clusters.ClusterMember{
				{FileID: 15, FileName: "a.jpg", AbsPath: "/Volumes/media/a.jpg", MediaType: "image", Role: "cover", Score: float64Ptr(0.99), QualityTier: "high", HasFace: true, SubjectCount: "single", CaptureType: "selfie", EmbeddingType: "person_visual", EmbeddingProvider: "semantic", EmbeddingModel: "semantic-ollama-qwen3-vl-8b-v1", EmbeddingVectorCount: 1},
				{FileID: 16, FileName: "b.jpg", AbsPath: "/Volumes/media/b.jpg", MediaType: "image", Role: "member", Score: float64Ptr(0.88), QualityTier: "medium", HasFace: true, SubjectCount: "single", CaptureType: "photo", EmbeddingType: "image_visual", EmbeddingProvider: "pixel", EmbeddingModel: "ffmpeg-gray-8x8-v1", EmbeddingVectorCount: 1},
			},
		},
	}
	store := clusters.PostgresStore{
		Rows:          queryer,
		DetailQueryer: queryer,
	}

	item, err := store.GetClusterDetail(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected detail lookup to succeed: %v", err)
	}
	if item.ID != 7 || len(item.Members) != 2 || item.Members[0].Role != "cover" || item.StrongMemberCount != 1 {
		t.Fatalf("unexpected cluster detail: %#v", item)
	}
	if item.Members[0].EmbeddingType != "person_visual" || item.Members[0].EmbeddingProvider != "semantic" || item.Members[1].EmbeddingModel != "ffmpeg-gray-8x8-v1" {
		t.Fatalf("expected member embedding detail, got %#v", item.Members)
	}
	if !item.Members[0].HasFace || item.Members[0].SubjectCount != "single" || item.Members[0].CaptureType != "selfie" {
		t.Fatalf("expected structured person signals on member, got %#v", item.Members[0])
	}
	if item.PersonVisualCount != 1 || item.GenericVisualCount != 1 || item.TopEvidenceType != "person_visual" {
		t.Fatalf("expected cluster evidence summary, got %#v", item)
	}
}

func TestPostgresStoreSummarizeClustersReturnsItems(t *testing.T) {
	store := clusters.PostgresStore{
		Rows: &recordingRowsQueryer{
			rows: &staticClusterSummaryRows{
				items: []clusters.ClusterSummary{
					{ClusterType: "same_content", Status: "candidate", ClusterCount: 4, MemberCount: 9},
					{ClusterType: "same_series", Status: "candidate", ClusterCount: 2, MemberCount: 5},
				},
			},
		},
	}

	items, err := store.SummarizeClusters(context.Background())
	if err != nil {
		t.Fatalf("expected summarize clusters to succeed: %v", err)
	}
	if len(items) != 2 || items[0].ClusterType != "same_content" || items[1].ClusterCount != 2 {
		t.Fatalf("unexpected cluster summary: %#v", items)
	}
}

type recordingRowsQueryer struct {
	query string
	args  []any
	rows  clusters.RowsScanner
}

func (r *recordingRowsQueryer) QueryContext(_ context.Context, query string, args ...any) (clusters.RowsScanner, error) {
	r.query = query
	r.args = args
	return r.rows, nil
}

type recordingDetailQueryer struct {
	query      string
	args       []any
	clusterRow staticClusterDetailRow
	memberRows clusters.RowsScanner
	queryCount int
}

func (r *recordingDetailQueryer) QueryContext(_ context.Context, query string, args ...any) (clusters.RowsScanner, error) {
	r.query = query
	r.args = args
	r.queryCount++
	return r.memberRows, nil
}

func (r *recordingDetailQueryer) QueryRowContext(_ context.Context, query string, args ...any) clusters.RowScanner {
	r.query = query
	r.args = args
	return r.clusterRow
}

type staticClusterRows struct {
	items []clusters.Cluster
	index int
}

func (r *staticClusterRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticClusterRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.ID
	*dest[1].(*string) = item.ClusterType
	*dest[2].(*string) = item.Title
	assignNullFloat64(dest[3], item.Confidence)
	*dest[4].(*string) = item.Status
	assignNullInt64(dest[5], item.CoverFileID)
	*dest[6].(*int64) = item.MemberCount
	*dest[7].(*int64) = item.StrongMemberCount
	assignNullFloat64(dest[8], item.TopMemberScore)
	*dest[9].(*int64) = item.PersonVisualCount
	*dest[10].(*int64) = item.GenericVisualCount
	*dest[11].(*string) = item.TopEvidenceType
	*dest[12].(*string) = item.CreatedAt
	return nil
}

func (r *staticClusterRows) Err() error   { return nil }
func (r *staticClusterRows) Close() error { return nil }

type staticClusterDetailRow struct {
	item clusters.Cluster
}

func (r staticClusterDetailRow) Scan(dest ...any) error {
	*dest[0].(*int64) = r.item.ID
	*dest[1].(*string) = r.item.ClusterType
	*dest[2].(*string) = r.item.Title
	assignNullFloat64(dest[3], r.item.Confidence)
	*dest[4].(*string) = r.item.Status
	assignNullInt64(dest[5], r.item.CoverFileID)
	*dest[6].(*int64) = r.item.MemberCount
	*dest[7].(*int64) = r.item.StrongMemberCount
	assignNullFloat64(dest[8], r.item.TopMemberScore)
	*dest[9].(*string) = r.item.CreatedAt
	return nil
}

type staticClusterMemberRows struct {
	items []clusters.ClusterMember
	index int
}

func (r *staticClusterMemberRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticClusterMemberRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*int64) = item.FileID
	*dest[1].(*string) = item.FileName
	*dest[2].(*string) = item.AbsPath
	*dest[3].(*string) = item.MediaType
	*dest[4].(*string) = item.Role
	assignNullFloat64(dest[5], item.Score)
	*dest[6].(*string) = item.QualityTier
	*dest[7].(*bool) = item.HasFace
	*dest[8].(*string) = item.SubjectCount
	*dest[9].(*string) = item.CaptureType
	*dest[10].(*string) = item.EmbeddingType
	*dest[11].(*string) = item.EmbeddingProvider
	*dest[12].(*string) = item.EmbeddingModel
	*dest[13].(*int64) = item.EmbeddingVectorCount
	return nil
}

func (r *staticClusterMemberRows) Err() error   { return nil }
func (r *staticClusterMemberRows) Close() error { return nil }

type staticClusterSummaryRows struct {
	items []clusters.ClusterSummary
	index int
}

func (r *staticClusterSummaryRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticClusterSummaryRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*string) = item.ClusterType
	*dest[1].(*string) = item.Status
	*dest[2].(*int64) = item.ClusterCount
	*dest[3].(*int64) = item.MemberCount
	return nil
}

func (r *staticClusterSummaryRows) Err() error   { return nil }
func (r *staticClusterSummaryRows) Close() error { return nil }

func assignNullFloat64(target any, value *float64) {
	ptr := target.(*sql.NullFloat64)
	if value == nil {
		*ptr = sql.NullFloat64{}
		return
	}
	*ptr = sql.NullFloat64{Float64: *value, Valid: true}
}

func assignNullInt64(target any, value *int64) {
	ptr := target.(*sql.NullInt64)
	if value == nil {
		*ptr = sql.NullInt64{}
		return
	}
	*ptr = sql.NullInt64{Int64: *value, Valid: true}
}

func float64Ptr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64       { return &v }

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
