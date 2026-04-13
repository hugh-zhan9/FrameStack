package quality

import (
	"context"
	"fmt"
	"strings"
)

type FileSource struct {
	FileID     int64
	MediaType  string
	Width      *int
	Height     *int
	DurationMS *int64
	Bitrate    *int64
	FPS        *float64
	Format     string
	Container  string
	VideoCodec string
	AudioCodec string
}

type AnalysisInput struct {
	FileID       int64
	AnalysisType string
	Status       string
	Summary      string
	QualityScore float64
	QualityTier  string
}

type Store interface {
	GetFileSource(ctx context.Context, fileID int64) (FileSource, error)
	UpsertQualityAnalysis(ctx context.Context, input AnalysisInput) error
}

type Service struct {
	Store Store
}

func (s Service) EvaluateFile(ctx context.Context, fileID int64) error {
	source, err := s.Store.GetFileSource(ctx, fileID)
	if err != nil {
		return err
	}
	score, tier, summary := evaluate(source)
	return s.Store.UpsertQualityAnalysis(ctx, AnalysisInput{
		FileID:       source.FileID,
		AnalysisType: "quality",
		Status:       "succeeded",
		Summary:      summary,
		QualityScore: score,
		QualityTier:  tier,
	})
}

func evaluate(source FileSource) (float64, string, string) {
	score := 20.0
	if source.Width != nil && source.Height != nil {
		pixels := (*source.Width) * (*source.Height)
		switch {
		case pixels >= 3840*2160:
			score += 60
		case pixels >= 1920*1080:
			score += 45
		case pixels >= 1280*720:
			score += 32
		case pixels >= 854*480:
			score += 18
		default:
			score += 8
		}
	}
	if source.MediaType == "video" && source.Bitrate != nil {
		switch {
		case *source.Bitrate >= 8_000_000:
			score += 20
		case *source.Bitrate >= 4_000_000:
			score += 12
		case *source.Bitrate >= 1_500_000:
			score += 6
		default:
			score += 2
		}
	}
	if source.MediaType == "video" && source.FPS != nil {
		switch {
		case *source.FPS >= 50:
			score += 8
		case *source.FPS >= 29.97:
			score += 5
		case *source.FPS >= 23.976:
			score += 3
		default:
			score += 1
		}
	}
	if source.MediaType == "video" && source.DurationMS != nil {
		switch {
		case *source.DurationMS >= 10*60*1000:
			score += 6
		case *source.DurationMS >= 2*60*1000:
			score += 4
		case *source.DurationMS >= 30*1000:
			score += 2
		}
	}
	if source.MediaType == "video" {
		switch source.VideoCodec {
		case "hevc", "h265", "av1":
			score += 4
		case "h264":
			score += 2
		}
		switch source.Container {
		case "mkv":
			score += 2
		case "mp4", "mov":
			score += 1
		}
	}
	if score > 100 {
		score = 100
	}

	tier := "low"
	switch {
	case score >= 75:
		tier = "high"
	case score >= 45:
		tier = "medium"
	}

	label := source.Format
	if source.MediaType == "video" && source.Container != "" {
		label = source.Container
	}
	if source.MediaType == "video" && source.VideoCodec != "" {
		label = strings.TrimSpace(source.Container + " " + source.VideoCodec)
		label = strings.TrimSpace(label)
	}
	if label == "" {
		label = source.MediaType
	}
	resolution := "unknown"
	if source.Width != nil && source.Height != nil {
		resolution = fmt.Sprintf("%dx%d", *source.Width, *source.Height)
	}
	extras := []string{resolution, label}
	if source.MediaType == "video" {
		if source.Bitrate != nil {
			extras = append(extras, formatBitrate(*source.Bitrate))
		}
		if source.FPS != nil {
			extras = append(extras, fmt.Sprintf("%.2f fps", *source.FPS))
		}
	}
	return score, tier, fmt.Sprintf("%s quality %s, %s.", source.MediaType, tier, strings.Join(extras, " • "))
}

func formatBitrate(value int64) string {
	switch {
	case value >= 1_000_000:
		return fmt.Sprintf("%.1f Mbps", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%d Kbps", value/1_000)
	default:
		return fmt.Sprintf("%d bps", value)
	}
}
