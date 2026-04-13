package jobexecutor

import (
	"context"
	"fmt"

	"idea/internal/scanner"
	"idea/internal/tasks"
)

type VolumeScanner interface {
	ScanVolume(ctx context.Context, volumeID int64) (scanner.Stats, error)
}

type FallbackExecutor interface {
	ExecuteJob(ctx context.Context, job tasks.Job) error
}

type MediaExtractor interface {
	ExtractImageFeatures(ctx context.Context, fileID int64) error
	ExtractVideoFeatures(ctx context.Context, fileID int64) error
}

type SearchDocumentIndexer interface {
	RecomputeSearchDocument(ctx context.Context, fileID int64) error
}

type MediaUnderstander interface {
	AnalyzeFile(ctx context.Context, fileID int64) error
}

type QualityEvaluator interface {
	EvaluateFile(ctx context.Context, fileID int64) error
}

type FileHasher interface {
	HashFile(ctx context.Context, fileID int64) error
}

type SameContentClusterer interface {
	ClusterFile(ctx context.Context, fileID int64) error
}

type SameSeriesClusterer interface {
	ClusterFile(ctx context.Context, fileID int64) error
}

type SamePersonClusterer interface {
	ClusterFile(ctx context.Context, fileID int64) error
}

type EmbeddingGenerator interface {
	EmbedImage(ctx context.Context, fileID int64) error
	EmbedVideoFrames(ctx context.Context, fileID int64) error
	EmbedPersonImage(ctx context.Context, fileID int64) error
	EmbedPersonVideoFrames(ctx context.Context, fileID int64) error
}

type Executor struct {
	Scanner          VolumeScanner
	Extractor        MediaExtractor
	SearchIndexer    SearchDocumentIndexer
	Understander     MediaUnderstander
	QualityEvaluator QualityEvaluator
	FileHasher       FileHasher
	SameContent      SameContentClusterer
	SameSeries       SameSeriesClusterer
	SamePerson       SamePersonClusterer
	Embeddings       EmbeddingGenerator
	Fallback         FallbackExecutor
}

func (e Executor) ExecuteJob(ctx context.Context, job tasks.Job) error {
	switch job.JobType {
	case "scan_volume":
		if e.Scanner == nil {
			return fmt.Errorf("scan_volume handler is not configured")
		}
		_, err := e.Scanner.ScanVolume(ctx, job.TargetID)
		return err
	case "extract_image_features":
		if e.Extractor == nil {
			return fmt.Errorf("extract_image_features handler is not configured")
		}
		return e.Extractor.ExtractImageFeatures(ctx, job.TargetID)
	case "extract_video_features":
		if e.Extractor == nil {
			return fmt.Errorf("extract_video_features handler is not configured")
		}
		return e.Extractor.ExtractVideoFeatures(ctx, job.TargetID)
	case "recompute_search_doc":
		if e.SearchIndexer == nil {
			return fmt.Errorf("recompute_search_doc handler is not configured")
		}
		return e.SearchIndexer.RecomputeSearchDocument(ctx, job.TargetID)
	case "infer_tags":
		if e.Understander == nil {
			return fmt.Errorf("infer_tags handler is not configured")
		}
		return e.Understander.AnalyzeFile(ctx, job.TargetID)
	case "infer_quality":
		if e.QualityEvaluator == nil {
			return fmt.Errorf("infer_quality handler is not configured")
		}
		return e.QualityEvaluator.EvaluateFile(ctx, job.TargetID)
	case "hash_file":
		if e.FileHasher == nil {
			return fmt.Errorf("hash_file handler is not configured")
		}
		return e.FileHasher.HashFile(ctx, job.TargetID)
	case "cluster_same_content":
		if e.SameContent == nil {
			return fmt.Errorf("cluster_same_content handler is not configured")
		}
		return e.SameContent.ClusterFile(ctx, job.TargetID)
	case "cluster_same_series":
		if e.SameSeries == nil {
			return fmt.Errorf("cluster_same_series handler is not configured")
		}
		return e.SameSeries.ClusterFile(ctx, job.TargetID)
	case "cluster_same_person":
		if e.SamePerson == nil {
			return fmt.Errorf("cluster_same_person handler is not configured")
		}
		return e.SamePerson.ClusterFile(ctx, job.TargetID)
	case "embed_image":
		if e.Embeddings == nil {
			return fmt.Errorf("embed_image handler is not configured")
		}
		return e.Embeddings.EmbedImage(ctx, job.TargetID)
	case "embed_video_frames":
		if e.Embeddings == nil {
			return fmt.Errorf("embed_video_frames handler is not configured")
		}
		return e.Embeddings.EmbedVideoFrames(ctx, job.TargetID)
	case "embed_person_image":
		if e.Embeddings == nil {
			return fmt.Errorf("embed_person_image handler is not configured")
		}
		return e.Embeddings.EmbedPersonImage(ctx, job.TargetID)
	case "embed_person_video_frames":
		if e.Embeddings == nil {
			return fmt.Errorf("embed_person_video_frames handler is not configured")
		}
		return e.Embeddings.EmbedPersonVideoFrames(ctx, job.TargetID)
	default:
		if e.Fallback == nil {
			return fmt.Errorf("no handler registered for job type %s", job.JobType)
		}
		return e.Fallback.ExecuteJob(ctx, job)
	}
}
