package tags

import (
	"context"
	"database/sql"
)

type Tag struct {
	Namespace   string
	Name        string
	DisplayName string
	FileCount   int64
}

type ListOptions struct {
	Namespace string
	Limit     int
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

type SQLRowsDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type PostgresStore struct {
	Rows RowsQueryer
}

func NewPostgresStoreFromDB(db SQLRowsDB) PostgresStore {
	return PostgresStore{
		Rows: sqlRowsQueryer{db: db},
	}
}

func ListQuery() string {
	return listTagsQuery
}

func (s PostgresStore) ListTags(ctx context.Context, options ListOptions) ([]Tag, error) {
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.Rows.QueryContext(ctx, listTagsQuery, options.Namespace, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Tag
	for rows.Next() {
		var item Tag
		if err := rows.Scan(&item.Namespace, &item.Name, &item.DisplayName, &item.FileCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listTagsQuery = `
select
  t.namespace,
  t.name,
  t.display_name,
  count(ft.id) as file_count
from tags t
left join file_tags ft on ft.tag_id = t.id
where ($1 = '' or t.namespace = $1)
group by t.id, t.namespace, t.name, t.display_name
order by count(ft.id) desc, t.display_name asc
limit $2
`

type sqlRowsQueryer struct {
	db SQLRowsDB
}

func (q sqlRowsQueryer) QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error) {
	return q.db.QueryContext(ctx, query, args...)
}
