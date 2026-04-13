package mediaextract_test

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"idea/internal/mediaextract"
)

func TestServiceExtractImageFeaturesStoresImageMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "poster.png")
	writePNG(t, path, 320, 180)

	store := &recordingStore{
		file: mediaextract.File{
			ID:        7,
			AbsPath:   path,
			Extension: ".png",
			MediaType: "image",
		},
	}
	enqueuer := &recordingSearchDocEnqueuer{}
	understandingEnqueuer := &recordingUnderstandingEnqueuer{}
	qualityEnqueuer := &recordingQualityEnqueuer{}
	sameContentEnqueuer := &recordingSameContentEnqueuer{}
	sameSeriesEnqueuer := &recordingSameSeriesEnqueuer{}
	embeddingEnqueuer := &recordingEmbeddingEnqueuer{}
	service := mediaextract.Service{
		Store:                 store,
		ThumbnailRoot:         filepath.Join(root, "thumbs"),
		SearchDocEnqueuer:     enqueuer,
		UnderstandingEnqueuer: understandingEnqueuer,
		QualityEnqueuer:       qualityEnqueuer,
		SameContentEnqueuer:   sameContentEnqueuer,
		SameSeriesEnqueuer:    sameSeriesEnqueuer,
		EmbeddingEnqueuer:     embeddingEnqueuer,
	}

	if err := service.ExtractImageFeatures(context.Background(), 7); err != nil {
		t.Fatalf("expected extraction to succeed: %v", err)
	}
	if store.imageAsset.FileID != 7 {
		t.Fatalf("unexpected file id: %#v", store.imageAsset)
	}
	if store.imageAsset.Width == nil || *store.imageAsset.Width != 320 {
		t.Fatalf("unexpected width: %#v", store.imageAsset.Width)
	}
	if store.imageAsset.Height == nil || *store.imageAsset.Height != 180 {
		t.Fatalf("unexpected height: %#v", store.imageAsset.Height)
	}
	if store.imageAsset.Format != "png" {
		t.Fatalf("unexpected format: %#v", store.imageAsset.Format)
	}
	if store.imageAsset.PHash == "" {
		t.Fatalf("expected image phash to be populated: %#v", store.imageAsset)
	}
	if store.imageAsset.ThumbnailPath == "" {
		t.Fatalf("expected image thumbnail path to be populated: %#v", store.imageAsset)
	}
	if _, err := os.Stat(store.imageAsset.ThumbnailPath); err != nil {
		t.Fatalf("expected thumbnail file to exist: %v", err)
	}
	if store.imageAsset.Orientation != "landscape" {
		t.Fatalf("unexpected orientation: %#v", store.imageAsset.Orientation)
	}
	if len(enqueuer.fileIDs) != 1 || enqueuer.fileIDs[0] != 7 {
		t.Fatalf("expected search doc enqueue for file 7, got %#v", enqueuer.fileIDs)
	}
	if len(understandingEnqueuer.fileIDs) != 1 || understandingEnqueuer.fileIDs[0] != 7 {
		t.Fatalf("expected understanding enqueue for file 7, got %#v", understandingEnqueuer.fileIDs)
	}
	if len(qualityEnqueuer.fileIDs) != 1 || qualityEnqueuer.fileIDs[0] != 7 {
		t.Fatalf("expected quality enqueue for file 7, got %#v", qualityEnqueuer.fileIDs)
	}
	if len(sameContentEnqueuer.fileIDs) != 1 || sameContentEnqueuer.fileIDs[0] != 7 {
		t.Fatalf("expected same content enqueue for file 7, got %#v", sameContentEnqueuer.fileIDs)
	}
	if len(sameSeriesEnqueuer.fileIDs) != 1 || sameSeriesEnqueuer.fileIDs[0] != 7 {
		t.Fatalf("expected same series enqueue for file 7, got %#v", sameSeriesEnqueuer.fileIDs)
	}
	if len(embeddingEnqueuer.imageFileIDs) != 1 || embeddingEnqueuer.imageFileIDs[0] != 7 {
		t.Fatalf("expected image embedding enqueue for file 7, got %#v", embeddingEnqueuer.imageFileIDs)
	}
}

func TestServiceExtractImageFeaturesAllowsUnsupportedImageFormats(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "poster.heic")
	if err := os.WriteFile(path, []byte("not-a-real-heic"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	store := &recordingStore{
		file: mediaextract.File{
			ID:        8,
			AbsPath:   path,
			Extension: ".heic",
			MediaType: "image",
		},
	}
	enqueuer := &recordingSearchDocEnqueuer{}
	understandingEnqueuer := &recordingUnderstandingEnqueuer{}
	qualityEnqueuer := &recordingQualityEnqueuer{}
	sameContentEnqueuer := &recordingSameContentEnqueuer{}
	sameSeriesEnqueuer := &recordingSameSeriesEnqueuer{}
	embeddingEnqueuer := &recordingEmbeddingEnqueuer{}
	service := mediaextract.Service{
		Store:                 store,
		SearchDocEnqueuer:     enqueuer,
		UnderstandingEnqueuer: understandingEnqueuer,
		QualityEnqueuer:       qualityEnqueuer,
		SameContentEnqueuer:   sameContentEnqueuer,
		SameSeriesEnqueuer:    sameSeriesEnqueuer,
		EmbeddingEnqueuer:     embeddingEnqueuer,
	}

	if err := service.ExtractImageFeatures(context.Background(), 8); err != nil {
		t.Fatalf("expected extraction to tolerate unsupported format: %v", err)
	}
	if store.imageAsset.Format != "heic" {
		t.Fatalf("unexpected format: %#v", store.imageAsset.Format)
	}
	if store.imageAsset.Width != nil || store.imageAsset.Height != nil {
		t.Fatalf("expected empty dimensions for unsupported image, got %#v", store.imageAsset)
	}
	if len(enqueuer.fileIDs) != 1 || enqueuer.fileIDs[0] != 8 {
		t.Fatalf("expected search doc enqueue for file 8, got %#v", enqueuer.fileIDs)
	}
	if len(understandingEnqueuer.fileIDs) != 1 || understandingEnqueuer.fileIDs[0] != 8 {
		t.Fatalf("expected understanding enqueue for file 8, got %#v", understandingEnqueuer.fileIDs)
	}
	if len(qualityEnqueuer.fileIDs) != 1 || qualityEnqueuer.fileIDs[0] != 8 {
		t.Fatalf("expected quality enqueue for file 8, got %#v", qualityEnqueuer.fileIDs)
	}
	if len(sameContentEnqueuer.fileIDs) != 1 || sameContentEnqueuer.fileIDs[0] != 8 {
		t.Fatalf("expected same content enqueue for file 8, got %#v", sameContentEnqueuer.fileIDs)
	}
	if len(sameSeriesEnqueuer.fileIDs) != 1 || sameSeriesEnqueuer.fileIDs[0] != 8 {
		t.Fatalf("expected same series enqueue for file 8, got %#v", sameSeriesEnqueuer.fileIDs)
	}
	if len(embeddingEnqueuer.imageFileIDs) != 1 || embeddingEnqueuer.imageFileIDs[0] != 8 {
		t.Fatalf("expected image embedding enqueue for file 8, got %#v", embeddingEnqueuer.imageFileIDs)
	}
}

func TestServiceExtractVideoFeaturesStoresProbedMetadata(t *testing.T) {
	store := &recordingStore{
		file: mediaextract.File{
			ID:        9,
			AbsPath:   "/Volumes/media/clip.mp4",
			Extension: ".mp4",
			MediaType: "video",
		},
	}
	service := mediaextract.Service{
		Store: store,
		VideoProbe: stubVideoProbe{
			metadata: mediaextract.VideoMetadata{
				DurationMS: int64Ptr(95_000),
				Width:      intPtr(1920),
				Height:     intPtr(1080),
				FPS:        float64Ptr(29.97),
				Container:  "mp4",
				VideoCodec: "h264",
				AudioCodec: "aac",
				Bitrate:    int64Ptr(2_000_000),
			},
		},
		FrameExtractor: stubVideoFrameExtractor{
			preview: mediaextract.VideoPreview{
				PosterPath: "/tmp/previews/9/poster.jpg",
				Frames: []mediaextract.VideoFrameInput{
					{TimestampMS: 5_000, FramePath: "/tmp/previews/9/frame-1.jpg", FrameRole: "understanding", PHash: "frame-1"},
					{TimestampMS: 45_000, FramePath: "/tmp/previews/9/frame-2.jpg", FrameRole: "understanding", PHash: "frame-2"},
				},
			},
		},
		SearchDocEnqueuer:     &recordingSearchDocEnqueuer{},
		UnderstandingEnqueuer: &recordingUnderstandingEnqueuer{},
		QualityEnqueuer:       &recordingQualityEnqueuer{},
		SameContentEnqueuer:   &recordingSameContentEnqueuer{},
		SameSeriesEnqueuer:    &recordingSameSeriesEnqueuer{},
		EmbeddingEnqueuer:     &recordingEmbeddingEnqueuer{},
	}

	if err := service.ExtractVideoFeatures(context.Background(), 9); err != nil {
		t.Fatalf("expected extraction to succeed: %v", err)
	}
	if store.videoAsset.FileID != 9 {
		t.Fatalf("unexpected asset: %#v", store.videoAsset)
	}
	if store.videoAsset.DurationMS == nil || *store.videoAsset.DurationMS != 95_000 {
		t.Fatalf("unexpected duration: %#v", store.videoAsset.DurationMS)
	}
	if store.videoAsset.Container != "mp4" || store.videoAsset.VideoCodec != "h264" {
		t.Fatalf("unexpected video asset: %#v", store.videoAsset)
	}
	if store.videoAsset.PosterPath != "/tmp/previews/9/poster.jpg" {
		t.Fatalf("unexpected poster path: %#v", store.videoAsset)
	}
	if len(store.videoFrames) != 2 {
		t.Fatalf("expected 2 frames, got %#v", store.videoFrames)
	}
	if store.videoFrames[0].PHash == "" || store.videoFrames[1].PHash == "" {
		t.Fatalf("expected frame phash values, got %#v", store.videoFrames)
	}
}

func TestServiceExtractVideoFeaturesFallsBackWhenProbeUnavailable(t *testing.T) {
	store := &recordingStore{
		file: mediaextract.File{
			ID:        10,
			AbsPath:   "/Volumes/media/clip.mkv",
			Extension: ".mkv",
			MediaType: "video",
		},
	}
	enqueuer := &recordingSearchDocEnqueuer{}
	understandingEnqueuer := &recordingUnderstandingEnqueuer{}
	qualityEnqueuer := &recordingQualityEnqueuer{}
	sameContentEnqueuer := &recordingSameContentEnqueuer{}
	sameSeriesEnqueuer := &recordingSameSeriesEnqueuer{}
	embeddingEnqueuer := &recordingEmbeddingEnqueuer{}
	service := mediaextract.Service{
		Store:                 store,
		VideoProbe:            stubVideoProbe{err: mediaextract.ErrProbeUnavailable},
		SearchDocEnqueuer:     enqueuer,
		UnderstandingEnqueuer: understandingEnqueuer,
		QualityEnqueuer:       qualityEnqueuer,
		SameContentEnqueuer:   sameContentEnqueuer,
		SameSeriesEnqueuer:    sameSeriesEnqueuer,
		EmbeddingEnqueuer:     embeddingEnqueuer,
	}

	if err := service.ExtractVideoFeatures(context.Background(), 10); err != nil {
		t.Fatalf("expected extraction to succeed with minimal metadata: %v", err)
	}
	if store.videoAsset.Container != "mkv" {
		t.Fatalf("unexpected fallback container: %#v", store.videoAsset.Container)
	}
	if store.videoAsset.DurationMS != nil {
		t.Fatalf("expected no probed metadata, got %#v", store.videoAsset)
	}
	if len(enqueuer.fileIDs) != 1 || enqueuer.fileIDs[0] != 10 {
		t.Fatalf("expected search doc enqueue for file 10, got %#v", enqueuer.fileIDs)
	}
	if len(understandingEnqueuer.fileIDs) != 1 || understandingEnqueuer.fileIDs[0] != 10 {
		t.Fatalf("expected understanding enqueue for file 10, got %#v", understandingEnqueuer.fileIDs)
	}
	if len(qualityEnqueuer.fileIDs) != 1 || qualityEnqueuer.fileIDs[0] != 10 {
		t.Fatalf("expected quality enqueue for file 10, got %#v", qualityEnqueuer.fileIDs)
	}
	if len(sameContentEnqueuer.fileIDs) != 1 || sameContentEnqueuer.fileIDs[0] != 10 {
		t.Fatalf("expected same content enqueue for file 10, got %#v", sameContentEnqueuer.fileIDs)
	}
	if len(sameSeriesEnqueuer.fileIDs) != 1 || sameSeriesEnqueuer.fileIDs[0] != 10 {
		t.Fatalf("expected same series enqueue for file 10, got %#v", sameSeriesEnqueuer.fileIDs)
	}
	if len(embeddingEnqueuer.videoFileIDs) != 1 || embeddingEnqueuer.videoFileIDs[0] != 10 {
		t.Fatalf("expected video embedding enqueue for file 10, got %#v", embeddingEnqueuer.videoFileIDs)
	}
}

type recordingStore struct {
	file        mediaextract.File
	imageAsset  mediaextract.ImageAssetInput
	videoAsset  mediaextract.VideoAssetInput
	videoFrames []mediaextract.VideoFrameInput
}

func (s *recordingStore) GetFile(_ context.Context, fileID int64) (mediaextract.File, error) {
	if s.file.ID == 0 {
		return mediaextract.File{}, errors.New("missing file")
	}
	return s.file, nil
}

func (s *recordingStore) UpsertImageAsset(_ context.Context, input mediaextract.ImageAssetInput) error {
	s.imageAsset = input
	return nil
}

func (s *recordingStore) UpsertVideoAsset(_ context.Context, input mediaextract.VideoAssetInput) error {
	s.videoAsset = input
	return nil
}

func (s *recordingStore) ReplaceVideoFrames(_ context.Context, fileID int64, frames []mediaextract.VideoFrameInput) error {
	s.videoFrames = append([]mediaextract.VideoFrameInput(nil), frames...)
	return nil
}

type stubVideoProbe struct {
	metadata mediaextract.VideoMetadata
	err      error
}

func (p stubVideoProbe) ProbeVideo(_ context.Context, _ string) (mediaextract.VideoMetadata, error) {
	if p.err != nil {
		return mediaextract.VideoMetadata{}, p.err
	}
	return p.metadata, nil
}

type stubVideoFrameExtractor struct {
	preview mediaextract.VideoPreview
	err     error
}

func (e stubVideoFrameExtractor) ExtractPreview(_ context.Context, _ string, _ int64, _ mediaextract.VideoMetadata) (mediaextract.VideoPreview, error) {
	if e.err != nil {
		return mediaextract.VideoPreview{}, e.err
	}
	return e.preview, nil
}

func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 80, B: 120, A: 255})
		}
	}

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode failed: %v", err)
	}
}

func intPtr(v int) *int             { return &v }
func int64Ptr(v int64) *int64       { return &v }
func float64Ptr(v float64) *float64 { return &v }

type recordingSearchDocEnqueuer struct {
	fileIDs []int64
}

func (e *recordingSearchDocEnqueuer) EnqueueSearchDocument(_ context.Context, fileID int64) error {
	e.fileIDs = append(e.fileIDs, fileID)
	return nil
}

type recordingUnderstandingEnqueuer struct {
	fileIDs []int64
}

func (e *recordingUnderstandingEnqueuer) EnqueueUnderstanding(_ context.Context, fileID int64) error {
	e.fileIDs = append(e.fileIDs, fileID)
	return nil
}

type recordingQualityEnqueuer struct {
	fileIDs []int64
}

func (e *recordingQualityEnqueuer) EnqueueQuality(_ context.Context, fileID int64) error {
	e.fileIDs = append(e.fileIDs, fileID)
	return nil
}

type recordingSameContentEnqueuer struct {
	fileIDs []int64
}

func (e *recordingSameContentEnqueuer) EnqueueSameContent(_ context.Context, fileID int64) error {
	e.fileIDs = append(e.fileIDs, fileID)
	return nil
}

type recordingSameSeriesEnqueuer struct {
	fileIDs []int64
}

func (e *recordingSameSeriesEnqueuer) EnqueueSameSeries(_ context.Context, fileID int64) error {
	e.fileIDs = append(e.fileIDs, fileID)
	return nil
}

type recordingEmbeddingEnqueuer struct {
	imageFileIDs []int64
	videoFileIDs []int64
}

func (e *recordingEmbeddingEnqueuer) EnqueueImageEmbedding(_ context.Context, fileID int64) error {
	e.imageFileIDs = append(e.imageFileIDs, fileID)
	return nil
}

func (e *recordingEmbeddingEnqueuer) EnqueueVideoFrameEmbeddings(_ context.Context, fileID int64) error {
	e.videoFileIDs = append(e.videoFileIDs, fileID)
	return nil
}
