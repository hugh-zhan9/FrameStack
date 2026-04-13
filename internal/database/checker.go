package database

import (
	"context"
	"database/sql"
)

type HealthChecker struct {
	DB *sql.DB
}

func (h HealthChecker) CheckHealth(ctx context.Context) error {
	if h.DB == nil {
		return sql.ErrConnDone
	}
	return h.DB.PingContext(ctx)
}
