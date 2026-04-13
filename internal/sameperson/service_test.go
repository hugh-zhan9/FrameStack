package sameperson_test

import (
	"context"
	"testing"
	"time"

	"idea/internal/sameperson"
)

func TestClusterFileUpsertsClusterForSharedPersonTag(t *testing.T) {
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:     7,
			FileName:   "alice-001.jpg",
			ParentPath: "/Volumes/media/alice",
			ModTime:    time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
			Status:     "active",
		},
		tags: []sameperson.PersonTag{{Name: "alice", Source: "human"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"alice": {{FileID: 7}, {FileName: "alice-002.jpg", FileID: 8}},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store)
	}
	if store.upserts[0].title != "person:alice" {
		t.Fatalf("unexpected title: %#v", store.upserts[0])
	}
}

func TestClusterFileDeactivatesClusterWhenOnlyOneCandidateExists(t *testing.T) {
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:     7,
			FileName:   "alice-001.jpg",
			ParentPath: "/Volumes/media/alice",
			ModTime:    time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
			Status:     "active",
		},
		tags: []sameperson.PersonTag{{Name: "alice", Source: "human"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"alice": {{FileID: 7, FileName: "alice-001.jpg"}},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected 1 deactivation, got %#v", store)
	}
}

func TestClusterFileSkipsWhenNoPersonTagsExist(t *testing.T) {
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:  7,
			Status:  "active",
			ModTime: time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 || len(store.deactivations) != 0 {
		t.Fatalf("expected no cluster mutations, got %#v", store)
	}
}

func TestClusterFileFallsBackToAutoPersonSignalsWhenNoExplicitPersonTagsExist(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              31,
			FileName:            "set-c-001.jpg",
			ParentPath:          "/Volumes/media/set-c",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			Width:               1920,
			Height:              1080,
			ImageEmbedding:      "[0.11,0.22,0.33]",
			ImageEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "单人写真", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"单人写真": {
				{FileID: 31, FileName: "set-c-001.jpg", ParentPath: "/Volumes/media/set-c", MediaType: "image", ModTime: now, Width: 1920, Height: 1080, ImageEmbedding: "[0.11,0.22,0.33]", ImageEmbeddingModel: "semantic-v1"},
				{FileID: 32, FileName: "set-c-002.jpg", ParentPath: "/Volumes/media/set-d", MediaType: "image", ModTime: now.Add(2 * time.Hour), Width: 1920, Height: 1080, ImageEmbedding: "[0.12,0.21,0.34]", ImageEmbeddingModel: "semantic-v1"},
				{FileID: 33, FileName: "other-object.jpg", ParentPath: "/Volumes/media/other", MediaType: "image", ModTime: now.Add(8 * time.Hour), Width: 1920, Height: 1080, ImageEmbedding: "[5.00,5.00,5.00]", ImageEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 31); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 2 {
		t.Fatalf("expected auto signal candidates to be narrowed by embedding, got %#v", store.upserts[0].files)
	}
	if store.upserts[0].title == "person:单人写真" {
		t.Fatalf("expected auto title specialization, got %#v", store.upserts[0])
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenImageResolutionIsIncompatible(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              34,
			FileName:            "set-d-001.jpg",
			ParentPath:          "/Volumes/media/set-d",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			Width:               3840,
			Height:              2160,
			ImageEmbedding:      "[0.11,0.22,0.33]",
			ImageEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "单人写真", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"单人写真": {
				{FileID: 34, FileName: "set-d-001.jpg", ParentPath: "/Volumes/media/set-d", MediaType: "image", ModTime: now, Width: 3840, Height: 2160, ImageEmbedding: "[0.11,0.22,0.33]", ImageEmbeddingModel: "semantic-v1"},
				{FileID: 35, FileName: "set-d-002.jpg", ParentPath: "/Volumes/media/other", MediaType: "image", ModTime: now.Add(time.Hour), Width: 320, Height: 180, ImageEmbedding: "[0.12,0.21,0.34]", ImageEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 34); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected incompatible image resolution auto signal to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for incompatible image resolution, got %#v", store.deactivations)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenImageAspectRatioIsIncompatible(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              36,
			FileName:            "set-e-001.jpg",
			ParentPath:          "/Volumes/media/set-e",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			Width:               1920,
			Height:              1080,
			ImageEmbedding:      "[0.11,0.22,0.33]",
			ImageEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "单人写真", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"单人写真": {
				{FileID: 36, FileName: "set-e-001.jpg", ParentPath: "/Volumes/media/set-e", MediaType: "image", ModTime: now, Width: 1920, Height: 1080, ImageEmbedding: "[0.11,0.22,0.33]", ImageEmbeddingModel: "semantic-v1"},
				{FileID: 37, FileName: "set-e-002.jpg", ParentPath: "/Volumes/media/other", MediaType: "image", ModTime: now.Add(time.Hour), Width: 1080, Height: 1920, ImageEmbedding: "[0.12,0.21,0.34]", ImageEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 36); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected incompatible image aspect ratio auto signal to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for incompatible image aspect ratio, got %#v", store.deactivations)
	}
}

func TestClusterFileNarrowsLargeAITagCandidateSets(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              7,
			FileName:            "model-a-001.jpg",
			ParentPath:          "/Volumes/media/set-a",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingModel: "pixel-v1",
		},
		tags: []sameperson.PersonTag{{Name: "alice", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"alice": {
				{FileID: 7, FileName: "model-a-001.jpg", ParentPath: "/Volumes/media/set-a", MediaType: "image", ModTime: now, ImageEmbedding: "[0.95,0.90,0.85]", ImageEmbeddingModel: "pixel-v1"},
				{FileID: 8, FileName: "model-a-002.jpg", ParentPath: "/Volumes/media/set-b", MediaType: "image", ModTime: now.Add(2 * time.Hour)},
				{FileID: 9, FileName: "other.jpg", ParentPath: "/Volumes/media/set-a", MediaType: "image", ModTime: now.Add(3 * time.Hour)},
				{FileID: 10, FileName: "random.jpg", ParentPath: "/Volumes/media/other", MediaType: "image", ModTime: now.Add(4 * time.Hour)},
				{FileID: 11, FileName: "late.jpg", ParentPath: "/Volumes/media/far", MediaType: "image", ModTime: now.Add(48 * time.Hour), ImageEmbedding: "[0.95,0.89,0.84]", ImageEmbeddingModel: "pixel-v1"},
				{FileID: 12, FileName: "far.jpg", ParentPath: "/Volumes/media/far", MediaType: "image", ModTime: now.Add(72 * time.Hour), ImageEmbedding: "[5.00,5.00,5.00]", ImageEmbeddingModel: "pixel-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 4 {
		t.Fatalf("expected narrowed ai candidate set, got %#v", store.upserts[0].files)
	}
	if store.upserts[0].title == "person:alice" {
		t.Fatalf("expected ai cluster title to be specialized, got %#v", store.upserts[0])
	}
}

func TestClusterFileRejectsAICandidatesWhenImageOrientationConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              13,
			FileName:            "model-c-001.jpg",
			ParentPath:          "/Volumes/media/set-c",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			Width:               1920,
			Height:              1080,
			ImageEmbedding:      "[0.55,0.44,0.33]",
			ImageEmbeddingModel: "pixel-v1",
		},
		tags: []sameperson.PersonTag{{Name: "cara", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"cara": {
				{FileID: 13, FileName: "model-c-001.jpg", ParentPath: "/Volumes/media/set-c", MediaType: "image", ModTime: now, Width: 1920, Height: 1080, ImageEmbedding: "[0.55,0.44,0.33]", ImageEmbeddingModel: "pixel-v1"},
				{FileID: 14, FileName: "elsewhere.jpg", ParentPath: "/Volumes/media/other", MediaType: "image", ModTime: now.Add(2 * time.Hour), Width: 1080, Height: 1920, ImageEmbedding: "[0.56,0.43,0.32]", ImageEmbeddingModel: "pixel-v1"},
				{FileID: 15, FileName: "model-c-002.jpg", ParentPath: "/Volumes/media/other", MediaType: "image", ModTime: now.Add(3 * time.Hour)},
				{FileID: 16, FileName: "other.jpg", ParentPath: "/Volumes/media/set-c", MediaType: "image", ModTime: now.Add(4 * time.Hour)},
				{FileID: 17, FileName: "late.jpg", ParentPath: "/Volumes/media/far", MediaType: "image", ModTime: now.Add(5 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 13); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 3 {
		t.Fatalf("expected orientation-conflicting ai candidate to be filtered, got %#v", store.upserts[0].files)
	}
	for _, file := range store.upserts[0].files {
		if file.FileID == 14 {
			t.Fatalf("expected orientation-conflicting ai candidate to be rejected, got %#v", store.upserts[0].files)
		}
	}
}

func TestClusterFileNarrowsLargeAITagCandidateSetsUsingVideoFrameEmbeddings(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              17,
			FileName:            "model-b-001.jpg",
			ParentPath:          "/Volumes/media/set-b",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.30,0.40,0.50]",
			ImageEmbeddingModel: "semantic-v1",
		},
		tags: []sameperson.PersonTag{{Name: "bob", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"bob": {
				{FileID: 17, FileName: "model-b-001.jpg", ParentPath: "/Volumes/media/set-b", MediaType: "image", ModTime: now, ImageEmbedding: "[0.30,0.40,0.50]", ImageEmbeddingModel: "semantic-v1"},
				{FileID: 18, FileName: "elsewhere.mp4", ParentPath: "/Volumes/media/videos", MediaType: "video", ModTime: now.Add(5 * time.Hour), VideoFrameEmbeddings: []string{"[0.31,0.39,0.49]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 19, FileName: "far.mp4", ParentPath: "/Volumes/media/far", MediaType: "video", ModTime: now.Add(6 * time.Hour), VideoFrameEmbeddings: []string{"[5.00,5.00,5.00]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 20, FileName: "other.mp4", ParentPath: "/Volumes/media/far", MediaType: "video", ModTime: now.Add(7 * time.Hour)},
				{FileID: 21, FileName: "misc.jpg", ParentPath: "/Volumes/media/misc", MediaType: "image", ModTime: now.Add(8 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 17); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 2 || store.upserts[0].files[1].FileID != 18 {
		t.Fatalf("expected video candidate with near frame embedding to remain, got %#v", store.upserts[0].files)
	}
}

func TestClusterFileRejectsAICandidatesWhenVideoOrientationConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   117,
			FileName:                 "model-v-001.mp4",
			ParentPath:               "/Volumes/media/set-v",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			DurationMS:               60_000,
			Width:                    1920,
			Height:                   1080,
			VideoFrameEmbeddings:     []string{"[0.30,0.40,0.50]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		tags: []sameperson.PersonTag{{Name: "video-bob", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"video-bob": {
				{FileID: 117, FileName: "model-v-001.mp4", ParentPath: "/Volumes/media/set-v", MediaType: "video", ModTime: now, DurationMS: 60_000, Width: 1920, Height: 1080, VideoFrameEmbeddings: []string{"[0.30,0.40,0.50]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 118, FileName: "elsewhere.mp4", ParentPath: "/Volumes/media/other", MediaType: "video", ModTime: now.Add(2 * time.Hour), DurationMS: 58_000, Width: 1080, Height: 1920, VideoFrameEmbeddings: []string{"[0.31,0.39,0.49]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 119, FileName: "model-v-002.mp4", ParentPath: "/Volumes/media/other", MediaType: "video", ModTime: now.Add(3 * time.Hour)},
				{FileID: 120, FileName: "other.mp4", ParentPath: "/Volumes/media/set-v", MediaType: "video", ModTime: now.Add(4 * time.Hour)},
				{FileID: 121, FileName: "late.mp4", ParentPath: "/Volumes/media/far", MediaType: "video", ModTime: now.Add(5 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 117); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 3 {
		t.Fatalf("expected orientation-conflicting ai video candidate to be filtered, got %#v", store.upserts[0].files)
	}
	for _, file := range store.upserts[0].files {
		if file.FileID == 118 {
			t.Fatalf("expected orientation-conflicting ai video candidate to be rejected, got %#v", store.upserts[0].files)
		}
	}
}

func TestClusterFileRejectsAICandidatesWhenVideoAspectRatioIsTooFarApart(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   127,
			FileName:                 "model-v-101.mp4",
			ParentPath:               "/Volumes/media/set-v",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			DurationMS:               60_000,
			Width:                    1920,
			Height:                   1080,
			VideoFrameEmbeddings:     []string{"[0.30,0.40,0.50]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		tags: []sameperson.PersonTag{{Name: "video-cara", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"video-cara": {
				{FileID: 127, FileName: "model-v-101.mp4", ParentPath: "/Volumes/media/set-v", MediaType: "video", ModTime: now, DurationMS: 60_000, Width: 1920, Height: 1080, VideoFrameEmbeddings: []string{"[0.30,0.40,0.50]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 128, FileName: "elsewhere.mp4", ParentPath: "/Volumes/media/other", MediaType: "video", ModTime: now.Add(2 * time.Hour), DurationMS: 58_000, Width: 1280, Height: 1024, VideoFrameEmbeddings: []string{"[0.31,0.39,0.49]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 129, FileName: "model-v-102.mp4", ParentPath: "/Volumes/media/other", MediaType: "video", ModTime: now.Add(3 * time.Hour)},
				{FileID: 130, FileName: "other.mp4", ParentPath: "/Volumes/media/set-v", MediaType: "video", ModTime: now.Add(4 * time.Hour)},
				{FileID: 131, FileName: "late.mp4", ParentPath: "/Volumes/media/far", MediaType: "video", ModTime: now.Add(5 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 127); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 3 {
		t.Fatalf("expected aspect-ratio-conflicting ai video candidate to be filtered, got %#v", store.upserts[0].files)
	}
	for _, file := range store.upserts[0].files {
		if file.FileID == 128 {
			t.Fatalf("expected aspect-ratio-conflicting ai video candidate to be rejected, got %#v", store.upserts[0].files)
		}
	}
}

func TestClusterFileRejectsAICandidatesWhenVideoResolutionIsTooFarApart(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   132,
			FileName:                 "model-v-201.mp4",
			ParentPath:               "/Volumes/media/set-v",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			DurationMS:               60_000,
			Width:                    3840,
			Height:                   2160,
			VideoFrameEmbeddings:     []string{"[0.30,0.40,0.50]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		tags: []sameperson.PersonTag{{Name: "video-dora", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"video-dora": {
				{FileID: 132, FileName: "model-v-201.mp4", ParentPath: "/Volumes/media/set-v", MediaType: "video", ModTime: now, DurationMS: 60_000, Width: 3840, Height: 2160, VideoFrameEmbeddings: []string{"[0.30,0.40,0.50]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 133, FileName: "elsewhere.mp4", ParentPath: "/Volumes/media/other", MediaType: "video", ModTime: now.Add(2 * time.Hour), DurationMS: 58_000, Width: 320, Height: 180, VideoFrameEmbeddings: []string{"[0.31,0.39,0.49]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 134, FileName: "model-v-202.mp4", ParentPath: "/Volumes/media/other", MediaType: "video", ModTime: now.Add(3 * time.Hour)},
				{FileID: 135, FileName: "other.mp4", ParentPath: "/Volumes/media/set-v", MediaType: "video", ModTime: now.Add(4 * time.Hour)},
				{FileID: 136, FileName: "late.mp4", ParentPath: "/Volumes/media/far", MediaType: "video", ModTime: now.Add(5 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 132); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 3 {
		t.Fatalf("expected resolution-conflicting ai video candidate to be filtered, got %#v", store.upserts[0].files)
	}
	for _, file := range store.upserts[0].files {
		if file.FileID == 133 {
			t.Fatalf("expected resolution-conflicting ai video candidate to be rejected, got %#v", store.upserts[0].files)
		}
	}
}

func TestClusterFileIgnoresMismatchedEmbeddingModelsForAICandidates(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              17,
			FileName:            "model-b-001.jpg",
			ParentPath:          "/Volumes/media/set-b",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.30,0.40,0.50]",
			ImageEmbeddingModel: "semantic-a",
		},
		tags: []sameperson.PersonTag{{Name: "bob", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"bob": {
				{FileID: 17, FileName: "model-b-001.jpg", ParentPath: "/Volumes/media/set-b", MediaType: "image", ModTime: now, ImageEmbedding: "[0.30,0.40,0.50]", ImageEmbeddingModel: "semantic-a"},
				{FileID: 18, FileName: "elsewhere.mp4", ParentPath: "/Volumes/media/videos", MediaType: "video", ModTime: now.Add(5 * time.Hour), VideoFrameEmbeddings: []string{"[0.31,0.39,0.49]"}, VideoFrameEmbeddingModel: "semantic-b"},
				{FileID: 19, FileName: "misc.jpg", ParentPath: "/Volumes/media/misc", MediaType: "image", ModTime: now.Add(8 * time.Hour), ImageEmbedding: "[0.31,0.39,0.49]", ImageEmbeddingModel: "semantic-b"},
				{FileID: 20, FileName: "model-b-002.jpg", ParentPath: "/Volumes/media/else", MediaType: "image", ModTime: now.Add(2 * time.Hour)},
				{FileID: 21, FileName: "other.jpg", ParentPath: "/Volumes/media/set-b", MediaType: "image", ModTime: now.Add(3 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 17); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	if len(store.upserts[0].files) != 3 {
		t.Fatalf("expected only anchor + filename family + parent path candidates, got %#v", store.upserts[0].files)
	}
}

func TestClusterFilePrefersHumanPersonTagWhenNameDuplicatesExist(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:     7,
			FileName:   "alice-001.jpg",
			ParentPath: "/Volumes/media/alice",
			ModTime:    now,
			Status:     "active",
		},
		tags: []sameperson.PersonTag{
			{Name: "alice", Source: "ai"},
			{Name: "alice", Source: "human"},
		},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"alice": {
				{FileID: 7, FileName: "alice-001.jpg", ParentPath: "/Volumes/media/alice", ModTime: now},
				{FileID: 8, FileName: "alice-002.jpg", ParentPath: "/Volumes/media/alice", ModTime: now.Add(time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 || store.upserts[0].title != "person:alice" {
		t.Fatalf("expected deduped human tag to win, got %#v", store.upserts)
	}
}

func TestClusterFileAssignsHigherScoreToEmbeddingSupportedCandidates(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              41,
			FileName:            "candidate-a-001.jpg",
			ParentPath:          "/Volumes/media/set-a",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.40,0.50,0.60]",
			ImageEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "单人写真", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"单人写真": {
				{FileID: 41, FileName: "candidate-a-001.jpg", ParentPath: "/Volumes/media/set-a", MediaType: "image", ModTime: now, ImageEmbedding: "[0.40,0.50,0.60]", ImageEmbeddingModel: "semantic-v1"},
				{FileID: 42, FileName: "other-name.jpg", ParentPath: "/Volumes/media/random", MediaType: "image", ModTime: now.Add(time.Hour), ImageEmbedding: "[0.41,0.49,0.61]", ImageEmbeddingModel: "semantic-v1"},
				{FileID: 43, FileName: "candidate-a-002.jpg", ParentPath: "/Volumes/media/else", MediaType: "image", ModTime: now.Add(2 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 41); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	files := store.upserts[0].files
	if len(files) != 3 {
		t.Fatalf("expected 3 scored candidates, got %#v", files)
	}
	if files[0].Score < 0.99 {
		t.Fatalf("expected anchor score to be highest, got %#v", files[0])
	}
	if files[1].FileID != 42 || files[2].FileID != 43 {
		t.Fatalf("expected candidates sorted by score, got %#v", files)
	}
	if files[1].Score <= files[2].Score {
		t.Fatalf("expected embedding-supported candidate to outrank filename-only candidate, got %#v", files)
	}
}

func TestClusterFilePrefersStructuredPersonSignalsOverFilenameOnlyCandidate(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              81,
			FileName:            "portrait-a-001.jpg",
			ParentPath:          "/Volumes/media/set-a",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.40,0.50,0.60]",
			ImageEmbeddingModel: "semantic-v1",
			HasFace:             true,
			SubjectCount:        "single",
			CaptureType:         "selfie",
		},
		autoTags: []sameperson.PersonTag{{Name: "单人写真", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"单人写真": {
				{FileID: 81, FileName: "portrait-a-001.jpg", ParentPath: "/Volumes/media/set-a", MediaType: "image", ModTime: now, ImageEmbedding: "[0.40,0.50,0.60]", ImageEmbeddingModel: "semantic-v1", HasFace: true, SubjectCount: "single", CaptureType: "selfie"},
				{FileID: 82, FileName: "other-name.jpg", ParentPath: "/Volumes/media/random", MediaType: "image", ModTime: now.Add(time.Hour), ImageEmbedding: "[0.41,0.49,0.61]", ImageEmbeddingModel: "semantic-v1", HasFace: true, SubjectCount: "single", CaptureType: "selfie"},
				{FileID: 83, FileName: "portrait-a-002.jpg", ParentPath: "/Volumes/media/else", MediaType: "image", ModTime: now.Add(2 * time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 81); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	files := store.upserts[0].files
	if len(files) != 3 {
		t.Fatalf("expected 3 scored candidates, got %#v", files)
	}
	if files[1].FileID != 82 || files[2].FileID != 83 {
		t.Fatalf("expected structured-signal candidate to outrank filename-only candidate, got %#v", files)
	}
	if files[1].Score <= files[2].Score {
		t.Fatalf("expected structured-signal candidate to score higher, got %#v", files)
	}
}

func TestClusterFilePrefersPersonVisualEvidenceOverGenericVisualEvidence(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              88,
			FileName:            "portrait-z-001.jpg",
			ParentPath:          "/Volumes/media/set-z",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.40,0.50,0.60]",
			ImageEmbeddingType:  "person_visual",
			ImageEmbeddingModel: "person-semantic-v1",
		},
		tags: []sameperson.PersonTag{{Name: "zoe", Source: "ai"}},
		candidates: map[string][]sameperson.PersonCandidateFile{
			"zoe": {
				{FileID: 88, FileName: "portrait-z-001.jpg", ParentPath: "/Volumes/media/set-z", MediaType: "image", ModTime: now, ImageEmbedding: "[0.40,0.50,0.60]", ImageEmbeddingType: "person_visual", ImageEmbeddingModel: "person-semantic-v1"},
				{FileID: 89, FileName: "other-a.jpg", ParentPath: "/Volumes/media/random-a", MediaType: "image", ModTime: now.Add(time.Hour), ImageEmbedding: "[0.41,0.49,0.61]", ImageEmbeddingType: "person_visual", ImageEmbeddingModel: "person-semantic-v1"},
				{FileID: 90, FileName: "other-b.jpg", ParentPath: "/Volumes/media/random-b", MediaType: "image", ModTime: now.Add(90 * time.Minute), ImageEmbedding: "[0.41,0.49,0.61]", ImageEmbeddingType: "image_visual", ImageEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 88); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	files := store.upserts[0].files
	if len(files) != 3 {
		t.Fatalf("expected 3 scored candidates, got %#v", files)
	}
	if files[1].FileID != 89 || files[2].FileID != 90 {
		t.Fatalf("expected person_visual candidate to outrank generic visual candidate, got %#v", files)
	}
	if files[1].Score <= files[2].Score {
		t.Fatalf("expected person_visual candidate to score higher, got %#v", files)
	}
}

func TestClusterFilePrefersTimeNearbyCandidateWhenEvidenceOtherwiseSimilar(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:              101,
			FileName:            "session-x-001.jpg",
			ParentPath:          "/Volumes/media/set-x",
			MediaType:           "image",
			ModTime:             now,
			Status:              "active",
			ImageEmbedding:      "[0.40,0.50,0.60]",
			ImageEmbeddingModel: "semantic-v1",
			HasFace:             true,
			SubjectCount:        "single",
		},
		autoTags: []sameperson.PersonTag{{Name: "单人写真", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"单人写真": {
				{FileID: 101, FileName: "session-x-001.jpg", ParentPath: "/Volumes/media/set-x", MediaType: "image", ModTime: now, ImageEmbedding: "[0.40,0.50,0.60]", ImageEmbeddingModel: "semantic-v1", HasFace: true, SubjectCount: "single"},
				{FileID: 102, FileName: "other-a.jpg", ParentPath: "/Volumes/media/random-a", MediaType: "image", ModTime: now.Add(30 * time.Minute), ImageEmbedding: "[0.41,0.49,0.61]", ImageEmbeddingModel: "semantic-v1", HasFace: true, SubjectCount: "single"},
				{FileID: 103, FileName: "other-b.jpg", ParentPath: "/Volumes/media/random-b", MediaType: "image", ModTime: now.Add(5 * 24 * time.Hour), ImageEmbedding: "[0.41,0.49,0.61]", ImageEmbeddingModel: "semantic-v1", HasFace: true, SubjectCount: "single"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 101); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("expected 1 upsert, got %#v", store.upserts)
	}
	files := store.upserts[0].files
	if len(files) != 3 {
		t.Fatalf("expected 3 scored candidates, got %#v", files)
	}
	if files[1].FileID != 102 || files[2].FileID != 103 {
		t.Fatalf("expected time-near candidate to outrank far candidate, got %#v", files)
	}
	if files[1].Score <= files[2].Score {
		t.Fatalf("expected time-near candidate to score higher, got %#v", files)
	}
}

func TestClusterFileDoesNotGroupWeakAutoSignalsWithoutStrongPersonEvidence(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:     51,
			FileName:   "clip-001.mp4",
			ParentPath: "/Volumes/media/set-z",
			MediaType:  "video",
			ModTime:    now,
			Status:     "active",
		},
		autoTags: []sameperson.PersonTag{{Name: "情侣", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"情侣": {
				{FileID: 51, FileName: "clip-001.mp4", ParentPath: "/Volumes/media/set-z", MediaType: "video", ModTime: now},
				{FileID: 52, FileName: "other.mp4", ParentPath: "/Volumes/media/set-z", MediaType: "video", ModTime: now.Add(time.Hour)},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 51); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected weak auto signals without person evidence to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for weak auto signal, got %#v", store.deactivations)
	}
}

func TestClusterFileAllowsWeakAutoSignalsWhenEmbeddingEvidenceExists(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   61,
			FileName:                 "clip-101.mp4",
			ParentPath:               "/Volumes/media/set-k",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "情侣", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"情侣": {
				{FileID: 61, FileName: "clip-101.mp4", ParentPath: "/Volumes/media/set-k", MediaType: "video", ModTime: now, VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 62, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(time.Hour), VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 61); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 1 || len(store.upserts[0].files) != 2 {
		t.Fatalf("expected embedding-backed weak auto signal to cluster, got %#v", store.upserts)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenOnlyEmbeddingMatchesAcrossLongTimeGap(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   71,
			FileName:                 "clip-201.mp4",
			ParentPath:               "/Volumes/media/set-q",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "情侣", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"情侣": {
				{FileID: 71, FileName: "clip-201.mp4", ParentPath: "/Volumes/media/set-q", MediaType: "video", ModTime: now, VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 72, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(45 * 24 * time.Hour), VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 71); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected long-gap weak auto signal to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for long-gap weak auto signal, got %#v", store.deactivations)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenStructuredPersonShapeConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   91,
			FileName:                 "clip-301.mp4",
			ParentPath:               "/Volumes/media/set-y",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			HasFace:                  true,
			SubjectCount:             "single",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "情侣", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"情侣": {
				{FileID: 91, FileName: "clip-301.mp4", ParentPath: "/Volumes/media/set-y", MediaType: "video", ModTime: now, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 92, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(time.Hour), HasFace: true, SubjectCount: "multiple", VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 91); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected conflicting structured person shape to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for conflicting weak auto signal, got %#v", store.deactivations)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenCaptureTypeConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   101,
			FileName:                 "clip-401.mp4",
			ParentPath:               "/Volumes/media/set-c",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			HasFace:                  true,
			SubjectCount:             "single",
			CaptureType:              "selfie",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "情侣", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"情侣": {
				{FileID: 101, FileName: "clip-401.mp4", ParentPath: "/Volumes/media/set-c", MediaType: "video", ModTime: now, HasFace: true, SubjectCount: "single", CaptureType: "selfie", VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 102, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(time.Hour), HasFace: true, SubjectCount: "single", CaptureType: "screenshot", VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 101); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected conflicting capture type to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for conflicting capture type, got %#v", store.deactivations)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenVideoDurationIsIncompatible(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   111,
			FileName:                 "clip-501.mp4",
			ParentPath:               "/Volumes/media/set-d",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			DurationMS:               60_000,
			HasFace:                  true,
			SubjectCount:             "single",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "情侣", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"情侣": {
				{FileID: 111, FileName: "clip-501.mp4", ParentPath: "/Volumes/media/set-d", MediaType: "video", ModTime: now, DurationMS: 60_000, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 112, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(time.Hour), DurationMS: 300_000, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 111); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected incompatible-duration weak auto signal to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for conflicting video duration, got %#v", store.deactivations)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenVideoOrientationConflicts(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   211,
			FileName:                 "clip-601.mp4",
			ParentPath:               "/Volumes/media/set-f",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			DurationMS:               60_000,
			Width:                    1920,
			Height:                   1080,
			HasFace:                  true,
			SubjectCount:             "single",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "多人", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"多人": {
				{FileID: 211, FileName: "clip-601.mp4", ParentPath: "/Volumes/media/set-f", MediaType: "video", ModTime: now, DurationMS: 60_000, Width: 1920, Height: 1080, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 212, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(time.Hour), DurationMS: 58_000, Width: 1080, Height: 1920, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 211); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected orientation-conflicting weak auto video signal to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for conflicting video orientation, got %#v", store.deactivations)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenVideoAspectRatioIsTooFarApart(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   221,
			FileName:                 "clip-701.mp4",
			ParentPath:               "/Volumes/media/set-g",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			DurationMS:               60_000,
			Width:                    1920,
			Height:                   1080,
			HasFace:                  true,
			SubjectCount:             "single",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "多人", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"多人": {
				{FileID: 221, FileName: "clip-701.mp4", ParentPath: "/Volumes/media/set-g", MediaType: "video", ModTime: now, DurationMS: 60_000, Width: 1920, Height: 1080, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 222, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(time.Hour), DurationMS: 58_000, Width: 1280, Height: 1024, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 221); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected aspect-ratio-conflicting weak auto video signal to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for conflicting video aspect ratio, got %#v", store.deactivations)
	}
}

func TestClusterFileRejectsWeakAutoSignalsWhenVideoResolutionIsTooFarApart(t *testing.T) {
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	store := &recordingStore{
		file: sameperson.FileContext{
			FileID:                   231,
			FileName:                 "clip-801.mp4",
			ParentPath:               "/Volumes/media/set-h",
			MediaType:                "video",
			ModTime:                  now,
			Status:                   "active",
			DurationMS:               60_000,
			Width:                    3840,
			Height:                   2160,
			HasFace:                  true,
			SubjectCount:             "single",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]"},
			VideoFrameEmbeddingModel: "semantic-v1",
		},
		autoTags: []sameperson.PersonTag{{Name: "多人", Source: "auto"}},
		autoCandidates: map[string][]sameperson.PersonCandidateFile{
			"多人": {
				{FileID: 231, FileName: "clip-801.mp4", ParentPath: "/Volumes/media/set-h", MediaType: "video", ModTime: now, DurationMS: 60_000, Width: 3840, Height: 2160, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.10,0.20,0.30]"}, VideoFrameEmbeddingModel: "semantic-v1"},
				{FileID: 232, FileName: "other.mp4", ParentPath: "/Volumes/media/random", MediaType: "video", ModTime: now.Add(time.Hour), DurationMS: 58_000, Width: 320, Height: 180, HasFace: true, SubjectCount: "single", VideoFrameEmbeddings: []string{"[0.11,0.21,0.29]"}, VideoFrameEmbeddingModel: "semantic-v1"},
			},
		},
	}
	service := sameperson.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 231); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("expected resolution-conflicting weak auto video signal to avoid upsert, got %#v", store.upserts)
	}
	if len(store.deactivations) != 1 {
		t.Fatalf("expected cluster deactivation for conflicting video resolution, got %#v", store.deactivations)
	}
}

type recordingStore struct {
	file           sameperson.FileContext
	tags           []sameperson.PersonTag
	autoTags       []sameperson.PersonTag
	candidates     map[string][]sameperson.PersonCandidateFile
	autoCandidates map[string][]sameperson.PersonCandidateFile
	upserts        []recordingUpsert
	deactivations  []string
}

type recordingUpsert struct {
	title string
	files []sameperson.PersonCandidateFile
}

func (s *recordingStore) GetFileContext(_ context.Context, _ int64) (sameperson.FileContext, error) {
	return s.file, nil
}

func (s *recordingStore) ListPersonTags(_ context.Context, _ int64) ([]sameperson.PersonTag, error) {
	return s.tags, nil
}

func (s *recordingStore) ListAutoPersonTags(_ context.Context, _ int64) ([]sameperson.PersonTag, error) {
	return s.autoTags, nil
}

func (s *recordingStore) ListFilesWithPersonTag(_ context.Context, tagName string) ([]sameperson.PersonCandidateFile, error) {
	return s.candidates[tagName], nil
}

func (s *recordingStore) ListFilesWithAutoPersonTag(_ context.Context, tagName string) ([]sameperson.PersonCandidateFile, error) {
	return s.autoCandidates[tagName], nil
}

func (s *recordingStore) UpsertSamePersonCluster(_ context.Context, title string, files []sameperson.PersonCandidateFile) error {
	s.upserts = append(s.upserts, recordingUpsert{title: title, files: files})
	return nil
}

func (s *recordingStore) DeactivateSamePersonCluster(_ context.Context, title string) error {
	s.deactivations = append(s.deactivations, title)
	return nil
}
