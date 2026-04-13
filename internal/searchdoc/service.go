package searchdoc

import (
	"context"
	"strconv"
	"strings"
)

type FileSource struct {
	FileID      int64
	AbsPath     string
	FileName    string
	Extension   string
	MediaType   string
	Status      string
	Width       *int
	Height      *int
	DurationMS  *int64
	Format      string
	Container   string
	VideoCodec  string
	AudioCodec  string
	Orientation string
}

type DocumentInput struct {
	FileID       int64
	DocumentText string
}

type SearchAnalysisInput struct {
	FileID  int64
	Summary string
}

type Store interface {
	GetFileSource(ctx context.Context, fileID int64) (FileSource, error)
	UpsertSearchDocument(ctx context.Context, input DocumentInput) error
	UpsertSearchAnalysis(ctx context.Context, input SearchAnalysisInput) error
}

type Service struct {
	Store Store
}

func (s Service) RecomputeSearchDocument(ctx context.Context, fileID int64) error {
	source, err := s.Store.GetFileSource(ctx, fileID)
	if err != nil {
		return err
	}
	document := buildDocument(source)
	if err := s.Store.UpsertSearchDocument(ctx, DocumentInput{
		FileID:       fileID,
		DocumentText: document,
	}); err != nil {
		return err
	}
	return s.Store.UpsertSearchAnalysis(ctx, SearchAnalysisInput{
		FileID:  fileID,
		Summary: document,
	})
}

func buildDocument(source FileSource) string {
	parts := []string{
		source.FileName,
		source.AbsPath,
		source.Extension,
		source.MediaType,
		source.Status,
		source.Format,
		source.Container,
		source.VideoCodec,
		source.AudioCodec,
		source.Orientation,
	}
	if source.Width != nil && source.Height != nil {
		parts = append(parts, strconv.Itoa(*source.Width)+"x"+strconv.Itoa(*source.Height))
	}
	if source.DurationMS != nil {
		parts = append(parts, strconv.FormatInt(*source.DurationMS/1000, 10)+"s")
	}

	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return strings.Join(result, " ")
}
