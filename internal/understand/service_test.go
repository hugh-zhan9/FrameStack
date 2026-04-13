package understand_test

import (
	"context"
	"testing"

	"idea/internal/understand"
)

func TestServiceAnalyzeFileStoresUnderstandingAndTags(t *testing.T) {
	store := &recordingStore{
		file: understand.File{
			ID:        7,
			AbsPath:   "/Volumes/media/photos/poster.jpg",
			FileName:  "poster.jpg",
			MediaType: "image",
		},
	}
	analyzer := &recordingAnalyzer{
		result: understand.Result{
			RawTags: []string{"单人写真", "室内"},
			CanonicalCandidates: []understand.TagCandidate{
				{Namespace: "content", Name: "单人写真", Confidence: 0.92},
				{Namespace: "content", Name: "室内", Confidence: 0.81},
			},
			Summary: "单人室内写真，画面清晰。",
			StructuredAttributes: map[string]any{
				"media_type":  "image",
				"orientation": "portrait",
			},
			Confidence: 0.88,
			Provider:   "placeholder",
			Model:      "placeholder-v1",
		},
	}
	service := understand.Service{
		Store:              store,
		Analyzer:           analyzer,
		SamePersonEnqueuer: &recordingSamePersonEnqueuer{},
	}

	if err := service.AnalyzeFile(context.Background(), 7); err != nil {
		t.Fatalf("expected analyze file to succeed: %v", err)
	}
	if analyzer.request.FileID != 7 || analyzer.request.FilePath != "/Volumes/media/photos/poster.jpg" {
		t.Fatalf("unexpected analyzer request: %#v", analyzer.request)
	}
	if store.analysis.FileID != 7 || store.analysis.AnalysisType != "understanding" {
		t.Fatalf("unexpected analysis input: %#v", store.analysis)
	}
	if len(store.tags) != 2 {
		t.Fatalf("expected 2 tag candidates, got %#v", store.tags)
	}
	if store.analysis.Provider != "placeholder" || store.analysis.ModelName != "placeholder-v1" {
		t.Fatalf("unexpected analysis metadata: %#v", store.analysis)
	}
}

func TestServiceAnalyzeFileEnqueuesSamePersonClustering(t *testing.T) {
	store := &recordingStore{
		file: understand.File{
			ID:        7,
			AbsPath:   "/Volumes/media/photos/poster.jpg",
			FileName:  "poster.jpg",
			MediaType: "image",
		},
	}
	enqueuer := &recordingSamePersonEnqueuer{}
	service := understand.Service{
		Store:              store,
		Analyzer:           &recordingAnalyzer{result: understand.Result{}},
		SamePersonEnqueuer: enqueuer,
	}

	if err := service.AnalyzeFile(context.Background(), 7); err != nil {
		t.Fatalf("expected analyze file to succeed: %v", err)
	}
	if enqueuer.fileID != 7 {
		t.Fatalf("expected same person enqueue for file 7, got %d", enqueuer.fileID)
	}
}

func TestServiceAnalyzeImageEnqueuesPersonEmbeddingWhenSignalsExist(t *testing.T) {
	store := &recordingStore{
		file: understand.File{
			ID:        17,
			AbsPath:   "/Volumes/media/photos/person.jpg",
			FileName:  "person.jpg",
			MediaType: "image",
		},
	}
	personEnqueuer := &recordingPersonEmbeddingEnqueuer{}
	service := understand.Service{
		Store:                  store,
		Analyzer:               &recordingAnalyzer{result: understand.Result{CanonicalCandidates: []understand.TagCandidate{{Namespace: "person", Name: "candidate_a"}}, StructuredAttributes: map[string]any{"has_face": true}}},
		PersonEmbeddingEnqueuer: personEnqueuer,
	}

	if err := service.AnalyzeFile(context.Background(), 17); err != nil {
		t.Fatalf("expected analyze file to succeed: %v", err)
	}
	if personEnqueuer.imageFileID != 17 {
		t.Fatalf("expected person image enqueue for file 17, got %d", personEnqueuer.imageFileID)
	}
}

func TestServiceAnalyzeVideoEnqueuesPersonFrameEmbeddingsWhenSignalsExist(t *testing.T) {
	store := &recordingStore{
		file: understand.File{
			ID:        19,
			AbsPath:   "/Volumes/media/videos/person.mp4",
			FileName:  "person.mp4",
			MediaType: "video",
		},
	}
	personEnqueuer := &recordingPersonEmbeddingEnqueuer{}
	service := understand.Service{
		Store:                  store,
		Analyzer:               &recordingAnalyzer{result: understand.Result{StructuredAttributes: map[string]any{"subject_count": "single", "has_face": true}}},
		PersonEmbeddingEnqueuer: personEnqueuer,
	}

	if err := service.AnalyzeFile(context.Background(), 19); err != nil {
		t.Fatalf("expected analyze file to succeed: %v", err)
	}
	if personEnqueuer.videoFileID != 19 {
		t.Fatalf("expected person video enqueue for file 19, got %d", personEnqueuer.videoFileID)
	}
}

func TestServiceAnalyzeFileSkipsPersonEmbeddingWhenNoSignalsExist(t *testing.T) {
	store := &recordingStore{
		file: understand.File{
			ID:        23,
			AbsPath:   "/Volumes/media/photos/landscape.jpg",
			FileName:  "landscape.jpg",
			MediaType: "image",
		},
	}
	personEnqueuer := &recordingPersonEmbeddingEnqueuer{}
	service := understand.Service{
		Store:                  store,
		Analyzer:               &recordingAnalyzer{result: understand.Result{StructuredAttributes: map[string]any{"media_type": "image"}}},
		PersonEmbeddingEnqueuer: personEnqueuer,
	}

	if err := service.AnalyzeFile(context.Background(), 23); err != nil {
		t.Fatalf("expected analyze file to succeed: %v", err)
	}
	if personEnqueuer.imageFileID != 0 || personEnqueuer.videoFileID != 0 {
		t.Fatalf("expected no person embedding enqueue, got %#v", personEnqueuer)
	}
}

func TestServiceAnalyzeVideoIncludesFramePaths(t *testing.T) {
	store := &recordingStore{
		file: understand.File{
			ID:        9,
			AbsPath:   "/Volumes/media/videos/clip.mp4",
			FileName:  "clip.mp4",
			MediaType: "video",
			FramePaths: []string{
				"/tmp/previews/9/frame-1.jpg",
				"/tmp/previews/9/frame-2.jpg",
			},
		},
	}
	analyzer := &recordingAnalyzer{}
	service := understand.Service{
		Store:    store,
		Analyzer: analyzer,
	}

	if err := service.AnalyzeFile(context.Background(), 9); err != nil {
		t.Fatalf("expected analyze file to succeed: %v", err)
	}
	if len(analyzer.request.FramePaths) != 2 {
		t.Fatalf("expected 2 frame paths, got %#v", analyzer.request)
	}
}

type recordingStore struct {
	file     understand.File
	analysis understand.AnalysisInput
	tags     []understand.TagCandidate
}

func (s *recordingStore) GetFile(_ context.Context, _ int64) (understand.File, error) {
	return s.file, nil
}

func (s *recordingStore) UpsertAnalysis(_ context.Context, input understand.AnalysisInput) error {
	s.analysis = input
	return nil
}

func (s *recordingStore) ReplaceAITags(_ context.Context, fileID int64, tags []understand.TagCandidate) error {
	s.tags = append([]understand.TagCandidate(nil), tags...)
	return nil
}

type recordingAnalyzer struct {
	request understand.Request
	result  understand.Result
	err     error
}

func (a *recordingAnalyzer) UnderstandMedia(_ context.Context, req understand.Request) (understand.Result, error) {
	a.request = req
	return a.result, a.err
}

type recordingSamePersonEnqueuer struct {
	fileID int64
}

func (e *recordingSamePersonEnqueuer) EnqueueSamePerson(_ context.Context, fileID int64) error {
	e.fileID = fileID
	return nil
}

type recordingPersonEmbeddingEnqueuer struct {
	imageFileID int64
	videoFileID int64
}

func (e *recordingPersonEmbeddingEnqueuer) EnqueuePersonImageEmbedding(_ context.Context, fileID int64) error {
	e.imageFileID = fileID
	return nil
}

func (e *recordingPersonEmbeddingEnqueuer) EnqueuePersonVideoFrameEmbeddings(_ context.Context, fileID int64) error {
	e.videoFileID = fileID
	return nil
}
