package database

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MigrationFile struct {
	Path string
	Body string
}

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

type Runner struct {
	Execer Execer
}

func DiscoverMigrationFiles(dir string) ([]MigrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]MigrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, MigrationFile{
			Path: path,
			Body: strings.TrimSpace(string(body)),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return filepath.Base(files[i].Path) < filepath.Base(files[j].Path)
	})

	return files, nil
}

func (r Runner) Run(ctx context.Context, dir string) error {
	files, err := DiscoverMigrationFiles(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Body == "" {
			continue
		}
		if err := r.Execer.ExecContext(ctx, file.Body); err != nil {
			return err
		}
	}
	return nil
}
