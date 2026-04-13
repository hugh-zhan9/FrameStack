package review

import "context"

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type PostgresStore struct {
	Execer Execer
}

func (s PostgresStore) CreateFileAction(ctx context.Context, fileID int64, input FileActionInput) error {
	return s.Execer.ExecContext(ctx, createFileActionQuery, fileID, input.ActionType, input.Note)
}

const createFileActionQuery = `
insert into review_actions (
  file_id,
  action_type,
  note
)
values (
  $1,
  $2,
  nullif($3, '')
)
`
