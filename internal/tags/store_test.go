package tags_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"idea/internal/tags"
)

func TestPostgresStoreListTagsReturnsItems(t *testing.T) {
	queryer := &recordingRowsQueryer{
		rows: &staticTagRows{
			items: []tags.Tag{
				{Namespace: "content", Name: "单人写真", DisplayName: "单人写真", FileCount: 12},
			},
		},
	}
	store := tags.PostgresStore{Rows: queryer}

	items, err := store.ListTags(context.Background(), tags.ListOptions{
		Namespace: "content",
		Limit:     8,
	})
	if err != nil {
		t.Fatalf("expected list tags to succeed: %v", err)
	}
	if len(items) != 1 || items[0].Name != "单人写真" || items[0].FileCount != 12 {
		t.Fatalf("unexpected tags: %#v", items)
	}
	normalized := normalizeSQL(queryer.query)
	expectedFragments := []string{
		"from tags t",
		"left join file_tags ft on ft.tag_id = t.id",
		"where ($1 = '' or t.namespace = $1)",
		"group by t.id",
		"order by count(ft.id) desc, t.display_name asc",
		"limit $2",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(normalized, normalizeSQL(fragment)) {
			t.Fatalf("expected query to contain %q, got %q", fragment, queryer.query)
		}
	}
}

func TestPostgresStoreListTagsPropagatesQueryError(t *testing.T) {
	store := tags.PostgresStore{
		Rows: &recordingRowsQueryer{err: errors.New("db down")},
	}

	_, err := store.ListTags(context.Background(), tags.ListOptions{})
	if err == nil {
		t.Fatal("expected list tags to fail")
	}
}

type recordingRowsQueryer struct {
	query string
	args  []any
	rows  tags.RowsScanner
	err   error
}

func (r *recordingRowsQueryer) QueryContext(_ context.Context, query string, args ...any) (tags.RowsScanner, error) {
	r.query = query
	r.args = args
	if r.err != nil {
		return nil, r.err
	}
	return r.rows, nil
}

type staticTagRows struct {
	items []tags.Tag
	index int
}

func (r *staticTagRows) Next() bool {
	return r.index < len(r.items)
}

func (r *staticTagRows) Scan(dest ...any) error {
	item := r.items[r.index]
	r.index++
	*dest[0].(*string) = item.Namespace
	*dest[1].(*string) = item.Name
	*dest[2].(*string) = item.DisplayName
	*dest[3].(*int64) = item.FileCount
	return nil
}

func (r *staticTagRows) Err() error   { return nil }
func (r *staticTagRows) Close() error { return nil }

func normalizeSQL(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
