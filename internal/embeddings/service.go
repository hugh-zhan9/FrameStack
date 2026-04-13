package embeddings

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	ImageVisualType      = "image_visual"
	VideoFrameVisualType = "video_frame_visual"
	PersonVisualType     = "person_visual"
	ModelNamePHashV1     = "phash-v1"
)

type ImageSource struct {
	FileID   int64
	FilePath string
	PHash    string
}

type VideoFrameSource struct {
	FrameID   int64
	FileID    int64
	FramePath string
	PHash     string
}

type FileEmbeddingInput struct {
	FileID        int64
	EmbeddingType string
	ModelName     string
	Vector        string
}

type FrameEmbeddingInput struct {
	FrameID       int64
	EmbeddingType string
	ModelName     string
	Vector        string
}

type Store interface {
	GetImageSource(ctx context.Context, fileID int64) (ImageSource, error)
	ListVideoFrameSources(ctx context.Context, fileID int64) ([]VideoFrameSource, error)
	UpsertFileEmbedding(ctx context.Context, input FileEmbeddingInput) error
	ReplaceFrameEmbeddings(ctx context.Context, fileID int64, embeddingType string, inputs []FrameEmbeddingInput) error
}

type EmbedRequest struct {
	EmbeddingType string
	MediaType  string
	FilePath   string
	ImagePHash string
	Frames     []EmbedFrame
}

type EmbedFrame struct {
	FrameID   int64
	FramePath string
	PHash     string
}

type EmbedResult struct {
	Vector       string
	FrameVectors []FrameVector
	Provider     string
	Model        string
	RawResponse  map[string]any
}

type FrameVector struct {
	FrameID int64
	Vector  string
}

type Embedder interface {
	EmbedMedia(ctx context.Context, input EmbedRequest) (EmbedResult, error)
}

type Service struct {
	Store    Store
	Embedder Embedder
}

func (s Service) EmbedImage(ctx context.Context, fileID int64) error {
	return s.embedImageByType(ctx, fileID, ImageVisualType)
}

func (s Service) EmbedPersonImage(ctx context.Context, fileID int64) error {
	return s.embedImageByType(ctx, fileID, PersonVisualType)
}

func (s Service) embedImageByType(ctx context.Context, fileID int64, embeddingType string) error {
	source, err := s.Store.GetImageSource(ctx, fileID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(source.PHash) == "" {
		return nil
	}
	vector := phashVector(source.PHash)
	modelName := ModelNamePHashV1
	if s.Embedder != nil {
		result, err := s.Embedder.EmbedMedia(ctx, EmbedRequest{
			EmbeddingType: embeddingType,
			MediaType:  "image",
			FilePath:   source.FilePath,
			ImagePHash: source.PHash,
		})
		if err != nil {
			return err
		}
		if strings.TrimSpace(result.Vector) != "" {
			vector = result.Vector
		}
		if strings.TrimSpace(result.Model) != "" {
			modelName = strings.TrimSpace(result.Model)
		}
	}
	return s.Store.UpsertFileEmbedding(ctx, FileEmbeddingInput{
		FileID:        source.FileID,
		EmbeddingType: embeddingType,
		ModelName:     modelName,
		Vector:        vector,
	})
}

func (s Service) EmbedVideoFrames(ctx context.Context, fileID int64) error {
	return s.embedVideoFramesByType(ctx, fileID, VideoFrameVisualType)
}

func (s Service) EmbedPersonVideoFrames(ctx context.Context, fileID int64) error {
	return s.embedVideoFramesByType(ctx, fileID, PersonVisualType)
}

func (s Service) embedVideoFramesByType(ctx context.Context, fileID int64, embeddingType string) error {
	frames, err := s.Store.ListVideoFrameSources(ctx, fileID)
	if err != nil {
		return err
	}
	inputs := make([]FrameEmbeddingInput, 0, len(frames))
	request := EmbedRequest{EmbeddingType: embeddingType, MediaType: "video", Frames: make([]EmbedFrame, 0, len(frames))}
	modelName := ModelNamePHashV1
	for _, frame := range frames {
		if strings.TrimSpace(frame.PHash) == "" {
			continue
		}
		request.Frames = append(request.Frames, EmbedFrame{
			FrameID:   frame.FrameID,
			FramePath: frame.FramePath,
			PHash:     frame.PHash,
		})
		inputs = append(inputs, FrameEmbeddingInput{
			FrameID:       frame.FrameID,
			EmbeddingType: embeddingType,
			ModelName:     ModelNamePHashV1,
			Vector:        phashVector(frame.PHash),
		})
	}
	if s.Embedder != nil && len(request.Frames) > 0 {
		result, err := s.Embedder.EmbedMedia(ctx, request)
		if err != nil {
			return err
		}
		if len(result.FrameVectors) > 0 {
			vectorsByFrameID := make(map[int64]string, len(result.FrameVectors))
			for _, item := range result.FrameVectors {
				if strings.TrimSpace(item.Vector) != "" {
					vectorsByFrameID[item.FrameID] = item.Vector
				}
			}
			for index := range inputs {
				if vector, ok := vectorsByFrameID[inputs[index].FrameID]; ok {
					inputs[index].Vector = vector
				}
			}
		}
		if strings.TrimSpace(result.Model) != "" {
			modelName = strings.TrimSpace(result.Model)
		}
	}
	for index := range inputs {
		inputs[index].ModelName = modelName
	}
	return s.Store.ReplaceFrameEmbeddings(ctx, fileID, embeddingType, inputs)
}

func phashVector(phash string) string {
	values := make([]string, 0, len(phash))
	for _, r := range strings.TrimSpace(strings.ToLower(phash)) {
		value, err := strconv.ParseUint(string(r), 16, 8)
		if err != nil {
			continue
		}
		values = append(values, fmt.Sprintf("%.4f", float64(value)/15.0))
	}
	if len(values) == 0 {
		return "[]"
	}
	return "[" + strings.Join(values, ",") + "]"
}
