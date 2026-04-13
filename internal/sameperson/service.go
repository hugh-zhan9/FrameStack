package sameperson

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const aiCandidateFilterThreshold = 4
const imagePersonEmbeddingDistanceThreshold = 0.08
const videoPersonEmbeddingDistanceThreshold = 0.10
const weakAutoSignalTimeWindow = 14 * 24 * time.Hour
const weakAutoSignalVideoDurationRatioThreshold = 0.85
const weakAutoSignalVideoAspectRatioThreshold = 0.20
const weakAutoSignalVideoResolutionRatioThreshold = 0.20
const weakAutoSignalImageAspectRatioThreshold = 0.20
const weakAutoSignalImageResolutionRatioThreshold = 0.20

type PersonTag struct {
	Name   string
	Source string
}

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
	HasFace                  bool
	SubjectCount             string
	CaptureType              string
	ImageEmbedding           string
	ImageEmbeddingType       string
	ImageEmbeddingModel      string
	VideoFrameEmbeddings     []string
	VideoFrameEmbeddingType  string
	VideoFrameEmbeddingModel string
}

type PersonCandidateFile struct {
	FileID                   int64
	ParentPath               string
	FileName                 string
	MediaType                string
	ModTime                  time.Time
	Score                    float64
	DurationMS               int64
	Width                    int64
	Height                   int64
	HasFace                  bool
	SubjectCount             string
	CaptureType              string
	ImageEmbedding           string
	ImageEmbeddingType       string
	ImageEmbeddingModel      string
	VideoFrameEmbeddings     []string
	VideoFrameEmbeddingType  string
	VideoFrameEmbeddingModel string
}

type Store interface {
	GetFileContext(ctx context.Context, fileID int64) (FileContext, error)
	ListPersonTags(ctx context.Context, fileID int64) ([]PersonTag, error)
	ListAutoPersonTags(ctx context.Context, fileID int64) ([]PersonTag, error)
	ListFilesWithPersonTag(ctx context.Context, tagName string) ([]PersonCandidateFile, error)
	ListFilesWithAutoPersonTag(ctx context.Context, tagName string) ([]PersonCandidateFile, error)
	UpsertSamePersonCluster(ctx context.Context, title string, files []PersonCandidateFile) error
	DeactivateSamePersonCluster(ctx context.Context, title string) error
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
	tags, err := s.Store.ListPersonTags(ctx, fileID)
	if err != nil {
		return err
	}
	dedupedTags := dedupeTags(tags)
	for _, tag := range dedupedTags {
		if err := s.ClusterTag(ctx, file, tag); err != nil {
			return err
		}
	}
	if len(dedupedTags) > 0 {
		return nil
	}
	autoTags, err := s.Store.ListAutoPersonTags(ctx, fileID)
	if err != nil {
		return err
	}
	for _, tag := range dedupeTags(autoTags) {
		if err := s.ClusterTag(ctx, file, tag); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) ClusterTag(ctx context.Context, anchor FileContext, tag PersonTag) error {
	var (
		candidates []PersonCandidateFile
		err        error
	)
	if tag.Source == "auto" {
		candidates, err = s.Store.ListFilesWithAutoPersonTag(ctx, tag.Name)
	} else {
		candidates, err = s.Store.ListFilesWithPersonTag(ctx, tag.Name)
	}
	if err != nil {
		return err
	}
	title := ClusterTitle(tag, anchor)
	if tag.Source == "auto" {
		candidates = narrowAutoCandidates(anchor, tag, candidates)
	} else if tag.Source != "human" && len(candidates) > aiCandidateFilterThreshold {
		candidates = narrowAICandidates(anchor, candidates)
	}
	candidates = scoreCandidates(anchor, tag, candidates)
	if len(candidates) < 2 {
		return s.Store.DeactivateSamePersonCluster(ctx, title)
	}
	return s.Store.UpsertSamePersonCluster(ctx, title, candidates)
}

func ClusterKey(tagName string) string {
	sum := sha1.Sum([]byte(tagName))
	return fmt.Sprintf("same_person:%s", hex.EncodeToString(sum[:])[:12])
}

func ClusterTitle(tag PersonTag, anchor FileContext) string {
	if tag.Source == "human" {
		return fmt.Sprintf("person:%s", tag.Name)
	}
	if tag.Source == "auto" {
		family := normalizedFileFamily(anchor.FileName)
		if family != "" {
			return fmt.Sprintf("person_auto:%s:%s", tag.Name, family)
		}
		sum := sha1.Sum([]byte(anchor.ParentPath))
		return fmt.Sprintf("person_auto:%s:%s", tag.Name, hex.EncodeToString(sum[:])[:8])
	}
	family := normalizedFileFamily(anchor.FileName)
	if family != "" {
		return fmt.Sprintf("person_ai:%s:%s", tag.Name, family)
	}
	sum := sha1.Sum([]byte(anchor.ParentPath))
	return fmt.Sprintf("person_ai:%s:%s", tag.Name, hex.EncodeToString(sum[:])[:8])
}

func dedupeTags(items []PersonTag) []PersonTag {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	prioritized := make(map[string]PersonTag, len(items))
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		existing, ok := prioritized[item.Name]
		if !ok || existing.Source != "human" && item.Source == "human" {
			prioritized[item.Name] = item
		}
	}
	result := make([]PersonTag, 0, len(items))
	for _, item := range items {
		item = prioritized[item.Name]
		if _, ok := seen[item.Name]; ok {
			continue
		}
		seen[item.Name] = struct{}{}
		result = append(result, item)
	}
	return result
}

func narrowAICandidates(anchor FileContext, candidates []PersonCandidateFile) []PersonCandidateFile {
	family := normalizedFileFamily(anchor.FileName)
	filtered := make([]PersonCandidateFile, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.FileID == anchor.FileID {
			filtered = append(filtered, candidate)
			continue
		}
		if !videoDurationCompatible(anchor, candidate) {
			continue
		}
		if !videoOrientationCompatible(anchor, candidate) {
			continue
		}
		if !videoResolutionCompatible(anchor, candidate) {
			continue
		}
		if !imageAspectRatioCompatible(anchor, candidate) {
			continue
		}
		if !imageResolutionCompatible(anchor, candidate) {
			continue
		}
		if family != "" && normalizedFileFamily(candidate.FileName) == family {
			filtered = append(filtered, candidate)
			continue
		}
		if anchor.ParentPath != "" && candidate.ParentPath == anchor.ParentPath {
			filtered = append(filtered, candidate)
			continue
		}
		if hasNearPersonEmbedding(anchor, candidate) {
			filtered = append(filtered, candidate)
			continue
		}
	}
	return filtered
}

func narrowAutoCandidates(anchor FileContext, tag PersonTag, candidates []PersonCandidateFile) []PersonCandidateFile {
	family := normalizedFileFamily(anchor.FileName)
	filtered := make([]PersonCandidateFile, 0, len(candidates))
	weakSignal := isWeakAutoSignal(tag.Name)
	for _, candidate := range candidates {
		if candidate.FileID == anchor.FileID {
			filtered = append(filtered, candidate)
			continue
		}
		familyMatch := family != "" && normalizedFileFamily(candidate.FileName) == family
		parentMatch := anchor.ParentPath != "" && candidate.ParentPath == anchor.ParentPath
		embeddingMatch := hasNearPersonEmbedding(anchor, candidate)
		if weakSignal {
			if hasConflictingPersonShape(anchor, candidate) {
				continue
			}
			if !videoDurationCompatible(anchor, candidate) {
				continue
			}
			if !videoOrientationCompatible(anchor, candidate) {
				continue
			}
			if !videoResolutionCompatible(anchor, candidate) {
				continue
			}
			if !imageAspectRatioCompatible(anchor, candidate) {
				continue
			}
			if !imageResolutionCompatible(anchor, candidate) {
				continue
			}
			if familyMatch || (embeddingMatch && withinTimeWindow(anchor.ModTime, candidate.ModTime, weakAutoSignalTimeWindow)) {
				filtered = append(filtered, candidate)
			}
			continue
		}
		if videoOrientationCompatible(anchor, candidate) && videoResolutionCompatible(anchor, candidate) && imageAspectRatioCompatible(anchor, candidate) && imageResolutionCompatible(anchor, candidate) && (familyMatch || parentMatch || embeddingMatch) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func videoDurationCompatible(anchor FileContext, candidate PersonCandidateFile) bool {
	if anchor.MediaType != "video" || candidate.MediaType != "video" {
		return true
	}
	if anchor.DurationMS <= 0 || candidate.DurationMS <= 0 {
		return true
	}
	shorter := minInt64(anchor.DurationMS, candidate.DurationMS)
	longer := maxInt64(anchor.DurationMS, candidate.DurationMS)
	return float64(shorter)/float64(longer) >= weakAutoSignalVideoDurationRatioThreshold
}

func videoOrientationCompatible(anchor FileContext, candidate PersonCandidateFile) bool {
	if anchor.MediaType != "video" || candidate.MediaType != "video" {
		return true
	}
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
	return math.Abs(anchorRatio-candidateRatio) <= weakAutoSignalVideoAspectRatioThreshold
}

func videoResolutionCompatible(anchor FileContext, candidate PersonCandidateFile) bool {
	if anchor.MediaType != "video" || candidate.MediaType != "video" {
		return true
	}
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
	return shorter/longer >= weakAutoSignalVideoResolutionRatioThreshold
}

func imageAspectRatioCompatible(anchor FileContext, candidate PersonCandidateFile) bool {
	if anchor.MediaType != "image" || candidate.MediaType != "image" {
		return true
	}
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
	return math.Abs(anchorRatio-candidateRatio) <= weakAutoSignalImageAspectRatioThreshold
}

func imageResolutionCompatible(anchor FileContext, candidate PersonCandidateFile) bool {
	if anchor.MediaType != "image" || candidate.MediaType != "image" {
		return true
	}
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
	return shorter/longer >= weakAutoSignalImageResolutionRatioThreshold
}

func normalizedAspectRatio(width int64, height int64) (float64, bool) {
	if width <= 0 || height <= 0 {
		return 0, false
	}
	longer := maxFloat64(float64(width), float64(height))
	shorter := minFloat64(float64(width), float64(height))
	if longer == 0 {
		return 0, false
	}
	return shorter / longer, true
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

func isWeakAutoSignal(name string) bool {
	value := strings.ToLower(strings.TrimSpace(name))
	if value == "" {
		return false
	}
	for _, token := range []string{"多人", "情侣", "av", "做爱", "口交"} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func scoreCandidates(anchor FileContext, tag PersonTag, candidates []PersonCandidateFile) []PersonCandidateFile {
	if len(candidates) == 0 {
		return nil
	}
	scored := make([]PersonCandidateFile, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.Score = candidateScore(anchor, tag, candidate)
		scored = append(scored, candidate)
	}
	sort.SliceStable(scored, func(left int, right int) bool {
		if scored[left].FileID == anchor.FileID {
			return true
		}
		if scored[right].FileID == anchor.FileID {
			return false
		}
		if scored[left].Score == scored[right].Score {
			return scored[left].FileID < scored[right].FileID
		}
		return scored[left].Score > scored[right].Score
	})
	return scored
}

func candidateScore(anchor FileContext, tag PersonTag, candidate PersonCandidateFile) float64 {
	if candidate.FileID == anchor.FileID {
		return 1.0
	}
	score := baseScoreForTagSource(tag.Source)
	if normalizedFileFamily(anchor.FileName) != "" && normalizedFileFamily(anchor.FileName) == normalizedFileFamily(candidate.FileName) {
		score += 0.10
	}
	if anchor.ParentPath != "" && anchor.ParentPath == candidate.ParentPath {
		score += 0.08
	}
	if bonus := personEmbeddingScore(anchor, candidate); bonus > 0 {
		score += bonus
	}
	score += structuredSignalScore(anchor, candidate)
	score += timeProximityScore(anchor.ModTime, candidate.ModTime)
	if score > 0.99 {
		score = 0.99
	}
	return math.Round(score*100) / 100
}

func baseScoreForTagSource(source string) float64 {
	switch source {
	case "human":
		return 0.86
	case "ai":
		return 0.68
	case "auto":
		return 0.56
	default:
		return 0.50
	}
}

func hasNearPersonEmbedding(anchor FileContext, candidate PersonCandidateFile) bool {
	return personEmbeddingScore(anchor, candidate) > 0
}

func personEmbeddingScore(anchor FileContext, candidate PersonCandidateFile) float64 {
	bestStrength := 0.0
	recordStrength := func(leftType string, rightType string, fallback string) {
		strength := personEmbeddingMatchStrength(leftType, rightType, fallback)
		if strength > bestStrength {
			bestStrength = strength
		}
	}
	if sameEmbeddingModel(anchor.ImageEmbeddingModel, candidate.ImageEmbeddingModel) &&
		isNearEmbedding(anchor.ImageEmbedding, candidate.ImageEmbedding, imagePersonEmbeddingDistanceThreshold) {
		recordStrength(anchor.ImageEmbeddingType, candidate.ImageEmbeddingType, "generic_visual")
	}
	if sameEmbeddingModel(anchor.VideoFrameEmbeddingModel, candidate.VideoFrameEmbeddingModel) &&
		sharedEmbeddingCount(anchor.VideoFrameEmbeddings, candidate.VideoFrameEmbeddings, videoPersonEmbeddingDistanceThreshold) >= 1 {
		recordStrength(anchor.VideoFrameEmbeddingType, candidate.VideoFrameEmbeddingType, "generic_visual")
	}
	if sameEmbeddingModel(anchor.ImageEmbeddingModel, candidate.VideoFrameEmbeddingModel) &&
		imageToFrameNear(anchor.ImageEmbedding, candidate.VideoFrameEmbeddings, videoPersonEmbeddingDistanceThreshold) {
		recordStrength(anchor.ImageEmbeddingType, candidate.VideoFrameEmbeddingType, "generic_visual")
	}
	if sameEmbeddingModel(candidate.ImageEmbeddingModel, anchor.VideoFrameEmbeddingModel) &&
		imageToFrameNear(candidate.ImageEmbedding, anchor.VideoFrameEmbeddings, videoPersonEmbeddingDistanceThreshold) {
		recordStrength(candidate.ImageEmbeddingType, anchor.VideoFrameEmbeddingType, "generic_visual")
	}
	return bestStrength
}

func personEmbeddingMatchStrength(leftType string, rightType string, fallback string) float64 {
	leftType = canonicalPersonEmbeddingType(leftType, fallback)
	rightType = canonicalPersonEmbeddingType(rightType, fallback)
	if leftType == "" || rightType == "" || leftType != rightType {
		return 0
	}
	switch leftType {
	case "person_visual":
		return 0.26
	case "generic_visual":
		return 0.18
	default:
		return 0
	}
}

func canonicalPersonEmbeddingType(raw string, fallback string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "person_visual":
		return "person_visual"
	case "image_visual", "video_frame_visual", "generic_visual":
		return "generic_visual"
	case "":
		return fallback
	default:
		return value
	}
}

func structuredSignalScore(anchor FileContext, candidate PersonCandidateFile) float64 {
	score := 0.0
	if anchor.HasFace && candidate.HasFace {
		score += 0.08
	}
	if sameStructuredValue(anchor.SubjectCount, candidate.SubjectCount) {
		score += 0.04
	}
	if sameStructuredValue(anchor.CaptureType, candidate.CaptureType) {
		score += 0.04
	}
	return score
}

func sameStructuredValue(left string, right string) bool {
	left = strings.TrimSpace(strings.ToLower(left))
	right = strings.TrimSpace(strings.ToLower(right))
	return left != "" && right != "" && left == right
}

func hasConflictingPersonShape(anchor FileContext, candidate PersonCandidateFile) bool {
	if structuredConflict(anchor.SubjectCount, candidate.SubjectCount) {
		return true
	}
	if captureTypeConflict(anchor.CaptureType, candidate.CaptureType) {
		return true
	}
	if anchor.HasFace && !candidate.HasFace && candidate.SubjectCount != "" {
		return true
	}
	return false
}

func structuredConflict(left string, right string) bool {
	left = normalizeSubjectCount(left)
	right = normalizeSubjectCount(right)
	return left != "" && right != "" && left != right
}

func normalizeSubjectCount(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "single", "单人", "1", "one":
		return "single"
	case "multiple", "multi", "多人", "couple", "情侣", "2", "two":
		return "multiple"
	default:
		return value
	}
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

func withinTimeWindow(left time.Time, right time.Time, limit time.Duration) bool {
	if left.IsZero() || right.IsZero() {
		return false
	}
	delta := left.Sub(right)
	if delta < 0 {
		delta = -delta
	}
	return delta <= limit
}

func timeProximityScore(left time.Time, right time.Time) float64 {
	if left.IsZero() || right.IsZero() {
		return 0
	}
	delta := left.Sub(right)
	if delta < 0 {
		delta = -delta
	}
	switch {
	case delta <= 6*time.Hour:
		return 0.06
	case delta <= 24*time.Hour:
		return 0.04
	case delta <= 7*24*time.Hour:
		return 0.02
	default:
		return 0
	}
}

func sameEmbeddingModel(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return left == right
}

func imageToFrameNear(imageEmbedding string, frameEmbeddings []string, threshold float64) bool {
	if strings.TrimSpace(imageEmbedding) == "" || len(frameEmbeddings) == 0 {
		return false
	}
	for _, frameEmbedding := range frameEmbeddings {
		if isNearEmbedding(imageEmbedding, frameEmbedding, threshold) {
			return true
		}
	}
	return false
}

func normalizedFileFamily(fileName string) string {
	base := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(fileName, filepath.Ext(fileName))))
	base = strings.Trim(base, "-_. ")
	if base == "" {
		return ""
	}
	cut := len(base)
	for cut > 0 && base[cut-1] >= '0' && base[cut-1] <= '9' {
		cut--
	}
	base = strings.Trim(base[:cut], "-_. ")
	return base
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
