package database_test

import (
	"os"
	"strings"
	"testing"
)

func TestInitMigrationPinsEmbeddingVectorDimensions(t *testing.T) {
	content, err := os.ReadFile("../../db/migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("expected to read migration: %v", err)
	}

	sql := string(content)
	if !strings.Contains(sql, "vector vector(64) not null") {
		t.Fatalf("expected embeddings.vector to use fixed dimensions, got migration content without vector(64)")
	}
}

func TestMigrationsAllowSeriesFocusClusterRole(t *testing.T) {
	initContent, err := os.ReadFile("../../db/migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("expected to read init migration: %v", err)
	}
	followupContent, err := os.ReadFile("../../db/migrations/0004_allow_series_focus_role.sql")
	if err != nil {
		t.Fatalf("expected to read follow-up migration: %v", err)
	}

	initSQL := string(initContent)
	followupSQL := string(followupContent)
	if !strings.Contains(initSQL, "'series_focus'") && !strings.Contains(followupSQL, "'series_focus'") {
		t.Fatalf("expected migrations to allow series_focus cluster role")
	}
}
