package mediaextract

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

type FFprobeVideoProbe struct {
	Command string
}

func (p FFprobeVideoProbe) ProbeVideo(ctx context.Context, path string) (VideoMetadata, error) {
	command := p.Command
	if command == "" {
		command = "ffprobe"
	}
	if _, err := exec.LookPath(command); err != nil {
		return VideoMetadata{}, ErrProbeUnavailable
	}

	cmd := exec.CommandContext(
		ctx,
		command,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return VideoMetadata{}, err
	}

	var response ffprobeResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return VideoMetadata{}, err
	}

	metadata := VideoMetadata{
		DurationMS: parseDurationMS(response.Format.Duration),
		Container:  normalizeContainer(response.Format.FormatName),
		Bitrate:    parseInt64(response.Format.BitRate),
	}

	for _, stream := range response.Streams {
		switch stream.CodecType {
		case "video":
			if metadata.Width == nil && stream.Width > 0 {
				metadata.Width = &stream.Width
			}
			if metadata.Height == nil && stream.Height > 0 {
				metadata.Height = &stream.Height
			}
			if metadata.FPS == nil {
				metadata.FPS = parseFrameRate(stream.RFrameRate)
			}
			if metadata.VideoCodec == "" {
				metadata.VideoCodec = stream.CodecName
			}
		case "audio":
			if metadata.AudioCodec == "" {
				metadata.AudioCodec = stream.CodecName
			}
		}
	}

	return metadata, nil
}

type ffprobeResponse struct {
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeFormat struct {
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
	FormatName string `json:"format_name"`
}

type ffprobeStream struct {
	CodecType  string `json:"codec_type"`
	CodecName  string `json:"codec_name"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
}

func parseDurationMS(raw string) *int64 {
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	duration := int64(value * 1000)
	return &duration
}

func parseInt64(raw string) *int64 {
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

func parseFrameRate(raw string) *float64 {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil
		}
		return &value
	}
	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil
	}
	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || denominator == 0 {
		return nil
	}
	value := numerator / denominator
	return &value
}

func normalizeContainer(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return strings.ToLower(raw)
	}
	return strings.ToLower(strings.TrimSpace(parts[0]))
}

var _ VideoProbe = FFprobeVideoProbe{}

var _ = errors.Is
