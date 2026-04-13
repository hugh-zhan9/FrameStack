package trash_test

import (
	"context"
	"errors"
	"testing"

	"idea/internal/trash"
)

func TestServiceMoveFileToTrash(t *testing.T) {
	store := &recordingStore{
		file: trash.File{
			ID:       7,
			AbsPath:  "/Volumes/media/photos/poster.jpg",
			Status:   "active",
			VolumeID: 3,
		},
	}
	mover := &recordingMover{}
	service := trash.Service{
		Store: store,
		Mover: mover,
	}

	if err := service.TrashFile(context.Background(), 7); err != nil {
		t.Fatalf("expected trash to succeed: %v", err)
	}
	if mover.path != "/Volumes/media/photos/poster.jpg" {
		t.Fatalf("unexpected trash path: %q", mover.path)
	}
	if store.trashedFileID != 7 {
		t.Fatalf("expected file 7 marked trashed, got %d", store.trashedFileID)
	}
}

func TestServiceSkipsMoverWhenAlreadyTrashed(t *testing.T) {
	store := &recordingStore{
		file: trash.File{
			ID:      8,
			AbsPath: "/Volumes/media/photos/old.jpg",
			Status:  "trashed",
		},
	}
	mover := &recordingMover{}
	service := trash.Service{
		Store: store,
		Mover: mover,
	}

	if err := service.TrashFile(context.Background(), 8); err != nil {
		t.Fatalf("expected already trashed file to be tolerated: %v", err)
	}
	if mover.path != "" {
		t.Fatalf("expected mover to be skipped, got %q", mover.path)
	}
	if store.trashedFileID != 0 {
		t.Fatalf("expected no db update, got %d", store.trashedFileID)
	}
}

func TestServicePropagatesMoverError(t *testing.T) {
	store := &recordingStore{
		file: trash.File{
			ID:      9,
			AbsPath: "/Volumes/media/photos/bad.jpg",
			Status:  "active",
		},
	}
	service := trash.Service{
		Store: store,
		Mover: &recordingMover{err: errors.New("finder unavailable")},
	}

	if err := service.TrashFile(context.Background(), 9); err == nil {
		t.Fatal("expected trash to fail")
	}
	if store.trashedFileID != 0 {
		t.Fatalf("expected store not to update after mover failure, got %d", store.trashedFileID)
	}
}

type recordingStore struct {
	file          trash.File
	trashedFileID int64
}

func (s *recordingStore) GetFile(_ context.Context, _ int64) (trash.File, error) {
	return s.file, nil
}

func (s *recordingStore) MarkFileTrashed(_ context.Context, fileID int64) error {
	s.trashedFileID = fileID
	return nil
}

type recordingMover struct {
	path string
	err  error
}

func (m *recordingMover) MoveToTrash(_ context.Context, path string) error {
	m.path = path
	return m.err
}
