package understand

import (
	"context"

	"idea/internal/workerclient"
)

type WorkerAnalyzer struct {
	Client workerclient.Client
}

func (a WorkerAnalyzer) UnderstandMedia(ctx context.Context, req Request) (Result, error) {
	session, err := a.Client.Start(ctx)
	if err != nil {
		return Result{}, err
	}
	defer session.Close()

	resp, err := session.UnderstandMedia(ctx, workerclient.UnderstandMediaRequest{
		FileID:     req.FileID,
		MediaType:  req.MediaType,
		FilePath:   req.FilePath,
		FramePaths: req.FramePaths,
		Context: workerclient.UnderstandMediaContext{
			AllowSensitiveLabels: true,
			MaxTags:              20,
			Language:             "zh-CN",
		},
	})
	if err != nil {
		return Result{}, err
	}

	result := Result{
		RawTags:              resp.RawTags,
		Summary:              resp.Summary,
		StructuredAttributes: resp.StructuredAttributes,
		Confidence:           resp.Confidence,
		Provider:             resp.Provider,
		Model:                resp.Model,
		RawResponse:          resp.RawResponse,
	}
	for _, item := range resp.CanonicalCandidates {
		result.CanonicalCandidates = append(result.CanonicalCandidates, TagCandidate{
			Namespace:  item.Namespace,
			Name:       item.Name,
			Confidence: item.Confidence,
		})
	}
	return result, nil
}
