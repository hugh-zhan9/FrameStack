package searchdoc_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"idea/internal/searchdoc"
)

func TestServiceRecomputeSearchDocumentBuildsImageDocument(t *testing.T) {
	store := &recordingStore{
		source: searchdoc.FileSource{
			FileID:      7,
			AbsPath:     "/Volumes/media/photos/poster.jpg",
			FileName:    "poster.jpg",
			Extension:   ".jpg",
			MediaType:   "image",
			Status:      "active",
			Width:       intPtr(320),
			Height:      intPtr(180),
			Format:      "jpg",
			Orientation: "landscape",
		},
	}
	service := searchdoc.Service{Store: store}

	if err := service.RecomputeSearchDocument(context.Background(), 7); err != nil {
		t.Fatalf("expected recompute to succeed: %v", err)
	}
	if store.document.FileID != 7 {
		t.Fatalf("unexpected document: %#v", store.document)
	}
	if store.analysis.FileID != 7 {
		t.Fatalf("expected search analysis to be written, got %#v", store.analysis)
	}
	expectedParts := []string{"poster.jpg", "/Volumes/media/photos/poster.jpg", "image", "active", "jpg", "320x180", "landscape"}
	for _, part := range expectedParts {
		if !strings.Contains(store.document.DocumentText, part) {
			t.Fatalf("expected document to contain %q, got %q", part, store.document.DocumentText)
		}
	}
	if !strings.Contains(store.analysis.Summary, "poster.jpg") {
		t.Fatalf("expected analysis summary to reuse search document, got %#v", store.analysis)
	}
}

func TestServiceRecomputeSearchDocumentBuildsVideoDocument(t *testing.T) {
	store := &recordingStore{
		source: searchdoc.FileSource{
			FileID:     9,
			AbsPath:    "/Volumes/media/videos/clip.mp4",
			FileName:   "clip.mp4",
			Extension:  ".mp4",
			MediaType:  "video",
			Status:     "active",
			Width:      intPtr(1920),
			Height:     intPtr(1080),
			DurationMS: int64Ptr(95_000),
			Container:  "mp4",
			VideoCodec: "h264",
			AudioCodec: "aac",
		},
	}
	service := searchdoc.Service{Store: store}

	if err := service.RecomputeSearchDocument(context.Background(), 9); err != nil {
		t.Fatalf("expected recompute to succeed: %v", err)
	}
	expectedParts := []string{"clip.mp4", "video", "1920x1080", "95s", "mp4", "h264", "aac"}
	for _, part := range expectedParts {
		if !strings.Contains(store.document.DocumentText, part) {
			t.Fatalf("expected document to contain %q, got %q", part, store.document.DocumentText)
		}
	}
}

func TestServiceRecomputeSearchDocumentReturnsStoreError(t *testing.T) {
	service := searchdoc.Service{
		Store: &recordingStore{getErr: errors.New("db down")},
	}

	if err := service.RecomputeSearchDocument(context.Background(), 7); err == nil {
		t.Fatal("expected recompute to fail")
	}
}

type recordingStore struct {
	source   searchdoc.FileSource
	getErr   error
	document searchdoc.DocumentInput
	analysis searchdoc.SearchAnalysisInput
}

func (s *recordingStore) GetFileSource(_ context.Context, _ int64) (searchdoc.FileSource, error) {
	if s.getErr != nil {
		return searchdoc.FileSource{}, s.getErr
	}
	return s.source, nil
}

func (s *recordingStore) UpsertSearchDocument(_ context.Context, input searchdoc.DocumentInput) error {
	s.document = input
	return nil
}

func (s *recordingStore) UpsertSearchAnalysis(_ context.Context, input searchdoc.SearchAnalysisInput) error {
	s.analysis = input
	return nil
}

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }
