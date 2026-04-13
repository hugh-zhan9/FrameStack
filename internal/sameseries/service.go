package sameseries

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const candidateWindow = 30 * time.Minute
const crossDirectoryFamilyWindow = 72 * time.Hour
const imageSeriesPHashHammingThreshold = 8
const imageSeriesEmbeddingDistanceThreshold = 0.08
const imageSeriesAspectRatioThreshold = 0.20
const imageSeriesResolutionRatioThreshold = 0.20
const videoSeriesEmbeddingDistanceThreshold = 0.10
const videoSeriesDurationRatioThreshold = 0.85
const videoSeriesAspectRatioThreshold = 0.20
const videoSeriesResolutionRatioThreshold = 0.20

var trailingIndexPattern = regexp.MustCompile(`-\d+$`)

type FileContext struct {
	FileID                   int64
	ParentPath               string
	FileName                 string
	MediaType                string
	ModTime                  time.Time
	Status                   string
	DurationMS               int64
	Width                    int64
	Height                   int64
	CaptureType              string
	ImagePHash               string
	ImageEmbedding           string
	ImageEmbeddingType       string
	ImageEmbeddingModel      string
	VideoFramePHashes        []string
	VideoFrameEmbeddings     []string
	VideoFrameEmbeddingType  string
	VideoFrameEmbeddingModel string
}

type SeriesCandidateFile struct {
	FileID                   int64
	Role                     string
	ParentPath               string
	FileName                 string
	ModTime                  time.Time
	DurationMS               int64
	Width                    int64
	Height                   int64
	CaptureType              string
	ImagePHash               string
	ImageEmbedding           string
	ImageEmbeddingType       string
	ImageEmbeddingModel      string
	VideoFramePHashes        []string
	VideoFrameEmbeddings     []string
	VideoFrameEmbeddingType  string
	VideoFrameEmbeddingModel string
}

type Store interface {
	GetFileContext(ctx context.Context, fileID int64) (FileContext, error)
	ListSeriesCandidateFiles(ctx context.Context, file FileContext, window time.Duration) ([]SeriesCandidateFile, error)
	ListNearbySeriesCandidateFiles(ctx context.Context, file FileContext, window time.Duration, limit int) ([]SeriesCandidateFile, error)
	UpsertSameSeriesCluster(ctx context.Context, key string, files []SeriesCandidateFile) error
	DeactivateSameSeriesCluster(ctx context.Context, key string) error
}

type Service struct {
	Store Store
}

func (s Service) ClusterFile(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFileContext(ctx, fileID)
	if err != nil {
		return err
	}
	if file.Status == "missing" || file.Status == "trashed" {
		return nil
	}
	if file.ParentPath == "" || file.MediaType == "" || file.ModTime.IsZero() {
		return nil
	}

	candidates, err := s.Store.ListSeriesCandidateFiles(ctx, file, candidateWindow)
	if err != nil {
		return err
	}
	if family := normalizedFileFamily(file.FileName); family != "" {
		nearby, err := s.Store.ListNearbySeriesCandidateFiles(ctx, file, candidateWindow, 64)
		if err != nil {
			return err
		}
		candidates = mergeSeriesCandidates(candidates, nearby)
	}
	filtered := filterCandidates(file, candidates)
	filtered = assignSeriesRoles(filtered)
	key := clusterKey(file.ParentPath, file.FileName, filtered)
	if len(filtered) < 2 {
		return s.Store.DeactivateSameSeriesCluster(ctx, key)
	}
	return s.Store.UpsertSameSeriesCluster(ctx, key, filtered)
}

func clusterKey(parentPath string, fileName string, files []SeriesCandidateFile) string {
	if len(files) == 0 {
		sum := sha1.Sum([]byte(parentPath))
		pathKey := hex.EncodeToString(sum[:])[:12]
		return fmt.Sprintf("same_series:%s", pathKey)
	}
	earliest := files[0].ModTime.UTC()
	for _, file := range files[1:] {
		if file.ModTime.Before(earliest) {
			earliest = file.ModTime.UTC()
		}
	}
	sum := sha1.Sum([]byte(parentPath))
	pathKey := hex.EncodeToString(sum[:])[:12]
	family := normalizedFileFamily(fileName)
	if family == "" {
		return fmt.Sprintf("same_series:%s:%s", pathKey, earliest.Format("20060102T1504"))
	}
	return fmt.Sprintf("same_series:%s:%s:%s", pathKey, family, earliest.Format("20060102T1504"))
}

func filterCandidates(anchor FileContext, candidates []SeriesCandidateFile) []SeriesCandidateFile {
	family := normalizedFileFamily(anchor.FileName)
	filtered := make([]SeriesCandidateFile, 0, len(candidates))
	for _, candidate := range candidates {
		sameParent := anchor.ParentPath != "" && candidate.ParentPath == anchor.ParentPath
		sameFamily := family != "" && normalizedFileFamily(candidate.FileName) == family
		if candidate.FileID == anchor.FileID {
			filtered = append(filtered, candidate)
			continue
		}
		if captureTypeConflict(anchor.CaptureType, candidate.CaptureType) {
			continue
		}
		if anchor.MediaType == "video" && !videoDurationCompatible(anchor.DurationMS, candidate.DurationMS) {
			continue
		}
		if anchor.MediaType == "video" && !videoOrientationCompatible(anchor.Width, anchor.Height, candidate.Width, candidate.Height) {
			continue
		}
		if anchor.MediaType == "video" && !videoResolutionCompatible(anchor.Width, anchor.Height, candidate.Width, candidate.Height) {
			continue
		}
		if anchor.MediaType == "image" && !imageAspectRatioCompatible(anchor.Width, anchor.Height, candidate.Width, candidate.Height) {
			continue
		}
		if anchor.MediaType == "image" && !imageResolutionCompatible(anchor.Width, anchor.Height, candidate.Width, candidate.Height) {
			continue
		}
		if sameFamily && !sameParent && !withinSeriesWindow(anchor.ModTime, candidate.ModTime, crossDirectoryFamilyWindow) {
			continue
		}
		if sameParent && sameFamily {
			filtered = append(filtered, candidate)
			continue
		}
		switch anchor.MediaType {
		case "image":
			if isNearImage(anchor.ImagePHash, candidate.ImagePHash) ||
				(sameEmbeddingType(anchor.ImageEmbeddingType, candidate.ImageEmbeddingType) &&
					sameEmbeddingModel(anchor.ImageEmbeddingModel, candidate.ImageEmbeddingModel) &&
					isNearEmbedding(anchor.ImageEmbedding, candidate.ImageEmbedding, imageSeriesEmbeddingDistanceThreshold)) {
				if sameParent || sameFamily {
					filtered = append(filtered, candidate)
				}
			}
		case "video":
			if sharedPHashCount(anchor.VideoFramePHashes, candidate.VideoFramePHashes) >= 1 ||
				(sameEmbeddingType(anchor.VideoFrameEmbeddingType, candidate.VideoFrameEmbeddingType) &&
					sameEmbeddingModel(anchor.VideoFrameEmbeddingModel, candidate.VideoFrameEmbeddingModel) &&
					sharedEmbeddingCount(anchor.VideoFrameEmbeddings, candidate.VideoFrameEmbeddings, videoSeriesEmbeddingDistanceThreshold) >= 1) {
				if sameParent || sameFamily {
					filtered = append(filtered, candidate)
				}
			}
		}
	}
	slices.SortFunc(filtered, func(left, right SeriesCandidateFile) int {
		if left.ModTime.Before(right.ModTime) {
			return -1
		}
		if left.ModTime.After(right.ModTime) {
			return 1
		}
		switch {
		case left.FileID < right.FileID:
			return -1
		case left.FileID > right.FileID:
			return 1
		default:
			return 0
		}
	})
	return filtered
}

func imageAspectRatioCompatible(anchorWidth int64, anchorHeight int64, candidateWidth int64, candidateHeight int64) bool {
	anchorRatio, ok := normalizedAspectRatio(anchorWidth, anchorHeight)
	if !ok {
		return true
	}
	candidateRatio, ok := normalizedAspectRatio(candidateWidth, candidateHeight)
	if !ok {
		return true
	}
	return math.Abs(anchorRatio-candidateRatio) <= imageSeriesAspectRatioThreshold
}

func imageResolutionCompatible(anchorWidth int64, anchorHeight int64, candidateWidth int64, candidateHeight int64) bool {
	anchorPixels, ok := totalPixels(anchorWidth, anchorHeight)
	if !ok {
		return true
	}
	candidatePixels, ok := totalPixels(candidateWidth, candidateHeight)
	if !ok {
		return true
	}
	shorter := minFloat64(anchorPixels, candidatePixels)
	longer := maxFloat64(anchorPixels, candidatePixels)
	return shorter/longer >= imageSeriesResolutionRatioThreshold
}

func normalizedAspectRatio(width int64, height int64) (float64, bool) {
	if width <= 0 || height <= 0 {
		return 0, false
	}
	return float64(width) / float64(height), true
}

func imageOrientation(width int64, height int64) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if width > height {
		return "landscape"
	}
	if height > width {
		return "portrait"
	}
	return "square"
}

func totalPixels(width int64, height int64) (float64, bool) {
	if width <= 0 || height <= 0 {
		return 0, false
	}
	return float64(width * height), true
}

func videoDurationCompatible(anchorDurationMS int64, candidateDurationMS int64) bool {
	if anchorDurationMS <= 0 || candidateDurationMS <= 0 {
		return true
	}
	shorter := minInt64(anchorDurationMS, candidateDurationMS)
	longer := maxInt64(anchorDurationMS, candidateDurationMS)
	return float64(shorter)/float64(longer) >= videoSeriesDurationRatioThreshold
}

func videoOrientationCompatible(anchorWidth int64, anchorHeight int64, candidateWidth int64, candidateHeight int64) bool {
	anchorOrientation := imageOrientation(anchorWidth, anchorHeight)
	candidateOrientation := imageOrientation(candidateWidth, candidateHeight)
	if anchorOrientation == "" || candidateOrientation == "" {
		return true
	}
	if anchorOrientation != candidateOrientation {
		return false
	}
	anchorRatio, ok := normalizedAspectRatio(anchorWidth, anchorHeight)
	if !ok {
		return true
	}
	candidateRatio, ok := normalizedAspectRatio(candidateWidth, candidateHeight)
	if !ok {
		return true
	}
	return math.Abs(anchorRatio-candidateRatio) <= videoSeriesAspectRatioThreshold
}

func videoResolutionCompatible(anchorWidth int64, anchorHeight int64, candidateWidth int64, candidateHeight int64) bool {
	anchorPixels, ok := totalPixels(anchorWidth, anchorHeight)
	if !ok {
		return true
	}
	candidatePixels, ok := totalPixels(candidateWidth, candidateHeight)
	if !ok {
		return true
	}
	shorter := minFloat64(anchorPixels, candidatePixels)
	longer := maxFloat64(anchorPixels, candidatePixels)
	return shorter/longer >= videoSeriesResolutionRatioThreshold
}

func minInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func minFloat64(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func maxFloat64(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func captureTypeConflict(left string, right string) bool {
	left = normalizeCaptureType(left)
	right = normalizeCaptureType(right)
	return left != "" && right != "" && left != right
}

func normalizeCaptureType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "selfie", "自拍":
		return "selfie"
	case "screenshot", "截图", "screen_recording", "屏幕录制":
		return "screen"
	case "photo", "实拍", "live_action":
		return "photo"
	default:
		return value
	}
}

func withinSeriesWindow(left time.Time, right time.Time, limit time.Duration) bool {
	if left.IsZero() || right.IsZero() {
		return false
	}
	delta := left.Sub(right)
	if delta < 0 {
		delta = -delta
	}
	return delta <= limit
}

func assignSeriesRoles(files []SeriesCandidateFile) []SeriesCandidateFile {
	if len(files) == 0 {
		return nil
	}
	result := append([]SeriesCandidateFile(nil), files...)
	focusIndex := len(result) / 2
	for index := range result {
		result[index].Role = "member"
		if index == focusIndex {
			result[index].Role = "series_focus"
		}
	}
	return result
}

func mergeSeriesCandidates(primary []SeriesCandidateFile, extra []SeriesCandidateFile) []SeriesCandidateFile {
	if len(extra) == 0 {
		return primary
	}
	merged := make([]SeriesCandidateFile, 0, len(primary)+len(extra))
	seen := make(map[int64]struct{}, len(primary)+len(extra))
	for _, item := range primary {
		if _, ok := seen[item.FileID]; ok {
			continue
		}
		seen[item.FileID] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range extra {
		if _, ok := seen[item.FileID]; ok {
			continue
		}
		seen[item.FileID] = struct{}{}
		merged = append(merged, item)
	}
	return merged
}

func sameEmbeddingModel(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return left == right
}

func sameEmbeddingType(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return left == right
}

func normalizedFileFamily(fileName string) string {
	base := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(fileName, filepath.Ext(fileName))))
	if base == "" {
		return ""
	}
	replacer := strings.NewReplacer("_", "-", " ", "-", ".", "-")
	base = replacer.Replace(base)
	base = trailingIndexPattern.ReplaceAllString(base, "")
	base = strings.Trim(base, "-")
	return base
}

func isNearImage(left string, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	distance, ok := hammingDistanceHex(left, right)
	return ok && distance <= imageSeriesPHashHammingThreshold
}

func sharedPHashCount(left []string, right []string) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(left))
	for _, value := range left {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	count := 0
	for _, value := range right {
		if value == "" {
			continue
		}
		if _, ok := set[value]; ok {
			count++
		}
	}
	return count
}

func isNearEmbedding(left string, right string, threshold float64) bool {
	leftValues, ok := parseVector(left)
	if !ok {
		return false
	}
	rightValues, ok := parseVector(right)
	if !ok {
		return false
	}
	if len(leftValues) != len(rightValues) {
		return false
	}
	var total float64
	for index := range leftValues {
		total += math.Abs(leftValues[index] - rightValues[index])
	}
	return total/float64(len(leftValues)) <= threshold
}

func sharedEmbeddingCount(left []string, right []string, threshold float64) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	count := 0
	for _, leftValue := range left {
		for _, rightValue := range right {
			if isNearEmbedding(leftValue, rightValue, threshold) {
				count++
				break
			}
		}
	}
	return count
}

func parseVector(value string) ([]float64, bool) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if trimmed == "" {
		return nil, false
	}
	parts := strings.Split(trimmed, ",")
	result := make([]float64, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, false
		}
		result = append(result, number)
	}
	return result, len(result) > 0
}

func hammingDistanceHex(left string, right string) (int, bool) {
	leftBytes, err := hex.DecodeString(left)
	if err != nil {
		return 0, false
	}
	rightBytes, err := hex.DecodeString(right)
	if err != nil {
		return 0, false
	}
	if len(leftBytes) != len(rightBytes) {
		return 0, false
	}
	if len(leftBytes) == 0 {
		return 0, false
	}
	distance := 0
	for index := range leftBytes {
		distance += bitsSet(leftBytes[index] ^ rightBytes[index])
	}
	return distance, true
}

func bitsSet(value byte) int {
	count := 0
	for value != 0 {
		count++
		value &= value - 1
	}
	return count
}
