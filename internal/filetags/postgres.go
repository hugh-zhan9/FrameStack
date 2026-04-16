package filetags

import "context"

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type PostgresStore struct {
	Execer Execer
}

func (s PostgresStore) UpsertManualTag(ctx context.Context, fileID int64, input CreateInput) error {
	if err := s.Execer.ExecContext(ctx, upsertTagQuery, input.Namespace, input.Name, input.DisplayName, input.Namespace == "sensitive"); err != nil {
		return err
	}
	return s.Execer.ExecContext(ctx, upsertFileTagQuery, fileID, input.Namespace, input.Name)
}

func (s PostgresStore) DeleteManualTag(ctx context.Context, fileID int64, input DeleteInput) error {
	return s.Execer.ExecContext(ctx, deleteFileTagQuery, fileID, input.Namespace, input.Name)
}

func (s PostgresStore) ReplaceTag(ctx context.Context, fileID int64, input ReplaceInput) error {
	if err := s.Execer.ExecContext(ctx, upsertTagQuery, input.Namespace, input.Name, input.DisplayName, input.Namespace == "sensitive"); err != nil {
		return err
	}
	return s.Execer.ExecContext(ctx, replaceFileTagQuery, fileID, input.CurrentNamespace, input.CurrentName, input.Namespace, input.Name)
}

const upsertTagQuery = `
insert into tags (
  namespace,
  name,
  display_name,
  is_system,
  is_sensitive
)
values (
  $1,
  $2,
  $3,
  false,
  $4
)
on conflict (namespace, name) do update
set
  display_name = excluded.display_name,
  is_sensitive = excluded.is_sensitive
`

const upsertFileTagQuery = `
insert into file_tags (
  file_id,
  tag_id,
  source,
  confidence,
  evidence
)
select
  $1,
  t.id,
  'human',
  null,
  '{"source":"manual"}'::jsonb
from tags t
where t.namespace = $2 and t.name = $3
on conflict (file_id, tag_id, source) do update
set
  evidence = excluded.evidence
`

const deleteFileTagQuery = `
delete from file_tags ft
using tags t
where ft.tag_id = t.id
  and ft.file_id = $1
  and t.namespace = $2
  and t.name = $3
`

const replaceFileTagQuery = `
with deleted as (
  delete from file_tags ft
  using tags old_tag
  where ft.tag_id = old_tag.id
    and ft.file_id = $1
    and old_tag.namespace = $2
    and old_tag.name = $3
),
selected_tag as (
  select t.id
  from tags t
  where t.namespace = $4
    and t.name = $5
)
insert into file_tags (
  file_id,
  tag_id,
  source,
  confidence,
  evidence
)
select
  $1,
  st.id,
  'human',
  null,
  '{"source":"manual"}'::jsonb
from selected_tag st
on conflict (file_id, tag_id, source) do update
set
  evidence = excluded.evidence
`
