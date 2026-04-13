package scanner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"idea/internal/scanner"
)

func TestServiceScanVolumeIndexesMediaFilesAndMarksMissing(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "photo.jpg"), []byte("image"))
	mustWriteFile(t, filepath.Join(root, "clip.mp4"), []byte("video"))
	mustWriteFile(t, filepath.Join(root, "note.txt"), []byte("ignore"))

	store := &recordingStore{
		volume: scanner.Volume{ID: 7, MountPath: root},
	}
	service := scanner.Service{Store: store}

	stats, err := service.ScanVolume(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected scan to succeed: %v", err)
	}
	if stats.Discovered != 2 {
		t.Fatalf("expected 2 media files discovered, got %d", stats.Discovered)
	}
	if len(store.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(store.upserts))
	}
	if len(store.markMissingSeen) != 2 {
		t.Fatalf("expected 2 seen paths, got %d", len(store.markMissingSeen))
	}
	if !store.volumeTouched {
		t.Fatal("expected volume to be touched")
	}
}

func TestServiceScanVolumeEnqueuesProcessingForChangedFiles(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "photo.jpg"), []byte("image"))
	mustWriteFile(t, filepath.Join(root, "clip.mp4"), []byte("video"))

	store := &recordingStore{
		volume: scanner.Volume{ID: 7, MountPath: root},
		upsertResultsByPath: map[string]scanner.UpsertResult{
			filepath.Join(root, "photo.jpg"): {FileID: 101, Changed: true},
			filepath.Join(root, "clip.mp4"):  {FileID: 102, Changed: true},
		},
	}
	enqueuer := &recordingFileJobEnqueuer{}
	service := scanner.Service{
		Store:    store,
		Enqueuer: enqueuer,
	}

	_, err := service.ScanVolume(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected scan to succeed: %v", err)
	}
	if len(enqueuer.calls) != 2 {
		t.Fatalf("expected 2 enqueue calls, got %d", len(enqueuer.calls))
	}
	got := map[int64]string{}
	for _, call := range enqueuer.calls {
		got[call.FileID] = call.MediaType
	}
	if got[101] != "image" {
		t.Fatalf("expected file 101 to enqueue image processing, got %#v", got)
	}
	if got[102] != "video" {
		t.Fatalf("expected file 102 to enqueue video processing, got %#v", got)
	}
}

func TestServiceScanVolumeSkipsProcessingForUnchangedFiles(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "photo.jpg"), []byte("image"))

	store := &recordingStore{
		volume: scanner.Volume{ID: 7, MountPath: root},
		upsertResultsByPath: map[string]scanner.UpsertResult{
			filepath.Join(root, "photo.jpg"): {FileID: 101, Changed: false},
		},
	}
	enqueuer := &recordingFileJobEnqueuer{}
	service := scanner.Service{
		Store:    store,
		Enqueuer: enqueuer,
	}

	_, err := service.ScanVolume(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected scan to succeed: %v", err)
	}
	if len(enqueuer.calls) != 0 {
		t.Fatalf("expected no enqueue calls, got %d", len(enqueuer.calls))
	}
}

func TestServiceScanVolumeReturnsVolumeLookupError(t *testing.T) {
	service := scanner.Service{
		Store: &recordingStore{getVolumeErr: errors.New("db down")},
	}

	_, err := service.ScanVolume(context.Background(), 7)
	if err == nil {
		t.Fatal("expected scan to fail")
	}
}

func TestServiceScanVolumeReturnsOfflineErrorForMissingMountPath(t *testing.T) {
	service := scanner.Service{
		Store: &recordingStore{volume: scanner.Volume{ID: 7, MountPath: "/path/does/not/exist"}},
	}

	_, err := service.ScanVolume(context.Background(), 7)
	if !errors.Is(err, scanner.ErrVolumeOffline) {
		t.Fatalf("expected ErrVolumeOffline, got %v", err)
	}
}

type recordingStore struct {
	volume              scanner.Volume
	getVolumeErr        error
	upserts             []scanner.FileRecord
	upsertResultsByPath map[string]scanner.UpsertResult
	markMissingSeen     []string
	volumeTouched       bool
}

func (s *recordingStore) GetVolume(_ context.Context, volumeID int64) (scanner.Volume, error) {
	if s.getVolumeErr != nil {
		return scanner.Volume{}, s.getVolumeErr
	}
	if s.volume.ID == 0 {
		s.volume.ID = volumeID
	}
	return s.volume, nil
}

func (s *recordingStore) TouchVolume(_ context.Context, _ int64) error {
	s.volumeTouched = true
	return nil
}

func (s *recordingStore) UpsertFile(_ context.Context, record scanner.FileRecord) (scanner.UpsertResult, error) {
	s.upserts = append(s.upserts, record)
	if s.upsertResultsByPath == nil {
		return scanner.UpsertResult{}, nil
	}
	return s.upsertResultsByPath[record.AbsPath], nil
}

func (s *recordingStore) MarkMissingFiles(_ context.Context, _ int64, seenPaths []string) error {
	s.markMissingSeen = append([]string{}, seenPaths...)
	return nil
}

func mustWriteFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	modtime := time.Date(2026, 4, 9, 18, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, modtime, modtime); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}
}

type recordingFileJobEnqueuer struct {
	calls []recordingFileJob
}

type recordingFileJob struct {
	FileID    int64
	MediaType string
}

func (e *recordingFileJobEnqueuer) EnqueueFileProcessing(_ context.Context, fileID int64, mediaType string) error {
	e.calls = append(e.calls, recordingFileJob{
		FileID:    fileID,
		MediaType: mediaType,
	})
	return nil
}
