package mediaextract

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var ErrProbeUnavailable = errors.New("video probe unavailable")

type File struct {
	ID        int64
	AbsPath   string
	Extension string
	MediaType string
}

type ImageAssetInput struct {
	FileID        int64
	Width         *int
	Height        *int
	Format        string
	Orientation   string
	PHash         string
	ThumbnailPath string
}

type VideoAssetInput struct {
	FileID     int64
	DurationMS *int64
	Width      *int
	Height     *int
	FPS        *float64
	Container  string
	VideoCodec string
	AudioCodec string
	Bitrate    *int64
	PosterPath string
}

type VideoMetadata struct {
	DurationMS *int64
	Width      *int
	Height     *int
	FPS        *float64
	Container  string
	VideoCodec string
	AudioCodec string
	Bitrate    *int64
}

type VideoFrameInput struct {
	TimestampMS int64
	FramePath   string
	FrameRole   string
	PHash       string
}

type VideoPreview struct {
	PosterPath string
	Frames     []VideoFrameInput
}

type Store interface {
	GetFile(ctx context.Context, fileID int64) (File, error)
	UpsertImageAsset(ctx context.Context, input ImageAssetInput) error
	UpsertVideoAsset(ctx context.Context, input VideoAssetInput) error
	ReplaceVideoFrames(ctx context.Context, fileID int64, frames []VideoFrameInput) error
}

type VideoProbe interface {
	ProbeVideo(ctx context.Context, path string) (VideoMetadata, error)
}

type VideoFrameExtractor interface {
	ExtractPreview(ctx context.Context, path string, fileID int64, metadata VideoMetadata) (VideoPreview, error)
}

type SearchDocEnqueuer interface {
	EnqueueSearchDocument(ctx context.Context, fileID int64) error
}

type UnderstandingEnqueuer interface {
	EnqueueUnderstanding(ctx context.Context, fileID int64) error
}

type QualityEnqueuer interface {
	EnqueueQuality(ctx context.Context, fileID int64) error
}

type SameContentEnqueuer interface {
	EnqueueSameContent(ctx context.Context, fileID int64) error
}

type SameSeriesEnqueuer interface {
	EnqueueSameSeries(ctx context.Context, fileID int64) error
}

type EmbeddingEnqueuer interface {
	EnqueueImageEmbedding(ctx context.Context, fileID int64) error
	EnqueueVideoFrameEmbeddings(ctx context.Context, fileID int64) error
}

type Service struct {
	Store                 Store
	VideoProbe            VideoProbe
	FrameExtractor        VideoFrameExtractor
	ThumbnailRoot         string
	SearchDocEnqueuer     SearchDocEnqueuer
	UnderstandingEnqueuer UnderstandingEnqueuer
	QualityEnqueuer       QualityEnqueuer
	SameContentEnqueuer   SameContentEnqueuer
	SameSeriesEnqueuer    SameSeriesEnqueuer
	EmbeddingEnqueuer     EmbeddingEnqueuer
}

func (s Service) ExtractImageFeatures(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	if file.MediaType != "image" {
		return fmt.Errorf("file %d is not an image", fileID)
	}

	input := ImageAssetInput{
		FileID:      file.ID,
		Format:      normalizeAssetFormat(file.Extension),
		Orientation: "",
	}

	width, height, format, err := decodeImageConfig(file.AbsPath)
	switch {
	case err == nil:
		input.Width = &width
		input.Height = &height
		if format != "" {
			input.Format = format
		}
		input.Orientation = classifyOrientation(width, height)
		input.PHash = computeImagePHash(file.AbsPath)
		input.ThumbnailPath = s.ensureImageThumbnail(file.AbsPath, file.ID)
	case canSkipImageDecode(file.Extension):
		// Leave width and height empty when the runtime cannot decode the format.
	default:
		return err
	}

	if err := s.Store.UpsertImageAsset(ctx, input); err != nil {
		return err
	}
	if s.SearchDocEnqueuer != nil {
		if err := s.SearchDocEnqueuer.EnqueueSearchDocument(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.UnderstandingEnqueuer != nil {
		if err := s.UnderstandingEnqueuer.EnqueueUnderstanding(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.QualityEnqueuer != nil {
		if err := s.QualityEnqueuer.EnqueueQuality(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.EmbeddingEnqueuer != nil {
		if err := s.EmbeddingEnqueuer.EnqueueImageEmbedding(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.SameContentEnqueuer != nil {
		if err := s.SameContentEnqueuer.EnqueueSameContent(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.SameSeriesEnqueuer != nil {
		return s.SameSeriesEnqueuer.EnqueueSameSeries(ctx, file.ID)
	}
	return nil
}

func (s Service) ExtractVideoFeatures(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	if file.MediaType != "video" {
		return fmt.Errorf("file %d is not a video", fileID)
	}

	input := VideoAssetInput{
		FileID:    file.ID,
		Container: normalizeAssetFormat(file.Extension),
	}

	if s.VideoProbe != nil {
		metadata, err := s.VideoProbe.ProbeVideo(ctx, file.AbsPath)
		if err != nil && !errors.Is(err, ErrProbeUnavailable) {
			return err
		}
		if err == nil {
			input.DurationMS = metadata.DurationMS
			input.Width = metadata.Width
			input.Height = metadata.Height
			input.FPS = metadata.FPS
			if metadata.Container != "" {
				input.Container = metadata.Container
			}
			input.VideoCodec = metadata.VideoCodec
			input.AudioCodec = metadata.AudioCodec
			input.Bitrate = metadata.Bitrate
		}
	}
	if s.FrameExtractor != nil {
		preview, err := s.FrameExtractor.ExtractPreview(ctx, file.AbsPath, file.ID, metadataFromAssetInput(input))
		if err != nil && !errors.Is(err, ErrProbeUnavailable) {
			return err
		}
		if err == nil {
			input.PosterPath = preview.PosterPath
			for index := range preview.Frames {
				if preview.Frames[index].PHash == "" {
					preview.Frames[index].PHash = computeImagePHash(preview.Frames[index].FramePath)
				}
			}
			if err := s.Store.ReplaceVideoFrames(ctx, file.ID, preview.Frames); err != nil {
				return err
			}
		}
	}

	if err := s.Store.UpsertVideoAsset(ctx, input); err != nil {
		return err
	}
	if s.SearchDocEnqueuer != nil {
		if err := s.SearchDocEnqueuer.EnqueueSearchDocument(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.UnderstandingEnqueuer != nil {
		if err := s.UnderstandingEnqueuer.EnqueueUnderstanding(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.QualityEnqueuer != nil {
		if err := s.QualityEnqueuer.EnqueueQuality(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.EmbeddingEnqueuer != nil {
		if err := s.EmbeddingEnqueuer.EnqueueVideoFrameEmbeddings(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.SameContentEnqueuer != nil {
		if err := s.SameContentEnqueuer.EnqueueSameContent(ctx, file.ID); err != nil {
			return err
		}
	}
	if s.SameSeriesEnqueuer != nil {
		return s.SameSeriesEnqueuer.EnqueueSameSeries(ctx, file.ID)
	}
	return nil
}

func metadataFromAssetInput(input VideoAssetInput) VideoMetadata {
	return VideoMetadata{
		DurationMS: input.DurationMS,
		Width:      input.Width,
		Height:     input.Height,
		FPS:        input.FPS,
		Container:  input.Container,
		VideoCodec: input.VideoCodec,
		AudioCodec: input.AudioCodec,
		Bitrate:    input.Bitrate,
	}
}

func decodeImageConfig(path string) (int, int, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, "", err
	}
	defer file.Close()

	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, "", err
	}
	return config.Width, config.Height, strings.ToLower(format), nil
}

func classifyOrientation(width, height int) string {
	switch {
	case width > height:
		return "landscape"
	case height > width:
		return "portrait"
	case width > 0 && height > 0:
		return "square"
	default:
		return ""
	}
}

func normalizeAssetFormat(extension string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(extension)), ".")
}

func computeImagePHash(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return ""
	}
	return differenceHash(img)
}

func differenceHash(img image.Image) string {
	bounds := img.Bounds()
	if bounds.Empty() {
		return ""
	}
	const width = 9
	const height = 8
	gray := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + x*(bounds.Dx()-1)/(width-1)
			srcY := bounds.Min.Y + y*(bounds.Dy()-1)/(height-1)
			gray[y*width+x] = color.GrayModel.Convert(img.At(srcX, srcY)).(color.Gray).Y
		}
	}
	var hash uint64
	var bit uint
	for y := 0; y < height; y++ {
		for x := 0; x < width-1; x++ {
			left := gray[y*width+x]
			right := gray[y*width+x+1]
			if left > right {
				hash |= 1 << bit
			}
			bit++
		}
	}
	return fmt.Sprintf("%016x", hash)
}

func canSkipImageDecode(extension string) bool {
	switch strings.ToLower(extension) {
	case ".heic":
		return true
	default:
		return false
	}
}

func (s Service) ensureImageThumbnail(sourcePath string, fileID int64) string {
	img, _, err := decodeImage(sourcePath)
	if err != nil {
		return ""
	}
	outputRoot := s.ThumbnailRoot
	if outputRoot == "" {
		outputRoot = filepath.Join("tmp", "thumbnails")
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return ""
	}
	targetPath := filepath.Join(outputRoot, fmt.Sprintf("%d.jpg", fileID))
	if err := writeThumbnailJPEG(targetPath, img, 360); err != nil {
		return ""
	}
	return targetPath
}

func decodeImage(path string) (image.Image, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	return image.Decode(file)
}

func writeThumbnailJPEG(path string, img image.Image, maxEdge int) error {
	bounds := img.Bounds()
	if bounds.Empty() {
		return fmt.Errorf("empty image bounds")
	}
	width := bounds.Dx()
	height := bounds.Dy()
	targetWidth := width
	targetHeight := height
	if width > maxEdge || height > maxEdge {
		if width >= height {
			targetWidth = maxEdge
			targetHeight = max(1, height*maxEdge/width)
		} else {
			targetHeight = maxEdge
			targetWidth = max(1, width*maxEdge/height)
		}
	}
	canvas := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(canvas, canvas.Bounds(), img, bounds, draw.Over, nil)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return jpeg.Encode(file, canvas, &jpeg.Options{Quality: 82})
}
