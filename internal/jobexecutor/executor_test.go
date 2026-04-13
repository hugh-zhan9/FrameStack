package jobexecutor_test

import (
	"context"
	"errors"
	"testing"

	"idea/internal/jobexecutor"
	"idea/internal/scanner"
	"idea/internal/tasks"
)

func TestExecutorRunsScanVolumeJob(t *testing.T) {
	scanner := &recordingScanner{}
	executor := jobexecutor.Executor{
		Scanner:  scanner,
		Fallback: tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       1,
		JobType:  "scan_volume",
		TargetID: 7,
	})
	if err != nil {
		t.Fatalf("expected executor to succeed: %v", err)
	}
	if scanner.volumeID != 7 {
		t.Fatalf("expected scanner to receive volume 7, got %d", scanner.volumeID)
	}
}

func TestExecutorReturnsScannerError(t *testing.T) {
	executor := jobexecutor.Executor{
		Scanner: &recordingScanner{err: errors.New("offline")},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		JobType:  "scan_volume",
		TargetID: 7,
	})
	if err == nil {
		t.Fatal("expected executor to fail")
	}
}

func TestExecutorRunsExtractImageFeaturesJob(t *testing.T) {
	extractor := &recordingMediaExtractor{}
	executor := jobexecutor.Executor{
		Extractor: extractor,
		Fallback:  tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       2,
		JobType:  "extract_image_features",
		TargetID: 101,
	})
	if err != nil {
		t.Fatalf("expected extractor to succeed: %v", err)
	}
	if extractor.imageFileID != 101 {
		t.Fatalf("expected image extractor to receive file 101, got %d", extractor.imageFileID)
	}
}

func TestExecutorRunsExtractVideoFeaturesJob(t *testing.T) {
	extractor := &recordingMediaExtractor{}
	executor := jobexecutor.Executor{
		Extractor: extractor,
		Fallback:  tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       3,
		JobType:  "extract_video_features",
		TargetID: 202,
	})
	if err != nil {
		t.Fatalf("expected extractor to succeed: %v", err)
	}
	if extractor.videoFileID != 202 {
		t.Fatalf("expected video extractor to receive file 202, got %d", extractor.videoFileID)
	}
}

func TestExecutorRunsRecomputeSearchDocumentJob(t *testing.T) {
	indexer := &recordingSearchDocumentIndexer{}
	executor := jobexecutor.Executor{
		SearchIndexer: indexer,
		Fallback:      tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       4,
		JobType:  "recompute_search_doc",
		TargetID: 303,
	})
	if err != nil {
		t.Fatalf("expected search doc indexer to succeed: %v", err)
	}
	if indexer.fileID != 303 {
		t.Fatalf("expected search doc indexer to receive file 303, got %d", indexer.fileID)
	}
}

func TestExecutorRunsInferTagsJob(t *testing.T) {
	understander := &recordingUnderstandService{}
	executor := jobexecutor.Executor{
		Understander: understander,
		Fallback:     tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       5,
		JobType:  "infer_tags",
		TargetID: 404,
	})
	if err != nil {
		t.Fatalf("expected infer_tags to succeed: %v", err)
	}
	if understander.fileID != 404 {
		t.Fatalf("expected understand service to receive file 404, got %d", understander.fileID)
	}
}

func TestExecutorRunsInferQualityJob(t *testing.T) {
	qualityService := &recordingQualityService{}
	executor := jobexecutor.Executor{
		QualityEvaluator: qualityService,
		Fallback:         tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       6,
		JobType:  "infer_quality",
		TargetID: 505,
	})
	if err != nil {
		t.Fatalf("expected infer_quality to succeed: %v", err)
	}
	if qualityService.fileID != 505 {
		t.Fatalf("expected quality service to receive file 505, got %d", qualityService.fileID)
	}
}

func TestExecutorRunsHashFileJob(t *testing.T) {
	hashService := &recordingFileHashService{}
	executor := jobexecutor.Executor{
		FileHasher: hashService,
		Fallback:   tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       7,
		JobType:  "hash_file",
		TargetID: 606,
	})
	if err != nil {
		t.Fatalf("expected hash_file to succeed: %v", err)
	}
	if hashService.fileID != 606 {
		t.Fatalf("expected file hash service to receive file 606, got %d", hashService.fileID)
	}
}

func TestExecutorRunsClusterSameContentJob(t *testing.T) {
	clusterService := &recordingSameContentService{}
	executor := jobexecutor.Executor{
		SameContent: clusterService,
		Fallback:    tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       8,
		JobType:  "cluster_same_content",
		TargetID: 707,
	})
	if err != nil {
		t.Fatalf("expected cluster_same_content to succeed: %v", err)
	}
	if clusterService.fileID != 707 {
		t.Fatalf("expected same content service to receive file 707, got %d", clusterService.fileID)
	}
}

func TestExecutorRunsClusterSameSeriesJob(t *testing.T) {
	clusterService := &recordingSameSeriesService{}
	executor := jobexecutor.Executor{
		SameSeries: clusterService,
		Fallback:   tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       9,
		JobType:  "cluster_same_series",
		TargetID: 808,
	})
	if err != nil {
		t.Fatalf("expected cluster_same_series to succeed: %v", err)
	}
	if clusterService.fileID != 808 {
		t.Fatalf("expected same series service to receive file 808, got %d", clusterService.fileID)
	}
}

func TestExecutorRunsClusterSamePersonJob(t *testing.T) {
	clusterService := &recordingSamePersonService{}
	executor := jobexecutor.Executor{
		SamePerson: clusterService,
		Fallback:   tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       10,
		JobType:  "cluster_same_person",
		TargetID: 909,
	})
	if err != nil {
		t.Fatalf("expected cluster_same_person to succeed: %v", err)
	}
	if clusterService.fileID != 909 {
		t.Fatalf("expected same person service to receive file 909, got %d", clusterService.fileID)
	}
}

func TestExecutorRunsEmbedImageJob(t *testing.T) {
	embeddingsService := &recordingEmbeddingsService{}
	executor := jobexecutor.Executor{
		Embeddings: embeddingsService,
		Fallback:   tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       11,
		JobType:  "embed_image",
		TargetID: 1001,
	})
	if err != nil {
		t.Fatalf("expected embed_image to succeed: %v", err)
	}
	if embeddingsService.imageFileID != 1001 {
		t.Fatalf("expected embed_image to receive file 1001, got %d", embeddingsService.imageFileID)
	}
}

func TestExecutorRunsEmbedVideoFramesJob(t *testing.T) {
	embeddingsService := &recordingEmbeddingsService{}
	executor := jobexecutor.Executor{
		Embeddings: embeddingsService,
		Fallback:   tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       12,
		JobType:  "embed_video_frames",
		TargetID: 1002,
	})
	if err != nil {
		t.Fatalf("expected embed_video_frames to succeed: %v", err)
	}
	if embeddingsService.videoFileID != 1002 {
		t.Fatalf("expected embed_video_frames to receive file 1002, got %d", embeddingsService.videoFileID)
	}
}

func TestExecutorRunsEmbedPersonImageJob(t *testing.T) {
	embeddingsService := &recordingEmbeddingsService{}
	executor := jobexecutor.Executor{
		Embeddings: embeddingsService,
		Fallback:   tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       13,
		JobType:  "embed_person_image",
		TargetID: 1003,
	})
	if err != nil {
		t.Fatalf("expected embed_person_image to succeed: %v", err)
	}
	if embeddingsService.personImageFileID != 1003 {
		t.Fatalf("expected embed_person_image to receive file 1003, got %d", embeddingsService.personImageFileID)
	}
}

func TestExecutorRunsEmbedPersonVideoFramesJob(t *testing.T) {
	embeddingsService := &recordingEmbeddingsService{}
	executor := jobexecutor.Executor{
		Embeddings: embeddingsService,
		Fallback:   tasks.NoopExecutor{},
	}

	err := executor.ExecuteJob(context.Background(), tasks.Job{
		ID:       14,
		JobType:  "embed_person_video_frames",
		TargetID: 1004,
	})
	if err != nil {
		t.Fatalf("expected embed_person_video_frames to succeed: %v", err)
	}
	if embeddingsService.personVideoFileID != 1004 {
		t.Fatalf("expected embed_person_video_frames to receive file 1004, got %d", embeddingsService.personVideoFileID)
	}
}

type recordingScanner struct {
	volumeID int64
	err      error
}

func (s *recordingScanner) ScanVolume(_ context.Context, volumeID int64) (scanner.Stats, error) {
	s.volumeID = volumeID
	if s.err != nil {
		return scanner.Stats{}, s.err
	}
	return scanner.Stats{Discovered: 1}, nil
}

type recordingMediaExtractor struct {
	imageFileID int64
	videoFileID int64
	err         error
}

func (e *recordingMediaExtractor) ExtractImageFeatures(_ context.Context, fileID int64) error {
	e.imageFileID = fileID
	return e.err
}

func (e *recordingMediaExtractor) ExtractVideoFeatures(_ context.Context, fileID int64) error {
	e.videoFileID = fileID
	return e.err
}

type recordingSearchDocumentIndexer struct {
	fileID int64
	err    error
}

func (i *recordingSearchDocumentIndexer) RecomputeSearchDocument(_ context.Context, fileID int64) error {
	i.fileID = fileID
	return i.err
}

type recordingUnderstandService struct {
	fileID int64
	err    error
}

func (s *recordingUnderstandService) AnalyzeFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}

type recordingQualityService struct {
	fileID int64
	err    error
}

type recordingEmbeddingsService struct {
	imageFileID int64
	videoFileID int64
	personImageFileID int64
	personVideoFileID int64
	err         error
}

func (e *recordingEmbeddingsService) EmbedImage(_ context.Context, fileID int64) error {
	e.imageFileID = fileID
	return e.err
}

func (e *recordingEmbeddingsService) EmbedVideoFrames(_ context.Context, fileID int64) error {
	e.videoFileID = fileID
	return e.err
}

func (e *recordingEmbeddingsService) EmbedPersonImage(_ context.Context, fileID int64) error {
	e.personImageFileID = fileID
	return e.err
}

func (e *recordingEmbeddingsService) EmbedPersonVideoFrames(_ context.Context, fileID int64) error {
	e.personVideoFileID = fileID
	return e.err
}

func (s *recordingQualityService) EvaluateFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}

type recordingFileHashService struct {
	fileID int64
	err    error
}

func (s *recordingFileHashService) HashFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}

type recordingSameContentService struct {
	fileID int64
	err    error
}

func (s *recordingSameContentService) ClusterFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}

type recordingSameSeriesService struct {
	fileID int64
	err    error
}

func (s *recordingSameSeriesService) ClusterFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}

type recordingSamePersonService struct {
	fileID int64
	err    error
}

func (s *recordingSamePersonService) ClusterFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}
