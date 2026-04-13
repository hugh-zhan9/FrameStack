package filehash_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"idea/internal/filehash"
)

func TestServiceHashFileComputesAndStoresHashes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.bin")
	body := []byte("hello local media governance")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	store := &recordingStore{
		file: filehash.File{
			ID:      7,
			AbsPath: path,
		},
	}
	enqueuer := &recordingSameContentEnqueuer{}
	service := filehash.Service{
		Store:               store,
		SameContentEnqueuer: enqueuer,
	}

	if err := service.HashFile(context.Background(), 7); err != nil {
		t.Fatalf("expected hash to succeed: %v", err)
	}
	expectedSHA := sha256.Sum256(body)
	if store.input.FileID != 7 || store.input.SHA256 != hex.EncodeToString(expectedSHA[:]) || store.input.QuickHash == "" {
		t.Fatalf("unexpected hash input: %#v", store.input)
	}
	if enqueuer.fileID != 7 {
		t.Fatalf("expected same content enqueue for file 7, got %d", enqueuer.fileID)
	}
}

type recordingStore struct {
	file  filehash.File
	input filehash.HashInput
}

func (s *recordingStore) GetFile(_ context.Context, _ int64) (filehash.File, error) {
	return s.file, nil
}

func (s *recordingStore) UpdateHashes(_ context.Context, input filehash.HashInput) error {
	s.input = input
	return nil
}

type recordingSameContentEnqueuer struct {
	fileID int64
}

func (e *recordingSameContentEnqueuer) EnqueueSameContent(_ context.Context, fileID int64) error {
	e.fileID = fileID
	return nil
}
