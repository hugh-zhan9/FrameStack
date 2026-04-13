package embeddings_test

import (
	"context"
	"errors"
	"testing"

	"idea/internal/embeddings"
)

func TestServiceEmbedImageStoresVectorFromPHash(t *testing.T) {
	store := &recordingStore{
		image: embeddings.ImageSource{
			FileID: 7,
			PHash:  "0123456789abcdef",
		},
	}
	service := embeddings.Service{Store: store}

	if err := service.EmbedImage(context.Background(), 7); err != nil {
		t.Fatalf("expected image embedding to succeed: %v", err)
	}
	if store.fileEmbedding.FileID != 7 {
		t.Fatalf("unexpected file id: %#v", store.fileEmbedding)
	}
	if store.fileEmbedding.EmbeddingType != "image_visual" {
		t.Fatalf("unexpected embedding type: %#v", store.fileEmbedding)
	}
	if store.fileEmbedding.ModelName != "phash-v1" {
		t.Fatalf("unexpected model name: %#v", store.fileEmbedding)
	}
	if store.fileEmbedding.Vector == "" {
		t.Fatalf("expected vector to be populated: %#v", store.fileEmbedding)
	}
}

func TestServiceEmbedImageSkipsWhenPHashMissing(t *testing.T) {
	store := &recordingStore{
		image: embeddings.ImageSource{FileID: 8},
	}
	service := embeddings.Service{Store: store}

	if err := service.EmbedImage(context.Background(), 8); err != nil {
		t.Fatalf("expected image embedding to skip cleanly: %v", err)
	}
	if store.fileEmbedding.FileID != 0 {
		t.Fatalf("expected no embedding write, got %#v", store.fileEmbedding)
	}
}

func TestServiceEmbedVideoFramesReplacesFrameVectors(t *testing.T) {
	store := &recordingStore{
		frames: []embeddings.VideoFrameSource{
			{FrameID: 11, FileID: 9, PHash: "aaaaaaaaaaaaaaaa"},
			{FrameID: 12, FileID: 9, PHash: ""},
			{FrameID: 13, FileID: 9, PHash: "bbbbbbbbbbbbbbbb"},
		},
	}
	service := embeddings.Service{Store: store}

	if err := service.EmbedVideoFrames(context.Background(), 9); err != nil {
		t.Fatalf("expected frame embeddings to succeed: %v", err)
	}
	if store.replaceFileID != 9 {
		t.Fatalf("expected replace to target file 9, got %d", store.replaceFileID)
	}
	if len(store.frameEmbeddings) != 2 {
		t.Fatalf("expected 2 frame embeddings, got %#v", store.frameEmbeddings)
	}
	if store.frameEmbeddings[0].EmbeddingType != "video_frame_visual" {
		t.Fatalf("unexpected frame embedding type: %#v", store.frameEmbeddings[0])
	}
	if store.frameEmbeddings[0].ModelName != "phash-v1" {
		t.Fatalf("unexpected frame model name: %#v", store.frameEmbeddings[0])
	}
}

func TestServiceReturnsStoreError(t *testing.T) {
	service := embeddings.Service{
		Store: &recordingStore{err: errors.New("db down")},
	}

	if err := service.EmbedImage(context.Background(), 7); err == nil {
		t.Fatal("expected store error")
	}
}

func TestServiceEmbedImageUsesWorkerWhenConfigured(t *testing.T) {
	store := &recordingStore{
		image: embeddings.ImageSource{
			FileID:   7,
			PHash:    "0123456789abcdef",
			FilePath: "/Volumes/media/photos/poster.jpg",
		},
	}
	embedder := &recordingEmbedder{
		result: embeddings.EmbedResult{
			Vector:   "[0.1,0.2]",
			Provider: "semantic",
			Model:    "semantic-ollama-qwen3-vl-8b-v1",
		},
	}
	service := embeddings.Service{Store: store, Embedder: embedder}

	if err := service.EmbedImage(context.Background(), 7); err != nil {
		t.Fatalf("expected image embedding to succeed: %v", err)
	}
	if embedder.request.FilePath != "/Volumes/media/photos/poster.jpg" {
		t.Fatalf("unexpected embed request: %#v", embedder.request)
	}
	if embedder.request.EmbeddingType != embeddings.ImageVisualType {
		t.Fatalf("expected image embedding type in request, got %#v", embedder.request)
	}
	if store.fileEmbedding.Vector != "[0.1,0.2]" {
		t.Fatalf("unexpected stored vector: %#v", store.fileEmbedding)
	}
	if store.fileEmbedding.ModelName != "semantic-ollama-qwen3-vl-8b-v1" {
		t.Fatalf("expected worker model name to be preserved, got %#v", store.fileEmbedding)
	}
}

func TestServiceEmbedVideoFramesUsesWorkerModelNameWhenConfigured(t *testing.T) {
	store := &recordingStore{
		frames: []embeddings.VideoFrameSource{
			{FrameID: 11, FileID: 9, FramePath: "/tmp/11.jpg", PHash: "aaaaaaaaaaaaaaaa"},
			{FrameID: 13, FileID: 9, FramePath: "/tmp/13.jpg", PHash: "bbbbbbbbbbbbbbbb"},
		},
	}
	embedder := &recordingEmbedder{
		result: embeddings.EmbedResult{
			Provider: "semantic",
			Model:    "semantic-ollama-qwen3-vl-8b-v1",
			FrameVectors: []embeddings.FrameVector{
				{FrameID: 11, Vector: "[0.1,0.2]"},
				{FrameID: 13, Vector: "[0.3,0.4]"},
			},
		},
	}
	service := embeddings.Service{Store: store, Embedder: embedder}

	if err := service.EmbedVideoFrames(context.Background(), 9); err != nil {
		t.Fatalf("expected frame embeddings to succeed: %v", err)
	}
	if len(store.frameEmbeddings) != 2 {
		t.Fatalf("expected frame embeddings to be written, got %#v", store.frameEmbeddings)
	}
	if embedder.request.EmbeddingType != embeddings.VideoFrameVisualType {
		t.Fatalf("expected video frame embedding type in request, got %#v", embedder.request)
	}
	if store.frameEmbeddings[0].ModelName != "semantic-ollama-qwen3-vl-8b-v1" {
		t.Fatalf("expected worker frame model name to be preserved, got %#v", store.frameEmbeddings)
	}
}

func TestServiceEmbedPersonImageUsesPersonVisualType(t *testing.T) {
	store := &recordingStore{
		image: embeddings.ImageSource{
			FileID:   17,
			PHash:    "0123456789abcdef",
			FilePath: "/Volumes/media/photos/person.jpg",
		},
	}
	embedder := &recordingEmbedder{
		result: embeddings.EmbedResult{
			Vector:   "[0.9,0.8]",
			Provider: "semantic",
			Model:    "semantic-person-v1",
		},
	}
	service := embeddings.Service{Store: store, Embedder: embedder}

	if err := service.EmbedPersonImage(context.Background(), 17); err != nil {
		t.Fatalf("expected person image embedding to succeed: %v", err)
	}
	if embedder.request.EmbeddingType != embeddings.PersonVisualType {
		t.Fatalf("expected person visual request, got %#v", embedder.request)
	}
	if store.fileEmbedding.EmbeddingType != embeddings.PersonVisualType {
		t.Fatalf("expected person visual embedding to be written, got %#v", store.fileEmbedding)
	}
}

func TestServiceEmbedPersonVideoFramesUsesPersonVisualType(t *testing.T) {
	store := &recordingStore{
		frames: []embeddings.VideoFrameSource{
			{FrameID: 21, FileID: 19, FramePath: "/tmp/21.jpg", PHash: "aaaaaaaaaaaaaaaa"},
			{FrameID: 22, FileID: 19, FramePath: "/tmp/22.jpg", PHash: "bbbbbbbbbbbbbbbb"},
		},
	}
	embedder := &recordingEmbedder{
		result: embeddings.EmbedResult{
			Provider: "semantic",
			Model:    "semantic-person-v1",
			FrameVectors: []embeddings.FrameVector{
				{FrameID: 21, Vector: "[0.1,0.2]"},
				{FrameID: 22, Vector: "[0.3,0.4]"},
			},
		},
	}
	service := embeddings.Service{Store: store, Embedder: embedder}

	if err := service.EmbedPersonVideoFrames(context.Background(), 19); err != nil {
		t.Fatalf("expected person video frame embeddings to succeed: %v", err)
	}
	if embedder.request.EmbeddingType != embeddings.PersonVisualType {
		t.Fatalf("expected person visual request, got %#v", embedder.request)
	}
	if store.replaceEmbeddingType != embeddings.PersonVisualType {
		t.Fatalf("expected replace to target person visual embeddings, got %q", store.replaceEmbeddingType)
	}
	if len(store.frameEmbeddings) != 2 || store.frameEmbeddings[0].EmbeddingType != embeddings.PersonVisualType {
		t.Fatalf("expected person visual frame embeddings, got %#v", store.frameEmbeddings)
	}
}

type recordingStore struct {
	image           embeddings.ImageSource
	frames          []embeddings.VideoFrameSource
	fileEmbedding   embeddings.FileEmbeddingInput
	frameEmbeddings []embeddings.FrameEmbeddingInput
	replaceFileID   int64
	replaceEmbeddingType string
	err             error
}

func (s *recordingStore) GetImageSource(_ context.Context, _ int64) (embeddings.ImageSource, error) {
	if s.err != nil {
		return embeddings.ImageSource{}, s.err
	}
	return s.image, nil
}

func (s *recordingStore) ListVideoFrameSources(_ context.Context, _ int64) ([]embeddings.VideoFrameSource, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]embeddings.VideoFrameSource(nil), s.frames...), nil
}

func (s *recordingStore) UpsertFileEmbedding(_ context.Context, input embeddings.FileEmbeddingInput) error {
	if s.err != nil {
		return s.err
	}
	s.fileEmbedding = input
	return nil
}

func (s *recordingStore) ReplaceFrameEmbeddings(_ context.Context, fileID int64, embeddingType string, inputs []embeddings.FrameEmbeddingInput) error {
	if s.err != nil {
		return s.err
	}
	s.replaceFileID = fileID
	s.replaceEmbeddingType = embeddingType
	s.frameEmbeddings = append([]embeddings.FrameEmbeddingInput(nil), inputs...)
	return nil
}

type recordingEmbedder struct {
	request embeddings.EmbedRequest
	result  embeddings.EmbedResult
	err     error
}

func (e *recordingEmbedder) EmbedMedia(_ context.Context, input embeddings.EmbedRequest) (embeddings.EmbedResult, error) {
	e.request = input
	return e.result, e.err
}
