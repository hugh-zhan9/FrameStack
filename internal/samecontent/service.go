package samecontent

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"slices"
	"strconv"
	"strings"
)

const imagePHashHammingThreshold = 4
const imageEmbeddingDistanceThreshold = 0.08
const imageEmbeddingAspectRatioThreshold = 0.20
const imageEmbeddingResolutionRatioThreshold = 0.20
const videoEmbeddingDistanceThreshold = 0.10
const videoEmbeddingDurationRatioThreshold = 0.85
const videoEmbeddingAspectRatioThreshold = 0.20

type FileHash struct {
	FileID                   int64
	MediaType                string
	DurationMS               int64
	Width                    int64
	Height                   int64
	SHA256                   string
	ImagePHash               string
	ImageEmbedding           string
	ImageEmbeddingType       string
	ImageEmbeddingModel      string
	VideoFramePHashes        []string
	VideoFrameEmbeddings     []string
	VideoFrameEmbeddingType  string
	VideoFrameEmbeddingModel string
}

type DuplicateFile struct {
	FileID        int64
	Score         float64
	Role          string
	QualityScore  float64
	QualityTier   string
	Width         int64
	Height        int64
	DurationMS    int64
	SizeBytes     int64
	Bitrate       int64
	FPS           float64
	Container     string
}

type ImageCandidate struct {
	FileID         int64
	PHash          string
	Embedding      string
	EmbeddingType  string
	EmbeddingModel string
	QualityScore   float64
	QualityTier    string
	Width          int64
	Height         int64
	SizeBytes      int64
}

type Store interface {
	GetFileHash(ctx context.Context, fileID int64) (FileHash, error)
	ListDuplicateFiles(ctx context.Context, sha256 string) ([]DuplicateFile, error)
	ListImagePHashCandidates(ctx context.Context, prefix string) ([]ImageCandidate, error)
	ListImageEmbeddingCandidates(ctx context.Context, prefix string, model string) ([]ImageCandidate, error)
	ListVideoFramePHashMatches(ctx context.Context, phashes []string) ([]DuplicateFile, error)
	ListVideoFrameEmbeddingMatches(ctx context.Context, embeddings []string, model string) ([]DuplicateFile, error)
	UpsertSameContentCluster(ctx context.Context, sha256 string, files []DuplicateFile) error
	DeactivateSameContentCluster(ctx context.Context, sha256 string) error
}

type Service struct {
	Store Store
}

func (s Service) ClusterFile(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFileHash(ctx, fileID)
	if err != nil {
		return err
	}
	if file.SHA256 != "" {
		duplicates, err := s.Store.ListDuplicateFiles(ctx, file.SHA256)
		if err != nil {
			return err
		}
		if len(duplicates) >= 2 {
			duplicates = rankDuplicateFiles(duplicates)
			return s.Store.UpsertSameContentCluster(ctx, file.SHA256, duplicates)
		}
		if err := s.Store.DeactivateSameContentCluster(ctx, file.SHA256); err != nil {
			return err
		}
	}
	if file.MediaType == "video" {
		if len(file.VideoFramePHashes) < 2 && len(file.VideoFrameEmbeddings) < 2 {
			return nil
		}
		matches := []DuplicateFile(nil)
		clusterKey := ""
		if len(file.VideoFramePHashes) >= 2 {
			var err error
			matches, err = s.Store.ListVideoFramePHashMatches(ctx, file.VideoFramePHashes)
			if err != nil {
				return err
			}
			matches = filterVideoMatches(file, matches)
			clusterKey = VideoClusterKey(file.VideoFramePHashes)
		}
		if len(matches) < 2 && len(file.VideoFrameEmbeddings) >= 2 && file.VideoFrameEmbeddingType == "video_frame_visual" && file.VideoFrameEmbeddingModel != "" {
			var err error
			matches, err = s.Store.ListVideoFrameEmbeddingMatches(ctx, file.VideoFrameEmbeddings, file.VideoFrameEmbeddingModel)
			if err != nil {
				return err
			}
			matches = filterVideoMatches(file, matches)
			clusterKey = VideoEmbeddingClusterKey(file.VideoFrameEmbeddings)
		}
		if len(matches) < 2 {
			return s.Store.DeactivateSameContentCluster(ctx, clusterKey)
		}
		matches = rankDuplicateFiles(matches)
		return s.Store.UpsertSameContentCluster(ctx, clusterKey, matches)
	}
	if file.MediaType != "image" {
		return nil
	}
	if file.ImagePHash == "" && file.ImageEmbedding == "" {
		return nil
	}
	matches := []DuplicateFile(nil)
	clusterKey := ""
	if file.ImagePHash != "" {
		candidates, err := s.Store.ListImagePHashCandidates(ctx, imagePHashPrefix(file.ImagePHash))
		if err != nil {
			return err
		}
		matches = filterImagePHashMatches(file, candidates)
		clusterKey = ImageClusterKey(file.ImagePHash)
	}
	if len(matches) < 2 && file.ImageEmbedding != "" && file.ImageEmbeddingType == "image_visual" && file.ImageEmbeddingModel != "" {
		candidates, err := s.Store.ListImageEmbeddingCandidates(ctx, file.ImageEmbedding, file.ImageEmbeddingModel)
		if err != nil {
			return err
		}
		matches = filterImageEmbeddingMatches(file, candidates)
		clusterKey = ImageEmbeddingClusterKey(file.ImageEmbedding)
	}
	if len(matches) < 2 {
		return s.Store.DeactivateSameContentCluster(ctx, clusterKey)
	}
	matches = rankDuplicateFiles(matches)
	return s.Store.UpsertSameContentCluster(ctx, clusterKey, matches)
}

func filterVideoMatches(anchor FileHash, matches []DuplicateFile) []DuplicateFile {
	filtered := make([]DuplicateFile, 0, len(matches))
	for _, match := range matches {
		if !durationsCompatible(anchor.DurationMS, match.DurationMS) {
			continue
		}
		if !videoAspectRatioCompatible(anchor, match) {
			continue
		}
		filtered = append(filtered, match)
	}
	return filtered
}

func durationsCompatible(anchorDurationMS int64, candidateDurationMS int64) bool {
	if anchorDurationMS <= 0 || candidateDurationMS <= 0 {
		return true
	}
	shorter := minInt64(anchorDurationMS, candidateDurationMS)
	longer := maxInt64(anchorDurationMS, candidateDurationMS)
	return float64(shorter)/float64(longer) >= videoEmbeddingDurationRatioThreshold
}

func videoAspectRatioCompatible(anchor FileHash, candidate DuplicateFile) bool {
	anchorOrientation := imageOrientation(anchor.Width, anchor.Height)
	candidateOrientation := imageOrientation(candidate.Width, candidate.Height)
	if anchorOrientation == "" || candidateOrientation == "" {
		return true
	}
	if anchorOrientation != candidateOrientation {
		return false
	}
	anchorRatio, ok := normalizedAspectRatio(anchor.Width, anchor.Height)
	if !ok {
		return true
	}
	candidateRatio, ok := normalizedAspectRatio(candidate.Width, candidate.Height)
	if !ok {
		return true
	}
	return math.Abs(anchorRatio-candidateRatio) <= videoEmbeddingAspectRatioThreshold
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

func rankDuplicateFiles(files []DuplicateFile) []DuplicateFile {
	if len(files) == 0 {
		return nil
	}
	ranked := append([]DuplicateFile(nil), files...)
	sort.SliceStable(ranked, func(left int, right int) bool {
		leftScore := duplicateQualityScore(ranked[left])
		rightScore := duplicateQualityScore(ranked[right])
		if leftScore == rightScore {
			return ranked[left].FileID < ranked[right].FileID
		}
		return leftScore > rightScore
	})
	for index := range ranked {
		if index == 0 {
			ranked[index].Role = "best_quality"
			continue
		}
		ranked[index].Role = "duplicate_candidate"
	}
	topScore := duplicateQualityScore(ranked[0])
	if topScore <= 0 {
		topScore = 1
	}
	for index := range ranked {
		ranked[index].Score = duplicateMemberScore(duplicateQualityScore(ranked[index]), topScore)
	}
	return ranked
}

func duplicateQualityScore(file DuplicateFile) float64 {
	score := file.QualityScore * 100
	score += float64(file.Width * file.Height)
	score += float64(file.DurationMS) / 1000
	score += float64(file.SizeBytes) / 1_000_000
	score += float64(file.Bitrate) / 100_000
	score += file.FPS * 10
	switch strings.ToLower(strings.TrimSpace(file.Container)) {
	case "mkv":
		score += 6
	case "mp4":
		score += 4
	case "mov":
		score += 3
	}
	return score
}

func duplicateMemberScore(score float64, topScore float64) float64 {
	if topScore <= 0 {
		return 1
	}
	value := score / topScore
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func ClusterTitle(sha256 string) string {
	if len(sha256) > 12 {
		sha256 = sha256[:12]
	}
	return fmt.Sprintf("same_content:%s", sha256)
}

func ImageClusterKey(phash string) string {
	return fmt.Sprintf("image:%s", phash)
}

func ImageEmbeddingClusterKey(embedding string) string {
	return fmt.Sprintf("image_embedding:%s", imageEmbeddingPrefix(embedding))
}

func VideoClusterKey(phashes []string) string {
	normalized := uniqueSortedPHashes(phashes)
	if len(normalized) > 2 {
		normalized = normalized[:2]
	}
	return fmt.Sprintf("video:%s", strings.Join(normalized, "+"))
}

func VideoEmbeddingClusterKey(embeddings []string) string {
	normalized := uniqueSortedEmbeddings(embeddings)
	if len(normalized) > 2 {
		normalized = normalized[:2]
	}
	parts := make([]string, 0, len(normalized))
	for _, value := range normalized {
		parts = append(parts, imageEmbeddingPrefix(value))
	}
	return fmt.Sprintf("video_embedding:%s", strings.Join(parts, "+"))
}

func uniqueSortedPHashes(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	items := make([]string, 0, len(set))
	for value := range set {
		items = append(items, value)
	}
	slices.Sort(items)
	return items
}

func imagePHashPrefix(phash string) string {
	if len(phash) <= 4 {
		return phash
	}
	return phash[:4]
}

func imageEmbeddingPrefix(vector string) string {
	trimmed := strings.TrimSpace(vector)
	if len(trimmed) <= 12 {
		return trimmed
	}
	return trimmed[:12]
}

func filterImagePHashMatches(anchor FileHash, candidates []ImageCandidate) []DuplicateFile {
	var matches []DuplicateFile
	for _, candidate := range candidates {
		distance, ok := hammingDistanceHex(anchor.ImagePHash, candidate.PHash)
		if !ok || distance > imagePHashHammingThreshold {
			continue
		}
		if !imageAspectRatioCompatible(fileAnchorImageCandidate(anchor), candidate) {
			continue
		}
		matches = append(matches, DuplicateFile{
			FileID:       candidate.FileID,
			QualityScore: candidate.QualityScore,
			QualityTier:  candidate.QualityTier,
			Width:        candidate.Width,
			Height:       candidate.Height,
			SizeBytes:    candidate.SizeBytes,
		})
	}
	return matches
}

func filterImageEmbeddingMatches(anchor FileHash, candidates []ImageCandidate) []DuplicateFile {
	anchorCandidate := anchorImageCandidate(anchor, candidates)
	var matches []DuplicateFile
	for _, candidate := range candidates {
		if anchor.ImageEmbeddingType != "" && candidate.EmbeddingType != "" && candidate.EmbeddingType != anchor.ImageEmbeddingType {
			continue
		}
		if anchor.ImageEmbeddingModel != "" && candidate.EmbeddingModel != "" && candidate.EmbeddingModel != anchor.ImageEmbeddingModel {
			continue
		}
		if !isNearEmbedding(anchor.ImageEmbedding, candidate.Embedding, imageEmbeddingDistanceThreshold) {
			continue
		}
		if !imageAspectRatioCompatible(anchorCandidate, candidate) {
			continue
		}
		if !imageResolutionCompatible(anchorCandidate, candidate) {
			continue
		}
		matches = append(matches, DuplicateFile{
			FileID:       candidate.FileID,
			QualityScore: candidate.QualityScore,
			QualityTier:  candidate.QualityTier,
			Width:        candidate.Width,
			Height:       candidate.Height,
			SizeBytes:    candidate.SizeBytes,
		})
	}
	return matches
}

func anchorImageCandidate(anchor FileHash, candidates []ImageCandidate) ImageCandidate {
	if anchor.Width > 0 && anchor.Height > 0 {
		return ImageCandidate{
			FileID:         anchor.FileID,
			Width:          anchor.Width,
			Height:         anchor.Height,
			PHash:          anchor.ImagePHash,
			Embedding:      anchor.ImageEmbedding,
			EmbeddingType:  anchor.ImageEmbeddingType,
			EmbeddingModel: anchor.ImageEmbeddingModel,
		}
	}
	return imageAnchorCandidate(anchor.ImageEmbedding, candidates)
}

func fileAnchorImageCandidate(anchor FileHash) ImageCandidate {
	return ImageCandidate{
		FileID:         anchor.FileID,
		Width:          anchor.Width,
		Height:         anchor.Height,
		PHash:          anchor.ImagePHash,
		Embedding:      anchor.ImageEmbedding,
		EmbeddingType:  anchor.ImageEmbeddingType,
		EmbeddingModel: anchor.ImageEmbeddingModel,
	}
}

func imageAnchorCandidate(target string, candidates []ImageCandidate) ImageCandidate {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Embedding) == strings.TrimSpace(target) && candidate.Width > 0 && candidate.Height > 0 {
			return candidate
		}
	}
	for _, candidate := range candidates {
		if candidate.Width > 0 && candidate.Height > 0 {
			return candidate
		}
	}
	return ImageCandidate{}
}

func imageAspectRatioCompatible(anchor ImageCandidate, candidate ImageCandidate) bool {
	anchorOrientation := imageOrientation(anchor.Width, anchor.Height)
	candidateOrientation := imageOrientation(candidate.Width, candidate.Height)
	if anchorOrientation != "" && candidateOrientation != "" && anchorOrientation != candidateOrientation {
		return false
	}
	anchorRatio, ok := normalizedAspectRatio(anchor.Width, anchor.Height)
	if !ok {
		return true
	}
	candidateRatio, ok := normalizedAspectRatio(candidate.Width, candidate.Height)
	if !ok {
		return true
	}
	return math.Abs(anchorRatio-candidateRatio) <= imageEmbeddingAspectRatioThreshold
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

func imageResolutionCompatible(anchor ImageCandidate, candidate ImageCandidate) bool {
	anchorPixels, ok := totalPixels(anchor.Width, anchor.Height)
	if !ok {
		return true
	}
	candidatePixels, ok := totalPixels(candidate.Width, candidate.Height)
	if !ok {
		return true
	}
	shorter := minFloat64(anchorPixels, candidatePixels)
	longer := maxFloat64(anchorPixels, candidatePixels)
	return shorter/longer >= imageEmbeddingResolutionRatioThreshold
}

func totalPixels(width int64, height int64) (float64, bool) {
	if width <= 0 || height <= 0 {
		return 0, false
	}
	return float64(width * height), true
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

func uniqueSortedEmbeddings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		set[value] = struct{}{}
	}
	items := make([]string, 0, len(set))
	for value := range set {
		items = append(items, value)
	}
	slices.Sort(items)
	return items
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
