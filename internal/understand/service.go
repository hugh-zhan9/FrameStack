package understand

import (
	"context"
	"encoding/json"
)

type File struct {
	ID         int64
	AbsPath    string
	FileName   string
	MediaType  string
	FramePaths []string
}

type Request struct {
	FileID     int64
	MediaType  string
	FilePath   string
	FramePaths []string
}

type TagCandidate struct {
	Namespace  string
	Name       string
	Confidence float64
}

type Result struct {
	RawTags              []string
	CanonicalCandidates  []TagCandidate
	Summary              string
	StructuredAttributes map[string]any
	Confidence           float64
	Provider             string
	Model                string
	RawResponse          map[string]any
}

type AnalysisInput struct {
	FileID               int64
	AnalysisType         string
	Status               string
	Summary              string
	StructuredAttributes json.RawMessage
	RawModelOutput       json.RawMessage
	Provider             string
	ModelName            string
	PromptVersion        string
	AnalysisVersion      int
}

type Store interface {
	GetFile(ctx context.Context, fileID int64) (File, error)
	UpsertAnalysis(ctx context.Context, input AnalysisInput) error
	ReplaceAITags(ctx context.Context, fileID int64, tags []TagCandidate) error
}

type Analyzer interface {
	UnderstandMedia(ctx context.Context, req Request) (Result, error)
}

type SamePersonEnqueuer interface {
	EnqueueSamePerson(ctx context.Context, fileID int64) error
}

type PersonEmbeddingEnqueuer interface {
	EnqueuePersonImageEmbedding(ctx context.Context, fileID int64) error
	EnqueuePersonVideoFrameEmbeddings(ctx context.Context, fileID int64) error
}

type Service struct {
	Store              Store
	Analyzer           Analyzer
	SamePersonEnqueuer SamePersonEnqueuer
	PersonEmbeddingEnqueuer PersonEmbeddingEnqueuer
}

func (s Service) AnalyzeFile(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	result, err := s.Analyzer.UnderstandMedia(ctx, Request{
		FileID:     file.ID,
		MediaType:  file.MediaType,
		FilePath:   file.AbsPath,
		FramePaths: file.FramePaths,
	})
	if err != nil {
		return err
	}

	structuredAttributes, err := json.Marshal(result.StructuredAttributes)
	if err != nil {
		return err
	}
	rawOutput, err := json.Marshal(map[string]any{
		"raw_tags":              result.RawTags,
		"canonical_candidates":  result.CanonicalCandidates,
		"summary":               result.Summary,
		"structured_attributes": result.StructuredAttributes,
		"confidence":            result.Confidence,
		"provider":              result.Provider,
		"model":                 result.Model,
		"raw_response":          result.RawResponse,
	})
	if err != nil {
		return err
	}

	if err := s.Store.UpsertAnalysis(ctx, AnalysisInput{
		FileID:               file.ID,
		AnalysisType:         "understanding",
		Status:               "succeeded",
		Summary:              result.Summary,
		StructuredAttributes: structuredAttributes,
		RawModelOutput:       rawOutput,
		Provider:             result.Provider,
		ModelName:            result.Model,
		PromptVersion:        "understand-v1",
		AnalysisVersion:      1,
	}); err != nil {
		return err
	}
	if err := s.Store.ReplaceAITags(ctx, file.ID, dedupeTags(result.CanonicalCandidates)); err != nil {
		return err
	}
	if s.PersonEmbeddingEnqueuer != nil && shouldEnqueuePersonEmbedding(result) {
		switch file.MediaType {
		case "image":
			if err := s.PersonEmbeddingEnqueuer.EnqueuePersonImageEmbedding(ctx, file.ID); err != nil {
				return err
			}
		case "video":
			if err := s.PersonEmbeddingEnqueuer.EnqueuePersonVideoFrameEmbeddings(ctx, file.ID); err != nil {
				return err
			}
		}
	}
	if s.SamePersonEnqueuer != nil {
		return s.SamePersonEnqueuer.EnqueueSamePerson(ctx, file.ID)
	}
	return nil
}

func shouldEnqueuePersonEmbedding(result Result) bool {
	for _, item := range result.CanonicalCandidates {
		if item.Namespace == "person" {
			return true
		}
		if item.Namespace == "content" {
			name := item.Name
			if containsAnyToken(name, "单人", "多人", "情侣", "写真", "自拍", "portrait", "selfie") {
				return true
			}
		}
	}
	if len(result.StructuredAttributes) == 0 {
		return false
	}
	if truthyValue(result.StructuredAttributes["has_face"]) {
		return true
	}
	if containsAnyToken(stringValue(result.StructuredAttributes["subject_count"]), "single", "单人", "multiple", "多人", "couple", "情侣") {
		return true
	}
	if containsAnyToken(stringValue(result.StructuredAttributes["capture_type"]), "selfie", "自拍", "photo", "实拍") {
		return true
	}
	return false
}

func truthyValue(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	default:
		return containsAnyToken(stringValue(value), "true", "1", "yes")
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch item := value.(type) {
	case string:
		return item
	default:
		return ""
	}
}

func containsAnyToken(value string, tokens ...string) bool {
	for _, token := range tokens {
		if token != "" && value != "" && containsTokenFold(value, token) {
			return true
		}
	}
	return false
}

func containsTokenFold(value string, token string) bool {
	return len(value) >= len(token) && (value == token || containsFold(value, token))
}

func containsFold(value string, token string) bool {
	valueRunes := []rune(value)
	tokenRunes := []rune(token)
	for start := 0; start+len(tokenRunes) <= len(valueRunes); start++ {
		match := true
		for offset := range tokenRunes {
			left := valueRunes[start+offset]
			right := tokenRunes[offset]
			if left == right {
				continue
			}
			if left >= 'A' && left <= 'Z' {
				left = left + ('a' - 'A')
			}
			if right >= 'A' && right <= 'Z' {
				right = right + ('a' - 'A')
			}
			if left != right {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func dedupeTags(items []TagCandidate) []TagCandidate {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]TagCandidate, 0, len(items))
	for _, item := range items {
		if item.Namespace == "" || item.Name == "" {
			continue
		}
		key := item.Namespace + ":" + item.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
