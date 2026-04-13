package reveal_test

import (
	"context"
	"testing"

	"idea/internal/reveal"
)

func TestServiceRevealFileInvokesRevealer(t *testing.T) {
	store := &recordingStore{
		file: reveal.File{
			ID:      7,
			AbsPath: "/Volumes/media/poster.jpg",
			Status:  "active",
		},
	}
	revealer := &recordingRevealer{}
	service := reveal.Service{
		Store:    store,
		Revealer: revealer,
	}

	if err := service.RevealFile(context.Background(), 7); err != nil {
		t.Fatalf("expected reveal to succeed: %v", err)
	}
	if revealer.path != "/Volumes/media/poster.jpg" {
		t.Fatalf("expected revealer to receive file path, got %q", revealer.path)
	}
}

type recordingStore struct {
	file reveal.File
}

func (s *recordingStore) GetFile(_ context.Context, _ int64) (reveal.File, error) {
	return s.file, nil
}

type recordingRevealer struct {
	path string
}

func (r *recordingRevealer) RevealInFinder(_ context.Context, path string) error {
	r.path = path
	return nil
}
