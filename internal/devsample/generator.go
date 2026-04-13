package devsample

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

type Report struct {
	RootPath      string
	FileCount     int
	DuplicatePair [2]string
}

func Generate(root string) (Report, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Report{}, err
	}

	type sample struct {
		relPath string
		fill    color.RGBA
		accent  color.RGBA
	}

	samples := []sample{
		{
			relPath: "photos/set-a/model-a-001.png",
			fill:    color.RGBA{R: 230, G: 212, B: 187, A: 255},
			accent:  color.RGBA{R: 179, G: 71, B: 47, A: 255},
		},
		{
			relPath: "photos/set-a/model-a-002.png",
			fill:    color.RGBA{R: 235, G: 220, B: 198, A: 255},
			accent:  color.RGBA{R: 140, G: 86, B: 53, A: 255},
		},
		{
			relPath: "photos/set-b/model-b-001.png",
			fill:    color.RGBA{R: 198, G: 214, B: 225, A: 255},
			accent:  color.RGBA{R: 52, G: 91, B: 122, A: 255},
		},
		{
			relPath: "images/posters/poster-sample.png",
			fill:    color.RGBA{R: 245, G: 234, B: 208, A: 255},
			accent:  color.RGBA{R: 96, G: 43, B: 31, A: 255},
		},
	}

	for _, item := range samples {
		if err := writePNG(filepath.Join(root, item.relPath), item.fill, item.accent); err != nil {
			return Report{}, err
		}
	}

	original := filepath.Join(root, "photos/set-a/model-a-001.png")
	duplicate := filepath.Join(root, "photos/set-a/model-a-001-copy.png")
	if err := copyFile(original, duplicate); err != nil {
		return Report{}, err
	}

	return Report{
		RootPath:  root,
		FileCount: 5,
		DuplicatePair: [2]string{
			original,
			duplicate,
		},
	}, nil
}

func writePNG(path string, fill color.RGBA, accent color.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	canvas := image.NewRGBA(image.Rect(0, 0, 960, 640))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(80, 80, 880, 560), &image.Uniform{C: color.RGBA{R: 255, G: 250, B: 240, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(140, 120, 820, 520), &image.Uniform{C: accent}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(220, 180, 740, 460), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(300, 220, 660, 420), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)

	if err := png.Encode(file, canvas); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}

func copyFile(src string, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0o644)
}
