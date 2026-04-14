package samecontent_test

import (
	"context"
	"testing"

	"idea/internal/samecontent"
)

func TestServiceClusterFileCreatesCandidateForDuplicateSHA(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID: 7,
			SHA256: "abc",
		},
		duplicates: []samecontent.DuplicateFile{
			{FileID: 7, QualityScore: 61, Width: 1280, Height: 720, SizeBytes: 1_000_000},
			{FileID: 8, QualityScore: 84, Width: 1920, Height: 1080, SizeBytes: 2_000_000},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if store.clusterHash != "abc" || len(store.clusterFiles) != 2 {
		t.Fatalf("expected duplicate cluster to be upserted, got hash=%q files=%#v", store.clusterHash, store.clusterFiles)
	}
	if store.clusterFiles[0].FileID != 8 || store.clusterFiles[0].Role != "best_quality" {
		t.Fatalf("expected best quality file to be first and marked, got %#v", store.clusterFiles)
	}
	if store.clusterFiles[1].Role != "duplicate_candidate" {
		t.Fatalf("expected non-primary duplicate to be marked duplicate_candidate, got %#v", store.clusterFiles[1])
	}
	if store.clusterFiles[0].Score != 1 || store.clusterFiles[1].Score <= 0 || store.clusterFiles[1].Score >= 1 {
		t.Fatalf("expected duplicate scores to be normalized into (0,1], got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileSkipsUniqueSHA(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID: 7,
			SHA256: "abc",
		},
		duplicates: []samecontent.DuplicateFile{
			{FileID: 7},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if store.deactivatedHash != "abc" {
		t.Fatalf("expected duplicate cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileFallsBackToImagePHash(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:     7,
			MediaType:  "image",
			ImagePHash: "aaaaaaaaaaaaaaaa",
		},
		similarImages: []samecontent.ImageCandidate{
			{FileID: 7, PHash: "aaaaaaaaaaaaaaaa"},
			{FileID: 9, PHash: "aaaaaaaaaaaaaaaa"},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if store.clusterHash != samecontent.ImageClusterKey("aaaaaaaaaaaaaaaa") || len(store.clusterFiles) != 2 {
		t.Fatalf("expected image phash cluster, got %#v", store)
	}
}

func TestServiceClusterFileFallsBackToImageEmbedding(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:              7,
			MediaType:           "image",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		similarImageEmbeddings: []samecontent.ImageCandidate{
			{FileID: 7, Embedding: "[0.95,0.90,0.85]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1"},
			{FileID: 12, Embedding: "[0.95,0.89,0.84]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1"},
			{FileID: 13, Embedding: "[5.00,5.00,5.00]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1"},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 12 {
		t.Fatalf("expected image embedding cluster, got %#v", store.clusterFiles)
	}
	if store.lastImageEmbeddingModel != "pixel-v1" {
		t.Fatalf("expected image embedding model to be forwarded, got %q", store.lastImageEmbeddingModel)
	}
}

func TestServiceClusterFileIgnoresImageEmbeddingCandidatesFromDifferentTypes(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:              7,
			MediaType:           "image",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		similarImageEmbeddings: []samecontent.ImageCandidate{
			{FileID: 7, Embedding: "[0.95,0.90,0.85]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1"},
			{FileID: 12, Embedding: "[0.95,0.89,0.84]", EmbeddingType: "person_visual", EmbeddingModel: "pixel-v1"},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected different embedding types to be ignored, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileRejectsImageEmbeddingMatchesWhenAspectRatioIsIncompatible(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:              7,
			MediaType:           "image",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		similarImageEmbeddings: []samecontent.ImageCandidate{
			{FileID: 7, Embedding: "[0.95,0.90,0.85]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1", Width: 1920, Height: 1080},
			{FileID: 12, Embedding: "[0.95,0.89,0.84]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1", Width: 1080, Height: 1920},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected incompatible aspect ratio image embedding candidates to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedHash != samecontent.ImageEmbeddingClusterKey("[0.95,0.90,0.85]") {
		t.Fatalf("expected image embedding cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsImageEmbeddingMatchesWhenResolutionIsTooFarApart(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:              7,
			MediaType:           "image",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "pixel-v1",
		},
		similarImageEmbeddings: []samecontent.ImageCandidate{
			{FileID: 7, Embedding: "[0.95,0.90,0.85]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1", Width: 3840, Height: 2160},
			{FileID: 12, Embedding: "[0.95,0.89,0.84]", EmbeddingType: "image_visual", EmbeddingModel: "pixel-v1", Width: 320, Height: 180},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected extreme resolution gap image embedding candidates to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedHash != samecontent.ImageEmbeddingClusterKey("[0.95,0.90,0.85]") {
		t.Fatalf("expected image embedding cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFilePrefersHigherResolutionWhenQualityScoreMissing(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:     7,
			MediaType:  "image",
			ImagePHash: "aaaaaaaaaaaaaaaa",
		},
		similarImages: []samecontent.ImageCandidate{
			{FileID: 7, PHash: "aaaaaaaaaaaaaaaa"},
			{FileID: 9, PHash: "aaaaaaaaaaaaaaaa"},
		},
		duplicatesByID: map[int64]samecontent.DuplicateFile{
			7: {FileID: 7, Width: 1280, Height: 720, SizeBytes: 1_000_000},
			9: {FileID: 9, Width: 3840, Height: 2160, SizeBytes: 8_000_000},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[0].FileID != 9 {
		t.Fatalf("expected higher resolution file to be preferred, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileAcceptsNearImagePHashMatch(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:     7,
			MediaType:  "image",
			ImagePHash: "ffffffffffffffff",
		},
		similarImages: []samecontent.ImageCandidate{
			{FileID: 7, PHash: "ffffffffffffffff"},
			{FileID: 10, PHash: "fffffffffffffffe"},
			{FileID: 11, PHash: "0000000000000000"},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 10 {
		t.Fatalf("expected near image phash match only, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileRejectsImagePHashMatchesWhenOrientationConflicts(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:     7,
			MediaType:  "image",
			Width:      1920,
			Height:     1080,
			ImagePHash: "ffffffffffffffff",
		},
		similarImages: []samecontent.ImageCandidate{
			{FileID: 7, PHash: "ffffffffffffffff", Width: 1920, Height: 1080},
			{FileID: 10, PHash: "fffffffffffffffe", Width: 1080, Height: 1920},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected orientation-conflicting image phash match to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedHash != samecontent.ImageClusterKey("ffffffffffffffff") {
		t.Fatalf("expected image phash cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileUsesRealImageAnchorForOrientationChecks(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:     7,
			MediaType:  "image",
			Width:      1920,
			Height:     1080,
			ImagePHash: "ffffffffffffffff",
		},
		similarImages: []samecontent.ImageCandidate{
			{FileID: 10, PHash: "fffffffffffffffe", Width: 1080, Height: 1920},
			{FileID: 7, PHash: "ffffffffffffffff", Width: 1920, Height: 1080},
			{FileID: 11, PHash: "fffffffffffffffd", Width: 1280, Height: 720},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 11 {
		t.Fatalf("expected real anchor dimensions to keep landscape candidate only, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileFallsBackToVideoFramePHashes(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:            11,
			MediaType:         "video",
			DurationMS:        60_000,
			VideoFramePHashes: []string{"f300", "f100", "f200"},
		},
		similarVideos: []samecontent.DuplicateFile{
			{FileID: 11, DurationMS: 60_000},
			{FileID: 12, DurationMS: 58_000},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 11); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if store.clusterHash != samecontent.VideoClusterKey([]string{"f300", "f100", "f200"}) || len(store.clusterFiles) != 2 {
		t.Fatalf("expected video frame phash cluster, got %#v", store)
	}
}

func TestServiceClusterFileRejectsVideoFramePHashMatchesWhenDurationIsIncompatible(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:            11,
			MediaType:         "video",
			DurationMS:        60_000,
			VideoFramePHashes: []string{"f300", "f100", "f200"},
		},
		similarVideos: []samecontent.DuplicateFile{
			{FileID: 11, DurationMS: 60_000},
			{FileID: 12, DurationMS: 300_000},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 11); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected incompatible-duration video phash candidates to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedHash != samecontent.VideoClusterKey([]string{"f300", "f100", "f200"}) {
		t.Fatalf("expected video phash cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileFallsBackToVideoFrameEmbeddings(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:                   21,
			MediaType:                "video",
			DurationMS:               60_000,
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"},
		},
		similarVideoEmbeddings: []samecontent.DuplicateFile{
			{FileID: 21, DurationMS: 60_000},
			{FileID: 22, DurationMS: 58_000},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 21); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 22 {
		t.Fatalf("expected video embedding cluster, got %#v", store.clusterFiles)
	}
	if store.lastVideoEmbeddingModel != "semantic-v1" {
		t.Fatalf("expected video embedding model to be forwarded, got %q", store.lastVideoEmbeddingModel)
	}
}

func TestServiceClusterFileRejectsVideoEmbeddingMatchesWhenDurationIsIncompatible(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:                   21,
			MediaType:                "video",
			DurationMS:               60_000,
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"},
		},
		similarVideoEmbeddings: []samecontent.DuplicateFile{
			{FileID: 21, DurationMS: 60_000},
			{FileID: 22, DurationMS: 300_000},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 21); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected incompatible-duration video embedding candidates to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedHash != samecontent.VideoEmbeddingClusterKey([]string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"}) {
		t.Fatalf("expected embedding cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsVideoEmbeddingMatchesWhenOrientationConflicts(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:                   21,
			MediaType:                "video",
			DurationMS:               60_000,
			Width:                    1920,
			Height:                   1080,
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"},
		},
		similarVideoEmbeddings: []samecontent.DuplicateFile{
			{FileID: 21, DurationMS: 60_000, Width: 1920, Height: 1080},
			{FileID: 22, DurationMS: 58_000, Width: 1080, Height: 1920},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 21); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected orientation-conflicting video embedding candidates to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedHash != samecontent.VideoEmbeddingClusterKey([]string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"}) {
		t.Fatalf("expected embedding cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileRejectsVideoEmbeddingMatchesWhenAspectRatioIsTooFarApart(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:                   21,
			MediaType:                "video",
			DurationMS:               60_000,
			Width:                    1920,
			Height:                   1080,
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"},
		},
		similarVideoEmbeddings: []samecontent.DuplicateFile{
			{FileID: 21, DurationMS: 60_000, Width: 1920, Height: 1080},
			{FileID: 22, DurationMS: 58_000, Width: 1440, Height: 1080},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 21); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected aspect-ratio-conflicting video embedding candidates to be rejected, got %#v", store.clusterFiles)
	}
	if store.deactivatedHash != samecontent.VideoEmbeddingClusterKey([]string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"}) {
		t.Fatalf("expected embedding cluster to be deactivated, got %#v", store)
	}
}

func TestServiceClusterFileUsesRealVideoAnchorForOrientationChecks(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:                   21,
			MediaType:                "video",
			DurationMS:               60_000,
			Width:                    1920,
			Height:                   1080,
			VideoFrameEmbeddingType:  "video_frame_visual",
			VideoFrameEmbeddingModel: "semantic-v1",
			VideoFrameEmbeddings:     []string{"[0.10,0.20,0.30]", "[0.80,0.75,0.70]"},
		},
		similarVideoEmbeddings: []samecontent.DuplicateFile{
			{FileID: 22, DurationMS: 58_000, Width: 1080, Height: 1920},
			{FileID: 21, DurationMS: 60_000, Width: 1920, Height: 1080},
			{FileID: 23, DurationMS: 59_000, Width: 1280, Height: 720},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 21); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[1].FileID != 23 {
		t.Fatalf("expected real anchor dimensions to keep landscape candidate only, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFilePrefersHigherBitrateVideoWhenQualityIsClose(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:            11,
			MediaType:         "video",
			VideoFramePHashes: []string{"f300", "f100", "f200"},
		},
		similarVideos: []samecontent.DuplicateFile{
			{FileID: 11},
			{FileID: 12},
		},
		duplicatesByID: map[int64]samecontent.DuplicateFile{
			11: {FileID: 11, QualityScore: 82, Width: 1920, Height: 1080, DurationMS: 3_600_000, SizeBytes: 1_200_000_000, Bitrate: 4_000_000, FPS: 24, Container: "mp4"},
			12: {FileID: 12, QualityScore: 82, Width: 1920, Height: 1080, DurationMS: 3_600_000, SizeBytes: 1_400_000_000, Bitrate: 8_000_000, FPS: 30, Container: "mkv"},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 11); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 2 || store.clusterFiles[0].FileID != 12 || store.clusterFiles[0].Role != "best_quality" {
		t.Fatalf("expected higher bitrate video to be preferred, got %#v", store.clusterFiles)
	}
}

func TestServiceClusterFileSkipsImageEmbeddingCandidatesWhenModelDiffers(t *testing.T) {
	store := &recordingStore{
		file: samecontent.FileHash{
			FileID:              7,
			MediaType:           "image",
			ImageEmbedding:      "[0.95,0.90,0.85]",
			ImageEmbeddingType:  "image_visual",
			ImageEmbeddingModel: "semantic-a",
		},
		similarImageEmbeddings: []samecontent.ImageCandidate{
			{FileID: 7, Embedding: "[0.95,0.90,0.85]", EmbeddingType: "image_visual", EmbeddingModel: "semantic-b"},
			{FileID: 12, Embedding: "[0.95,0.89,0.84]", EmbeddingType: "image_visual", EmbeddingModel: "semantic-b"},
		},
	}
	service := samecontent.Service{Store: store}

	if err := service.ClusterFile(context.Background(), 7); err != nil {
		t.Fatalf("expected cluster file to succeed: %v", err)
	}
	if len(store.clusterFiles) != 0 {
		t.Fatalf("expected model-mismatched image embeddings to be ignored, got %#v", store.clusterFiles)
	}
}

type recordingStore struct {
	file                    samecontent.FileHash
	duplicates              []samecontent.DuplicateFile
	duplicatesByID          map[int64]samecontent.DuplicateFile
	similarImages           []samecontent.ImageCandidate
	similarImageEmbeddings  []samecontent.ImageCandidate
	similarVideos           []samecontent.DuplicateFile
	similarVideoEmbeddings  []samecontent.DuplicateFile
	clusterHash             string
	clusterFiles            []samecontent.DuplicateFile
	deactivatedHash         string
	lastImageEmbeddingModel string
	lastVideoEmbeddingModel string
}

func (s *recordingStore) GetFileHash(_ context.Context, _ int64) (samecontent.FileHash, error) {
	return s.file, nil
}

func (s *recordingStore) ListDuplicateFiles(_ context.Context, _ string) ([]samecontent.DuplicateFile, error) {
	return s.enrichDuplicates(s.duplicates), nil
}

func (s *recordingStore) ListImagePHashCandidates(_ context.Context, _ string) ([]samecontent.ImageCandidate, error) {
	items := append([]samecontent.ImageCandidate(nil), s.similarImages...)
	for index := range items {
		if extra, ok := s.duplicatesByID[items[index].FileID]; ok {
			items[index].QualityScore = extra.QualityScore
			items[index].QualityTier = extra.QualityTier
			items[index].Width = extra.Width
			items[index].Height = extra.Height
			items[index].SizeBytes = extra.SizeBytes
		}
	}
	return items, nil
}

func (s *recordingStore) ListImageEmbeddingCandidates(_ context.Context, _ string, model string) ([]samecontent.ImageCandidate, error) {
	s.lastImageEmbeddingModel = model
	items := append([]samecontent.ImageCandidate(nil), s.similarImageEmbeddings...)
	for index := range items {
		if extra, ok := s.duplicatesByID[items[index].FileID]; ok {
			items[index].QualityScore = extra.QualityScore
			items[index].QualityTier = extra.QualityTier
			items[index].Width = extra.Width
			items[index].Height = extra.Height
			items[index].SizeBytes = extra.SizeBytes
		}
	}
	return items, nil
}

func (s *recordingStore) ListVideoFramePHashMatches(_ context.Context, _ []string) ([]samecontent.DuplicateFile, error) {
	return s.enrichDuplicates(s.similarVideos), nil
}

func (s *recordingStore) ListVideoFrameEmbeddingMatches(_ context.Context, _ []string, model string) ([]samecontent.DuplicateFile, error) {
	s.lastVideoEmbeddingModel = model
	return s.enrichDuplicates(s.similarVideoEmbeddings), nil
}

func (s *recordingStore) UpsertSameContentCluster(_ context.Context, sha string, files []samecontent.DuplicateFile) error {
	s.clusterHash = sha
	s.clusterFiles = append([]samecontent.DuplicateFile(nil), files...)
	return nil
}

func (s *recordingStore) DeactivateSameContentCluster(_ context.Context, sha string) error {
	s.deactivatedHash = sha
	return nil
}

func (s *recordingStore) enrichDuplicates(items []samecontent.DuplicateFile) []samecontent.DuplicateFile {
	if len(items) == 0 {
		return nil
	}
	if len(s.duplicatesByID) == 0 {
		return append([]samecontent.DuplicateFile(nil), items...)
	}
	result := make([]samecontent.DuplicateFile, 0, len(items))
	for _, item := range items {
		if extra, ok := s.duplicatesByID[item.FileID]; ok {
			extra.FileID = item.FileID
			result = append(result, extra)
			continue
		}
		result = append(result, item)
	}
	return result
}
