package embeddings

import (
	"context"

	"idea/internal/workerclient"
)

type WorkerEmbedder struct {
	Client workerclient.Client
}

func (e WorkerEmbedder) EmbedMedia(ctx context.Context, input EmbedRequest) (EmbedResult, error) {
	session, err := e.Client.Start(ctx)
	if err != nil {
		return EmbedResult{}, err
	}
	defer session.Close()

	request := workerclient.EmbedMediaRequest{
		EmbeddingType: input.EmbeddingType,
		MediaType:  input.MediaType,
		FilePath:   input.FilePath,
		ImagePHash: input.ImagePHash,
		Frames:     make([]workerclient.EmbedFrameInput, 0, len(input.Frames)),
	}
	for _, frame := range input.Frames {
		request.Frames = append(request.Frames, workerclient.EmbedFrameInput{
			FrameID:   frame.FrameID,
			FramePath: frame.FramePath,
			PHash:     frame.PHash,
		})
	}

	response, err := session.EmbedMedia(ctx, request)
	if err != nil {
		return EmbedResult{}, err
	}
	result := EmbedResult{
		Vector:      response.Vector,
		Provider:    response.Provider,
		Model:       response.Model,
		RawResponse: response.RawResponse,
	}
	for _, item := range response.FrameVectors {
		result.FrameVectors = append(result.FrameVectors, FrameVector{
			FrameID: item.FrameID,
			Vector:  item.Vector,
		})
	}
	return result, nil
}
