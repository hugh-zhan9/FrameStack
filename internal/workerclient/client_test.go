package workerclient_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"idea/internal/workerclient"
)

func TestSessionHealthCheck(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Start(ctx)
	if err != nil {
		t.Fatalf("expected session to start: %v", err)
	}
	defer session.Close()

	resp, err := session.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("expected health check to succeed: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if len(resp.Providers) == 0 {
		t.Fatal("expected at least one provider in health response")
	}
}

func TestSessionListModels(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Start(ctx)
	if err != nil {
		t.Fatalf("expected session to start: %v", err)
	}
	defer session.Close()

	providers, err := session.ListModels(ctx)
	if err != nil {
		t.Fatalf("expected list models to succeed: %v", err)
	}
	if len(providers) == 0 {
		t.Fatal("expected providers to be returned")
	}
}

func TestSessionUnderstandMedia(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Start(ctx)
	if err != nil {
		t.Fatalf("expected session to start: %v", err)
	}
	defer session.Close()

	result, err := session.UnderstandMedia(ctx, workerclient.UnderstandMediaRequest{
		FileID:    9,
		MediaType: "image",
		FilePath:  "/Volumes/media/photos/poster.jpg",
		Context: workerclient.UnderstandMediaContext{
			AllowSensitiveLabels: true,
			MaxTags:              8,
			Language:             "zh-CN",
		},
	})
	if err != nil {
		t.Fatalf("expected understand media to succeed: %v", err)
	}
	if result.Provider == "" || len(result.CanonicalCandidates) == 0 || result.Summary == "" {
		t.Fatalf("unexpected understanding result: %#v", result)
	}
}

func TestSessionEmbedMedia(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Start(ctx)
	if err != nil {
		t.Fatalf("expected session to start: %v", err)
	}
	defer session.Close()

	result, err := session.EmbedMedia(ctx, workerclient.EmbedMediaRequest{
		MediaType: "video",
		FilePath:  "/Volumes/media/videos/clip.mp4",
		Frames: []workerclient.EmbedFrameInput{
			{FrameID: 11, FramePath: "/tmp/previews/11.jpg", PHash: "0123456789abcdef"},
			{FrameID: 12, FramePath: "/tmp/previews/12.jpg", PHash: "fedcba9876543210"},
		},
	})
	if err != nil {
		t.Fatalf("expected embed media to succeed: %v", err)
	}
	if result.Provider == "" || len(result.FrameVectors) != 2 || result.FrameVectors[0].Vector == "" {
		t.Fatalf("unexpected embed result: %#v", result)
	}
}

func newTestClient(t *testing.T) workerclient.Client {
	t.Helper()
	script := filepath.Clean(filepath.Join("..", "..", "worker", "main.py"))
	return workerclient.Client{
		Command: "python3",
		Script:  script,
	}
}
