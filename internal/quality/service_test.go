package quality_test

import (
	"context"
	"testing"

	"idea/internal/quality"
)

func TestServiceEvaluateImageQuality(t *testing.T) {
	store := &recordingStore{
		source: quality.FileSource{
			FileID:    7,
			MediaType: "image",
			Width:     intPtr(1920),
			Height:    intPtr(1080),
			Format:    "jpg",
		},
	}
	service := quality.Service{Store: store}

	if err := service.EvaluateFile(context.Background(), 7); err != nil {
		t.Fatalf("expected quality evaluation to succeed: %v", err)
	}
	if store.input.FileID != 7 || store.input.AnalysisType != "quality" {
		t.Fatalf("unexpected analysis input: %#v", store.input)
	}
	if store.input.QualityScore <= 0 || store.input.QualityTier == "" {
		t.Fatalf("expected score and tier, got %#v", store.input)
	}
}

func TestServiceEvaluateVideoQuality(t *testing.T) {
	store := &recordingStore{
		source: quality.FileSource{
			FileID:     8,
			MediaType:  "video",
			Width:      intPtr(3840),
			Height:     intPtr(2160),
			DurationMS: int64Ptr(90_000),
			Bitrate:    int64Ptr(8_000_000),
			FPS:        float64Ptr(29.97),
			Container:  "mp4",
			VideoCodec: "h264",
		},
	}
	service := quality.Service{Store: store}

	if err := service.EvaluateFile(context.Background(), 8); err != nil {
		t.Fatalf("expected video quality evaluation to succeed: %v", err)
	}
	if store.input.QualityTier != "high" {
		t.Fatalf("expected high tier, got %#v", store.input.QualityTier)
	}
	if store.input.Summary == "" {
		t.Fatalf("expected quality summary, got %#v", store.input)
	}
}

func TestServiceEvaluateLowBitrateLowFPSVideoAsLowerQuality(t *testing.T) {
	store := &recordingStore{
		source: quality.FileSource{
			FileID:     9,
			MediaType:  "video",
			Width:      intPtr(1280),
			Height:     intPtr(720),
			DurationMS: int64Ptr(30_000),
			Bitrate:    int64Ptr(700_000),
			FPS:        float64Ptr(15),
			Container:  "mp4",
			VideoCodec: "h264",
		},
	}
	service := quality.Service{Store: store}

	if err := service.EvaluateFile(context.Background(), 9); err != nil {
		t.Fatalf("expected low quality evaluation to succeed: %v", err)
	}
	if store.input.QualityTier == "high" {
		t.Fatalf("expected lower tier for weak video source, got %#v", store.input)
	}
	if store.input.QualityScore >= 75 {
		t.Fatalf("expected lower score for weak video source, got %#v", store.input)
	}
}

type recordingStore struct {
	source quality.FileSource
	input  quality.AnalysisInput
}

func (s *recordingStore) GetFileSource(_ context.Context, _ int64) (quality.FileSource, error) {
	return s.source, nil
}

func (s *recordingStore) UpsertQualityAnalysis(_ context.Context, input quality.AnalysisInput) error {
	s.input = input
	return nil
}

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }
func float64Ptr(v float64) *float64 { return &v }
