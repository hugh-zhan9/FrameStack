package sameseries_test

import (
	"context"
	"testing"
	"time"

	"idea/internal/sameseries"
)

func TestServiceClusterFileCreatesCandidateForNearbySiblingFiles(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:     7,
			ParentPath: "/Volumes/media/set-a",
			FileName:   "model-a-001.jpg",
			MediaType:  "image",
			ModTime:    now,
			Status:     "active",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 7, ParentPath: "/Volumes/media/set-a", FileName: "model-a-001.jpg", ModTime: now},
			{FileID: 8, ParentPath: "/Volumes/media/set-a", FileName: "model-a-002.jpg", ModTime: now.Add(8 * time.Minute)},
			{FileID: 9, ParentPath: "/Volumes/media/set-a", FileName: "model-a-003.jpg", ModTime: now.Add(-6 * time.Minute)},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if store.clusterKey == "" {
		t.Fatal("expected same_series cluster to be upserted")
	}
	if len(store.clusterFiles) != 3 {
		t.Fatalf("expected 3 cluster members, got %#v", store.clusterFiles)
	}
	if store.clusterFiles[1].Role != "series_focus" {
		t.Fatalf("expected middle candidate to be marked series_focus, got %#v", store.clusterFiles)
	}
	if store.clusterFiles[0].Role != "member" || store.clusterFiles[2].Role != "member" {
		t.Fatalf("expected non-focus series members to remain member, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileSkipsWhenNotEnoughCandidates(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:     7,
			ParentPath: "/Volumes/media/set-a",
			FileName:   "model-a-001.jpg",
			MediaType:  "image",
			ModTime:    now,
			Status:     "active",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 7, ParentPath: "/Volumes/media/set-a", FileName: "model-a-001.jpg", ModTime: now},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileSkipsMissingOrTrashedFiles(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	for _, status := range []string{"missing", "trashed"} {
		store := &recordingStore{
			file: sameseries.FileContext{
				FileID:     7,
				ParentPath: "/Volumes/media/set-a",
				FileName:   "model-a-001.jpg",
				MediaType:  "image",
				ModTime:    now,
				Status:     status,
			},
		}
		service := sameseries.Service{Store: store}

		if err := service.ClusterFile(context.Background(), 7); err != nil {
			t.Fatalf("expected cluster file to succeed for status %s: %v", status, err)
		}
		if store.clusterKey != "" {
			t.Fatalf("expected no cluster write for status %s", status)
		}
	}
}

func TestServiceClusterFileFiltersOutUnrelatedNearbyCandidates(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:     7,
			ParentPath: "/Volumes/media/set-a",
			FileName:   "model-a-001.jpg",
			MediaType:  "image",
			ModTime:    now,
			Status:     "active",
			ImagePHash: "ffffffffffffffff",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 7, ParentPath: "/Volumes/media/set-a", FileName: "model-a-001.jpg", ModTime: now, ImagePHash: "ffffffffffffffff"},
			{FileID: 8, ParentPath: "/Volumes/media/set-a", FileName: "random-cover.jpg", ModTime: now.Add(5 * time.Minute), ImagePHash: "fffffffffffffffe"},
			{FileID: 9, ParentPath: "/Volumes/media/set-a", FileName: "other-batch.jpg", ModTime: now.Add(4 * time.Minute), ImagePHash: "0000000000000000"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 8 {
		t.Fatalf("expected only related nearby candidates to remain, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileKeepsImageCandidatesWithNearEmbeddings(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              27,
			ParentPath:          "/Volumes/media/set-b",
			FileName:            "lookbook-a.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 27, ParentPath: "/Volumes/media/set-b", FileName: "lookbook-a.jpg", ModTime: now, ImageEmbedding: "[0.95,0.90,0.85]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
			{FileID: 28, ParentPath: "/Volumes/media/set-b", FileName: "unrelated-title.jpg", ModTime: now.Add(4 * time.Minute), ImageEmbedding: "[0.95,0.89,0.84]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
			{FileID: 29, ParentPath: "/Volumes/media/set-b", FileName: "other-batch.jpg", ModTime: now.Add(5 * time.Minute), ImageEmbedding: "[5.00,5.00,5.00]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 27); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 28 {
		t.Fatalf("expected only embedding-near candidate to remain, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileIgnoresImageEmbeddingsFromDifferentModels(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              27,
			ParentPath:          "/Volumes/media/set-b",
			FileName:            "lookbook-a.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "semantic-a",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 27, ParentPath: "/Volumes/media/set-b", FileName: "lookbook-a.jpg", ModTime: now, ImageEmbedding: "[0.95,0.90,0.85]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "semantic-a"},
			{FileID: 28, ParentPath: "/Volumes/media/set-b", FileName: "unrelated-title.jpg", ModTime: now.Add(4 * time.Minute), ImageEmbedding: "[0.95,0.89,0.84]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "semantic-b"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 27); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected model-mismatched image embedding candidate to be ignored, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileIgnoresImageEmbeddingsFromDifferentTypes(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              27,
			ParentPath:          "/Volumes/media/set-b",
			FileName:            "lookbook-a.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "semantic-a",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 27, ParentPath: "/Volumes/media/set-b", FileName: "lookbook-a.jpg", ModTime: now, ImageEmbedding: "[0.95,0.90,0.85]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "semantic-a"},
			{FileID: 28, ParentPath: "/Volumes/media/set-b", FileName: "unrelated-title.jpg", ModTime: now.Add(4 * time.Minute), ImageEmbedding: "[0.95,0.89,0.84]", ImageEmbeddingType: "person_visual", ImageEmbeddingModel: "semantic-a"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 27); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected type-mismatched image embedding candidate to be ignored, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileRejectsImageCandidatesWhenAspectRatioConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              37,
			ParentPath:          "/Volumes/media/set-b",
			FileName:            "lookbook-b.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			Width:               1920,
			Height:              1080,
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 37, ParentPath: "/Volumes/media/set-b", FileName: "lookbook-b.jpg", ModTime: now, Width: 1920, Height: 1080, ImageEmbedding: "[0.95,0.90,0.85]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
			{FileID: 38, ParentPath: "/Volumes/media/set-b", FileName: "lookbook-b-002.jpg", ModTime: now.Add(4 * time.Minute), Width: 1080, Height: 1920, ImageEmbedding: "[0.95,0.89,0.84]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 37); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected aspect-ratio-conflicting image candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsImageCandidatesWhenResolutionIsTooFarApart(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              47,
			ParentPath:          "/Volumes/media/set-b",
			FileName:            "lookbook-c.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			Width:               3840,
			Height:              2160,
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 47, ParentPath: "/Volumes/media/set-b", FileName: "lookbook-c.jpg", ModTime: now, Width: 3840, Height: 2160, ImageEmbedding: "[0.95,0.90,0.85]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
			{FileID: 48, ParentPath: "/Volumes/media/set-b", FileName: "lookbook-c-002.jpg", ModTime: now.Add(4 * time.Minute), Width: 320, Height: 180, ImageEmbedding: "[0.95,0.89,0.84]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 47); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected extreme-resolution-gap image candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileKeepsVideoCandidatesWithSharedFrames(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:            17,
			ParentPath:        "/Volumes/media/set-v",
			FileName:          "clip-a-001.mp4",
			MediaType:         "video",
			ModTime:           now,
			Status:            "active",
			DurationMS:        60_000,
			VideoFramePHashes: []string{"a1", "b2", "c3"},
			VideoFrameEmbeddings: []string{
				"[0.10,0.20,0.30]",
				"[0.80,0.75,0.70]",
			},
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 17, ParentPath: "/Volumes/media/set-v", FileName: "clip-a-001.mp4", ModTime: now, DurationMS: 60_000, VideoFramePHashes: []string{"a1", "b2", "c3"}, VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingType: "video_frame_visual", VideoFrameEmbeddingModel: "semantic-v1"},
			{FileID: 18, ParentPath: "/Volumes/media/set-v", FileName: "weird-name.mp4", ModTime: now.Add(3 * time.Minute), DurationMS: 58_000, VideoFramePHashes: []string{"z9", "b2"}},
			{FileID: 19, ParentPath: "/Volumes/media/set-v", FileName: "other.mp4", ModTime: now.Add(4 * time.Minute), DurationMS: 61_000, VideoFramePHashes: []string{"x1", "y2"}, VideoFrameEmbeddings: []string{"[0.79,0.74,0.69]"}, VideoFrameEmbeddingType: "video_frame_visual", VideoFrameEmbeddingModel: "semantic-v1"},
			{FileID: 20, ParentPath: "/Volumes/media/set-v", FileName: "far.mp4", ModTime: now.Add(5 * time.Minute), DurationMS: 62_000, VideoFramePHashes: []string{"m1", "n2"}, VideoFrameEmbeddings: []string{"[0.05,0.05,0.05]"}},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 17); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 3 || store.clusterFiles[1].FileID != 18 || store.clusterFiles[2].FileID != 19 {
		t.Fatalf("expected shared-frame and embedding-near video candidates, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileRejectsVideoCandidatesWhenDurationIsIncompatible(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:            27,
			ParentPath:        "/Volumes/media/set-v",
			FileName:          "clip-b-001.mp4",
			MediaType:         "video",
			ModTime:           now,
			Status:            "active",
			DurationMS:        60_000,
			VideoFramePHashes: []string{"a1", "b2", "c3"},
			VideoFrameEmbeddings: []string{
				"[0.10,0.20,0.30]",
			},
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 27, ParentPath: "/Volumes/media/set-v", FileName: "clip-b-001.mp4", ModTime: now, DurationMS: 60_000, VideoFramePHashes: []string{"a1", "b2", "c3"}, VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingType: "video_frame_visual", VideoFrameEmbeddingModel: "semantic-v1"},
			{FileID: 28, ParentPath: "/Volumes/media/set-v", FileName: "clip-b-002.mp4", ModTime: now.Add(4 * time.Minute), DurationMS: 300_000, VideoFramePHashes: []string{"z9", "b2"}},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 27); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected incompatible-duration video candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsVideoCandidatesWhenOrientationConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:            37,
			ParentPath:        "/Volumes/media/set-v",
			FileName:          "clip-c-001.mp4",
			MediaType:         "video",
			ModTime:           now,
			Status:            "active",
			DurationMS:        60_000,
			Width:             1920,
			Height:            1080,
			VideoFramePHashes: []string{"a1", "b2", "c3"},
			VideoFrameEmbeddings: []string{
				"[0.10,0.20,0.30]",
			},
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 37, ParentPath: "/Volumes/media/set-v", FileName: "clip-c-001.mp4", ModTime: now, DurationMS: 60_000, Width: 1920, Height: 1080, VideoFramePHashes: []string{"a1", "b2", "c3"}, VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingType: "video_frame_visual", VideoFrameEmbeddingModel: "semantic-v1"},
			{FileID: 38, ParentPath: "/Volumes/media/set-v", FileName: "clip-c-002.mp4", ModTime: now.Add(4 * time.Minute), DurationMS: 58_000, Width: 1080, Height: 1920, VideoFramePHashes: []string{"z9", "b2"}},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 37); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected orientation-conflicting video candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsVideoCandidatesWhenAspectRatioIsTooFarApart(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:            47,
			ParentPath:        "/Volumes/media/set-v",
			FileName:          "clip-d-001.mp4",
			MediaType:         "video",
			ModTime:           now,
			Status:            "active",
			DurationMS:        60_000,
			Width:             1920,
			Height:            1080,
			VideoFramePHashes: []string{"a1", "b2", "c3"},
			VideoFrameEmbeddings: []string{
				"[0.10,0.20,0.30]",
			},
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 47, ParentPath: "/Volumes/media/set-v", FileName: "clip-d-001.mp4", ModTime: now, DurationMS: 60_000, Width: 1920, Height: 1080, VideoFramePHashes: []string{"a1", "b2", "c3"}, VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingType: "video_frame_visual", VideoFrameEmbeddingModel: "semantic-v1"},
			{FileID: 48, ParentPath: "/Volumes/media/set-v", FileName: "clip-d-002.mp4", ModTime: now.Add(4 * time.Minute), DurationMS: 58_000, Width: 1440, Height: 1080, VideoFramePHashes: []string{"z9", "b2"}},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 47); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected aspect-ratio-conflicting video candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsVideoCandidatesWhenResolutionIsTooFarApart(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:                 57,
			ParentPath:             "/Volumes/media/set-v",
			FileName:               "clip-e-001.mp4",
			MediaType:              "video",
			ModTime:                now,
			Status:                 "active",
			DurationMS:             60_000,
			Width:                  3840,
			Height:                 2160,
			VideoFramePHashes:        []string{"a1", "b2", "c3"},
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 57, ParentPath: "/Volumes/media/set-v", FileName: "clip-e-001.mp4", ModTime: now, DurationMS: 60_000, Width: 3840, Height: 2160, VideoFramePHashes: []string{"a1", "b2", "c3"}, VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingType: "video_frame_visual", VideoFrameEmbeddingModel: "semantic-v1"},
			{FileID: 58, ParentPath: "/Volumes/media/set-v", FileName: "clip-e-002.mp4", ModTime: now.Add(4 * time.Minute), DurationMS: 58_000, Width: 320, Height: 180, VideoFramePHashes: []string{"z9", "b2"}},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 57); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected resolution-conflicting video candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileAllowsCrossDirectoryFamilyCandidatesWhenTechnicallyNear(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              31,
			ParentPath:          "/Volumes/media/set-a",
			FileName:            "model-a-001.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.50,0.50,0.50]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 31, ParentPath: "/Volumes/media/set-a", FileName: "model-a-001.jpg", ModTime: now, ImageEmbedding: "[0.50,0.50,0.50]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
			{FileID: 32, ParentPath: "/Volumes/media/set-b", FileName: "model-a-002.jpg", ModTime: now.Add(4 * time.Minute), ImageEmbedding: "[0.49,0.50,0.51]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 31); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 32 {
		t.Fatalf("expected cross-directory family candidate to remain, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileRejectsCrossDirectoryFamilyCandidatesWhenTooFarInTime(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              41,
			ParentPath:          "/Volumes/media/set-a",
			FileName:            "model-a-001.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.50,0.50,0.50]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 41, ParentPath: "/Volumes/media/set-a", FileName: "model-a-001.jpg", ModTime: now, ImageEmbedding: "[0.50,0.50,0.50]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
			{FileID: 42, ParentPath: "/Volumes/media/set-b", FileName: "model-a-002.jpg", ModTime: now.Add(21 * 24 * time.Hour), ImageEmbedding: "[0.49,0.50,0.51]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 41); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected far cross-directory family candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsCandidatesWhenCaptureTypeConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameseries.FileContext{
			FileID:              51,
			ParentPath:          "/Volumes/media/set-a",
			FileName:            "model-a-001.jpg",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			CaptureType:         "photo",
			ImageEmbedding:      "[0.50,0.50,0.50]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		candidates: []sameseries.SeriesCandidateFile{
			{FileID: 51, ParentPath: "/Volumes/media/set-a", FileName: "model-a-001.jpg", ModTime: now, CaptureType: "photo", ImageEmbedding: "[0.50,0.50,0.50]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
			{FileID: 52, ParentPath: "/Volumes/media/set-a", FileName: "model-a-002.jpg", ModTime: now.Add(4 * time.Minute), CaptureType: "screenshot", ImageEmbedding: "[0.49,0.50,0.51]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "pixel-v1"},
		},
	}
	service := sameseries.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 51); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected capture type conflict candidate to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedKey == "" {
		t.Fatalf("expected same_series cluster to be deactivated, got %#v", store)
	}
}

type recordingStore struct {
	file           sameseries.FileContext
	candidates     []sameseries.SeriesCandidateFile
	nearby         []sameseries.SeriesCandidateFile
	clusterKey     string
	clusterFiles   []sameseries.SeriesCandidateFile
	deactivatedKey string
}

func (s *recordingStore) GetFileContext(_ context.Context, _ int64) (sameseries.FileContext, error) {
	return s.file, nil
}

func (s *recordingStore) ListSeriesCandidateFiles(_ context.Context, file sameseries.FileContext, window time.Duration) ([]sameseries.SeriesCandidateFile, error) {
	return s.candidates, nil
}

func (s *recordingStore) ListNearbySeriesCandidateFiles(_ context.Context, file sameseries.FileContext, window time.Duration, limit int) ([]sameseries.SeriesCandidateFile, error) {
	return s.nearby, nil
}

func (s *recordingStore) UpsertSameSeriesCluster(_ context.Context, key string, files []sameseries.SeriesCandidateFile) error {
	s.clusterKey = key
	s.clusterFiles = append([]sameseries.SeriesCandidateFile(nil), files...)
	return nil
}

func (s *recordingStore) DeactivateSameSeriesCluster(_ context.Context, key string) error {
	s.deactivatedKey = key
	return nil
}
