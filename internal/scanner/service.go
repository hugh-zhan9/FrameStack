package scanner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrVolumeOffline = errors.New("volume offline")

type Volume struct {
	ID        int64
	MountPath string
}

type FileRecord struct {
	VolumeID   int64
	AbsPath    string
	ParentPath string
	FileName   string
	Extension  string
	MediaType  string
	SizeBytes  int64
	ModTime    time.Time
}

type UpsertResult struct {
	FileID  int64
	Changed bool
}

type Stats struct {
	Discovered int
}

type Store interface {
	GetVolume(ctx context.Context, volumeID int64) (Volume, error)
	TouchVolume(ctx context.Context, volumeID int64) error
	UpsertFile(ctx context.Context, record FileRecord) (UpsertResult, error)
	MarkMissingFiles(ctx context.Context, volumeID int64, seenPaths []string) error
}

type FileJobEnqueuer interface {
	EnqueueFileProcessing(ctx context.Context, fileID int64, mediaType string) error
}

type Service struct {
	Store    Store
	Enqueuer FileJobEnqueuer
}

func (s Service) ScanVolume(ctx context.Context, volumeID int64) (Stats, error) {
	volume, err := s.Store.GetVolume(ctx, volumeID)
	if err != nil {
		return Stats{}, err
	}

	info, err := os.Stat(volume.MountPath)
	if err != nil || !info.IsDir() {
		return Stats{}, ErrVolumeOffline
	}

	seenPaths := make([]string, 0, 128)
	stats := Stats{}
	walkErr := filepath.WalkDir(volume.MountPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		mediaType, ok := detectMediaType(path)
		if !ok {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		record := FileRecord{
			VolumeID:   volume.ID,
			AbsPath:    path,
			ParentPath: filepath.Dir(path),
			FileName:   filepath.Base(path),
			Extension:  strings.ToLower(filepath.Ext(path)),
			MediaType:  mediaType,
			SizeBytes:  info.Size(),
			ModTime:    info.ModTime(),
		}
		upserted, err := s.Store.UpsertFile(ctx, record)
		if err != nil {
			return err
		}
		if s.Enqueuer != nil && upserted.Changed {
			if err := s.Enqueuer.EnqueueFileProcessing(ctx, upserted.FileID, record.MediaType); err != nil {
				return err
			}
		}
		seenPaths = append(seenPaths, path)
		stats.Discovered++
		return nil
	})
	if walkErr != nil {
		return Stats{}, walkErr
	}

	if err := s.Store.TouchVolume(ctx, volume.ID); err != nil {
		return Stats{}, err
	}
	if err := s.Store.MarkMissingFiles(ctx, volume.ID, seenPaths); err != nil {
		return Stats{}, err
	}
	return stats, nil
}

func detectMediaType(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".heic":
		return "image", true
	case ".mp4", ".mov", ".mkv", ".avi", ".wmv", ".m4v", ".webm":
		return "video", true
	default:
		return "", false
	}
}
