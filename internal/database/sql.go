package database

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}
	return sql.Open("pgx", databaseURL)
}

type SQLExecer struct {
	DB *sql.DB
}

func (s SQLExecer) ExecContext(ctx context.Context, query string, args ...any) error {
	_, err := s.DB.ExecContext(ctx, query, args...)
	return err
}

func NewRunner(db *sql.DB) Runner {
	return Runner{Execer: SQLExecer{DB: db}}
}
