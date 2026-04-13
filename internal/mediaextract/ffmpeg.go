package mediaextract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type FFmpegVideoFrameExtractor struct {
	Command    string
	OutputRoot string
}

func (e FFmpegVideoFrameExtractor) ExtractPreview(ctx context.Context, path string, fileID int64, metadata VideoMetadata) (VideoPreview, error) {
	command := e.Command
	if command == "" {
		command = "ffmpeg"
	}
	if _, err := exec.LookPath(command); err != nil {
		return VideoPreview{}, ErrProbeUnavailable
	}

	outputRoot := e.OutputRoot
	if outputRoot == "" {
		outputRoot = filepath.Join("tmp", "previews")
	}
	outputDir := filepath.Join(outputRoot, fmt.Sprintf("%d", fileID))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return VideoPreview{}, err
	}

	offsets := previewOffsets(metadata.DurationMS)
	posterPath := filepath.Join(outputDir, "poster.jpg")
	if err := extractFrameAt(ctx, command, path, offsets[0], posterPath); err != nil {
		return VideoPreview{}, err
	}

	frames := make([]VideoFrameInput, 0, len(offsets))
	for index, offset := range offsets {
		framePath := filepath.Join(outputDir, fmt.Sprintf("frame-%d.jpg", index+1))
		if err := extractFrameAt(ctx, command, path, offset, framePath); err != nil {
			return VideoPreview{}, err
		}
		frames = append(frames, VideoFrameInput{
			TimestampMS: offset,
			FramePath:   framePath,
			FrameRole:   "understanding",
		})
	}

	return VideoPreview{
		PosterPath: posterPath,
		Frames:     frames,
	}, nil
}

func extractFrameAt(ctx context.Context, command string, inputPath string, offsetMS int64, outputPath string) error {
	cmd := exec.CommandContext(
		ctx,
		command,
		"-y",
		"-ss", formatTimestamp(offsetMS),
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg extract frame failed: %w: %s", err, string(output))
	}
	return nil
}

func previewOffsets(durationMS *int64) []int64 {
	if durationMS == nil || *durationMS <= 0 {
		return []int64{1_000, 5_000, 10_000}
	}
	total := *durationMS
	points := []int64{total / 10, total / 2, (total * 9) / 10}
	result := make([]int64, 0, len(points))
	for _, point := range points {
		if point < 0 {
			point = 0
		}
		result = append(result, point)
	}
	return result
}

func formatTimestamp(offsetMS int64) string {
	seconds := float64(offsetMS) / 1000.0
	return fmt.Sprintf("%.3f", seconds)
}
