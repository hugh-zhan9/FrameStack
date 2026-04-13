package httpserver

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"idea/internal/systemstatus"
	"idea/internal/tasks"
)

//go:embed assets/index.html assets/app.css assets/app.js
var webAssets embed.FS

type TaskSummaryProvider interface {
	TaskSummary(ctx context.Context) (tasks.Summary, error)
}

type SystemStatusProvider interface {
	SystemStatus(ctx context.Context) (systemstatus.Snapshot, error)
}

type JobListProvider interface {
	ListJobs(ctx context.Context, options tasks.JobListOptions) ([]tasks.Job, error)
}

type JobCreator interface {
	CreateJob(ctx context.Context, input tasks.CreateJobInput) (tasks.Job, error)
}

type JobRetrier interface {
	RetryJob(ctx context.Context, jobID int64) (tasks.Job, error)
}

type JobEventListProvider interface {
	ListJobEvents(ctx context.Context, jobID int64, limit int) ([]tasks.JobEvent, error)
}

type VolumeDTO struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	MountPath   string `json:"mount_path"`
	IsOnline    bool   `json:"is_online"`
}

type CreateVolumeRequest struct {
	DisplayName string `json:"display_name"`
	MountPath   string `json:"mount_path"`
}

type VolumeListProvider interface {
	ListVolumes(ctx context.Context) ([]VolumeDTO, error)
}

type VolumeCreator interface {
	CreateVolume(ctx context.Context, input CreateVolumeRequest) (VolumeDTO, error)
}

type VolumeScanner interface {
	EnqueueVolumeScan(ctx context.Context, volumeID int64) (tasks.Job, error)
}

type TagDTO struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	FileCount   int64  `json:"file_count"`
}

type TagListRequest struct {
	Namespace string `json:"namespace,omitempty"`
	Limit     int    `json:"limit"`
}

type TagListProvider interface {
	ListTags(ctx context.Context, input TagListRequest) ([]TagDTO, error)
}

type ClusterDTO struct {
	ID          int64    `json:"id"`
	ClusterType string   `json:"cluster_type"`
	Title       string   `json:"title"`
	Confidence  *float64 `json:"confidence,omitempty"`
	Status      string   `json:"status"`
	CoverFileID *int64   `json:"cover_file_id,omitempty"`
	MemberCount int64    `json:"member_count"`
	StrongMemberCount int64 `json:"strong_member_count,omitempty"`
	TopMemberScore *float64 `json:"top_member_score,omitempty"`
	PersonVisualCount int64 `json:"person_visual_count,omitempty"`
	GenericVisualCount int64 `json:"generic_visual_count,omitempty"`
	TopEvidenceType string `json:"top_evidence_type,omitempty"`
	CreatedAt   string   `json:"created_at"`
}

type ClusterMemberDTO struct {
	FileID               int64    `json:"file_id"`
	FileName             string   `json:"file_name"`
	AbsPath              string   `json:"abs_path"`
	MediaType            string   `json:"media_type"`
	Role                 string   `json:"role"`
	Score                *float64 `json:"score,omitempty"`
	QualityTier          string   `json:"quality_tier,omitempty"`
	HasFace              bool     `json:"has_face,omitempty"`
	SubjectCount         string   `json:"subject_count,omitempty"`
	CaptureType          string   `json:"capture_type,omitempty"`
	EmbeddingType        string   `json:"embedding_type,omitempty"`
	EmbeddingProvider    string   `json:"embedding_provider,omitempty"`
	EmbeddingModel       string   `json:"embedding_model,omitempty"`
	EmbeddingVectorCount int64    `json:"embedding_vector_count,omitempty"`
}

type ClusterDetailDTO struct {
	ClusterDTO
	PersonVisualCount  int64              `json:"person_visual_count,omitempty"`
	GenericVisualCount int64              `json:"generic_visual_count,omitempty"`
	TopEvidenceType    string             `json:"top_evidence_type,omitempty"`
	Members            []ClusterMemberDTO `json:"members"`
}

type ClusterListRequest struct {
	ClusterType string `json:"cluster_type,omitempty"`
	Status      string `json:"status,omitempty"`
	Limit       int    `json:"limit"`
}

type ClusterListProvider interface {
	ListClusters(ctx context.Context, input ClusterListRequest) ([]ClusterDTO, error)
}

type ClusterDetailProvider interface {
	GetClusterDetail(ctx context.Context, clusterID int64) (ClusterDetailDTO, error)
}

type ClusterSummaryDTO struct {
	ClusterType  string `json:"cluster_type"`
	Status       string `json:"status"`
	ClusterCount int64  `json:"cluster_count"`
	MemberCount  int64  `json:"member_count"`
}

type ClusterSummaryProvider interface {
	SummarizeClusters(ctx context.Context) ([]ClusterSummaryDTO, error)
}

type ClusterReviewer interface {
	ApplyClusterReviewAction(ctx context.Context, clusterID int64, input FileReviewActionRequest) error
	UpdateClusterStatus(ctx context.Context, clusterID int64, status string) error
}

type FileDTO struct {
	ID           int64    `json:"id"`
	VolumeID     int64    `json:"volume_id"`
	AbsPath      string   `json:"abs_path"`
	FileName     string   `json:"file_name"`
	MediaType    string   `json:"media_type"`
	Status       string   `json:"status"`
	SizeBytes    int64    `json:"size_bytes"`
	UpdatedAt    string   `json:"updated_at"`
	Width        *int     `json:"width,omitempty"`
	Height       *int     `json:"height,omitempty"`
	DurationMS   *int64   `json:"duration_ms,omitempty"`
	Format       string   `json:"format,omitempty"`
	Container    string   `json:"container,omitempty"`
	FPS          *float64 `json:"fps,omitempty"`
	Bitrate      *int64   `json:"bitrate,omitempty"`
	VideoCodec   string   `json:"video_codec,omitempty"`
	AudioCodec   string   `json:"audio_codec,omitempty"`
	QualityScore *float64 `json:"quality_score,omitempty"`
	QualityTier  string   `json:"quality_tier,omitempty"`
	ReviewAction string   `json:"review_action,omitempty"`
	TagNames     []string `json:"tag_names,omitempty"`
	HasPreview   bool     `json:"has_preview"`
}

type PathHistoryDTO struct {
	AbsPath   string `json:"abs_path"`
	EventType string `json:"event_type"`
	SeenAt    string `json:"seen_at"`
}

type FileDetailDTO struct {
	FileDTO
	PathHistory     []PathHistoryDTO     `json:"path_history"`
	CurrentAnalyses []CurrentAnalysisDTO `json:"current_analyses"`
	Tags            []FileTagDTO         `json:"tags"`
	ReviewActions   []ReviewActionDTO    `json:"review_actions"`
	Clusters        []FileClusterDTO     `json:"clusters"`
	Embeddings      []EmbeddingInfoDTO   `json:"embeddings"`
	VideoFrames     []VideoFrameDTO      `json:"video_frames"`
}

type FileClusterDTO struct {
	ID          int64  `json:"id"`
	ClusterType string `json:"cluster_type"`
	Title       string `json:"title"`
	Status      string `json:"status"`
}

type CurrentAnalysisDTO struct {
	AnalysisType string   `json:"analysis_type"`
	Status       string   `json:"status"`
	Summary      string   `json:"summary"`
	QualityScore *float64 `json:"quality_score,omitempty"`
	QualityTier  string   `json:"quality_tier,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

type FileTagDTO struct {
	Namespace   string   `json:"namespace"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Source      string   `json:"source"`
	Confidence  *float64 `json:"confidence,omitempty"`
}

type ReviewActionDTO struct {
	ActionType string `json:"action_type"`
	Note       string `json:"note"`
	CreatedAt  string `json:"created_at"`
}

type EmbeddingInfoDTO struct {
	EmbeddingType string `json:"embedding_type"`
	Provider      string `json:"provider"`
	ModelName     string `json:"model_name"`
	VectorCount   int64  `json:"vector_count"`
}

type VideoFrameDTO struct {
	TimestampMS int64  `json:"timestamp_ms"`
	FrameRole   string `json:"frame_role"`
	PHash       string `json:"phash"`
}

type FileListRequest struct {
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset,omitempty"`
	Query         string `json:"query,omitempty"`
	MediaType     string `json:"media_type,omitempty"`
	QualityTier   string `json:"quality_tier,omitempty"`
	ReviewAction  string `json:"review_action,omitempty"`
	Status        string `json:"status,omitempty"`
	VolumeID      int64  `json:"volume_id,omitempty"`
	TagNamespace  string `json:"tag_namespace,omitempty"`
	Tag           string `json:"tag,omitempty"`
	ClusterType   string `json:"cluster_type,omitempty"`
	ClusterStatus string `json:"cluster_status,omitempty"`
	Sort          string `json:"sort,omitempty"`
}

type FileListProvider interface {
	ListFiles(ctx context.Context, input FileListRequest) ([]FileDTO, error)
}

type FileDetailProvider interface {
	GetFileDetail(ctx context.Context, fileID int64) (FileDetailDTO, error)
}

type FileContent struct {
	AbsPath     string
	FileName    string
	ContentType string
	UpdatedAt   time.Time
}

type FileContentProvider interface {
	GetFileContent(ctx context.Context, fileID int64) (FileContent, error)
	GetFilePreview(ctx context.Context, fileID int64) (FileContent, error)
	GetVideoFramePreview(ctx context.Context, fileID int64, frameIndex int) (FileContent, error)
}

type FileTrasher interface {
	TrashFile(ctx context.Context, fileID int64) error
}

type FileRevealer interface {
	RevealFile(ctx context.Context, fileID int64) error
}

type FileReviewActionRequest struct {
	ActionType string `json:"action_type"`
	Note       string `json:"note,omitempty"`
}

type FileTagCreateRequest struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
}

type FileTagDeleteRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type FileReviewer interface {
	ApplyFileReviewAction(ctx context.Context, fileID int64, input FileReviewActionRequest) error
}

type FileTagCreator interface {
	CreateFileTag(ctx context.Context, fileID int64, input FileTagCreateRequest) error
	DeleteFileTag(ctx context.Context, fileID int64, input FileTagDeleteRequest) error
}

type FileRecomputeRequest struct {
	MediaType string `json:"media_type"`
}

type FileJobRunner interface {
	RecomputeFileEmbeddings(ctx context.Context, fileID int64, input FileRecomputeRequest) error
	ReclusterFile(ctx context.Context, fileID int64, input FileRecomputeRequest) error
}

type Dependencies struct {
	SystemStatusProvider   SystemStatusProvider
	TaskSummaryProvider    TaskSummaryProvider
	JobListProvider        JobListProvider
	JobCreator             JobCreator
	JobRetrier             JobRetrier
	JobEventListProvider   JobEventListProvider
	VolumeListProvider     VolumeListProvider
	VolumeCreator          VolumeCreator
	VolumeScanner          VolumeScanner
	TagListProvider        TagListProvider
	ClusterListProvider    ClusterListProvider
	ClusterDetailProvider  ClusterDetailProvider
	ClusterSummaryProvider ClusterSummaryProvider
	ClusterReviewer        ClusterReviewer
	FileListProvider       FileListProvider
	FileDetailProvider     FileDetailProvider
	FileContentProvider    FileContentProvider
	FileTrasher            FileTrasher
	FileRevealer           FileRevealer
	FileReviewer           FileReviewer
	FileTagCreator         FileTagCreator
	FileJobRunner          FileJobRunner
}

func NewMux(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler()))
	mux.HandleFunc("/healthz", okHandler)
	mux.HandleFunc("/readyz", readyHandler(deps.SystemStatusProvider))
	mux.HandleFunc("/api/system-status", systemStatusHandler(deps.SystemStatusProvider))
	mux.HandleFunc("/api/task-summary", taskSummaryHandler(deps.TaskSummaryProvider))
	mux.HandleFunc("/api/jobs", jobsHandler(deps.JobListProvider, deps.JobCreator))
	mux.HandleFunc("/api/jobs/", jobActionHandler(deps.JobRetrier, deps.JobEventListProvider))
	mux.HandleFunc("/api/volumes", volumesHandler(deps.VolumeListProvider, deps.VolumeCreator))
	mux.HandleFunc("/api/volumes/", volumeActionHandler(deps.VolumeScanner))
	mux.HandleFunc("/api/tags", tagsHandler(deps.TagListProvider))
	mux.HandleFunc("/api/cluster-summary", clusterSummaryHandler(deps.ClusterSummaryProvider))
	mux.HandleFunc("/api/clusters", clustersHandler(deps.ClusterListProvider))
	mux.HandleFunc("/api/clusters/", clusterActionHandler(deps.ClusterDetailProvider, deps.ClusterReviewer))
	mux.HandleFunc("/api/files", filesHandler(deps.FileListProvider))
	mux.HandleFunc("/api/files/", fileActionHandler(deps.FileDetailProvider, deps.FileContentProvider, deps.FileTrasher, deps.FileRevealer, deps.FileReviewer, deps.FileTagCreator, deps.FileJobRunner))
	return mux
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func readyHandler(provider SystemStatusProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		snapshot := systemstatus.Snapshot{
			Status: "ready",
			Checks: []systemstatus.Check{},
		}
		if provider != nil {
			result, err := provider.SystemStatus(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			snapshot = result
		}

		statusCode := http.StatusOK
		if snapshot.Status != "ready" {
			statusCode = http.StatusServiceUnavailable
		}
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(snapshot)
	}
}

func systemStatusHandler(provider SystemStatusProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		snapshot := systemstatus.Snapshot{
			Status: "ready",
			Checks: []systemstatus.Check{},
		}
		if provider != nil {
			result, err := provider.SystemStatus(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			snapshot = result
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(snapshot)
	}
}

func taskSummaryHandler(provider TaskSummaryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		summary := tasks.Summary{}
		if provider != nil {
			result, err := provider.TaskSummary(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			summary = result
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(summary)
	}
}

func indexHandler(w http.ResponseWriter, _ *http.Request) {
	body, err := webAssets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func jobsHandler(provider JobListProvider, creator JobCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")

			items := []tasks.Job{}
			if provider != nil {
				options := tasks.JobListOptions{
					Status: r.URL.Query().Get("status"),
					Limit:  parseLimit(r.URL.Query().Get("limit")),
				}
				result, err := provider.ListJobs(r.Context(), options)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				items = result
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(items)
		case http.MethodPost:
			if creator == nil {
				http.Error(w, "job creation unavailable", http.StatusNotImplemented)
				return
			}

			var input tasks.CreateJobInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload"})
				return
			}

			item, err := creator.CreateJob(r.Context(), input)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(item)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var contentType string
		switch r.URL.Path {
		case "app.css":
			contentType = "text/css; charset=utf-8"
		case "app.js":
			contentType = "application/javascript; charset=utf-8"
		default:
			http.NotFound(w, r)
			return
		}

		body, err := webAssets.ReadFile("assets/" + r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}

func volumesHandler(provider VolumeListProvider, creator VolumeCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			items := []VolumeDTO{}
			if provider != nil {
				result, err := provider.ListVolumes(r.Context())
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				items = result
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(items)
		case http.MethodPost:
			if creator == nil {
				http.Error(w, "volume creation unavailable", http.StatusNotImplemented)
				return
			}

			var input CreateVolumeRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload"})
				return
			}

			item, err := creator.CreateVolume(r.Context(), input)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(item)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func volumeActionHandler(scanner VolumeScanner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/scan") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if scanner == nil {
			http.Error(w, "volume scan unavailable", http.StatusNotImplemented)
			return
		}
		volumeID, ok := parseVolumeScanPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		item, err := scanner.EnqueueVolumeScan(r.Context(), volumeID)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(item)
	}
}

func tagsHandler(provider TagListProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		items := []TagDTO{}
		if provider != nil {
			result, err := provider.ListTags(r.Context(), TagListRequest{
				Namespace: strings.TrimSpace(r.URL.Query().Get("namespace")),
				Limit:     parseLimit(r.URL.Query().Get("limit")),
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			items = result
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(items)
	}
}

func clustersHandler(provider ClusterListProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		items := []ClusterDTO{}
		if provider != nil {
			result, err := provider.ListClusters(r.Context(), ClusterListRequest{
				ClusterType: strings.TrimSpace(r.URL.Query().Get("cluster_type")),
				Status:      strings.TrimSpace(r.URL.Query().Get("status")),
				Limit:       parseLimit(r.URL.Query().Get("limit")),
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			items = result
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(items)
	}
}

func clusterSummaryHandler(provider ClusterSummaryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		items := []ClusterSummaryDTO{}
		if provider != nil {
			result, err := provider.SummarizeClusters(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			items = result
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(items)
	}
}

func clusterActionHandler(provider ClusterDetailProvider, reviewer ClusterReviewer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/review-actions") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if reviewer == nil {
				http.Error(w, "cluster reviewer unavailable", http.StatusNotImplemented)
				return
			}
			clusterID, ok := parseClusterReviewActionsPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			var input FileReviewActionRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload"})
				return
			}
			if err := reviewer.ApplyClusterReviewAction(r.Context(), clusterID, input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/status") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if reviewer == nil {
				http.Error(w, "cluster reviewer unavailable", http.StatusNotImplemented)
				return
			}
			clusterID, ok := parseClusterStatusPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			var input struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload"})
				return
			}
			if err := reviewer.UpdateClusterStatus(r.Context(), clusterID, input.Status); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if provider == nil {
			http.Error(w, "cluster detail unavailable", http.StatusNotImplemented)
			return
		}
		clusterID, ok := parseClusterDetailPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		item, err := provider.GetClusterDetail(r.Context(), clusterID)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(item)
	}
}

func filesHandler(provider FileListProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		items := []FileDTO{}
		if provider != nil {
			result, err := provider.ListFiles(r.Context(), FileListRequest{
				Limit:         parseLimit(r.URL.Query().Get("limit")),
				Offset:        parseOffset(r.URL.Query().Get("offset")),
				Query:         strings.TrimSpace(r.URL.Query().Get("q")),
				MediaType:     strings.TrimSpace(r.URL.Query().Get("media_type")),
				QualityTier:   strings.TrimSpace(r.URL.Query().Get("quality_tier")),
				ReviewAction:  strings.TrimSpace(r.URL.Query().Get("review_action")),
				Status:        strings.TrimSpace(r.URL.Query().Get("status")),
				VolumeID:      parseInt64(r.URL.Query().Get("volume_id")),
				TagNamespace:  strings.TrimSpace(r.URL.Query().Get("tag_namespace")),
				Tag:           strings.TrimSpace(r.URL.Query().Get("tag")),
				ClusterType:   strings.TrimSpace(r.URL.Query().Get("cluster_type")),
				ClusterStatus: strings.TrimSpace(r.URL.Query().Get("cluster_status")),
				Sort:          strings.TrimSpace(r.URL.Query().Get("sort")),
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			items = result
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(items)
	}
}

func fileActionHandler(detailProvider FileDetailProvider, contentProvider FileContentProvider, fileTrasher FileTrasher, fileRevealer FileRevealer, fileReviewer FileReviewer, fileTagCreator FileTagCreator, fileJobRunner FileJobRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/frames/") && strings.HasSuffix(r.URL.Path, "/preview") {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if contentProvider == nil {
				http.Error(w, "file content unavailable", http.StatusNotImplemented)
				return
			}
			fileID, frameIndex, ok := parseFileFramePreviewPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			item, err := contentProvider.GetVideoFramePreview(r.Context(), fileID, frameIndex)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			file, err := os.Open(item.AbsPath)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			defer file.Close()
			contentType := item.ContentType
			if contentType == "" {
				buffer := make([]byte, 512)
				n, _ := file.Read(buffer)
				contentType = http.DetectContentType(buffer[:n])
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
			}
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, file)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/content") || strings.HasSuffix(r.URL.Path, "/preview") {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if contentProvider == nil {
				http.Error(w, "file content unavailable", http.StatusNotImplemented)
				return
			}
			fileID, ok := parseFileContentPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			var item FileContent
			var err error
			if strings.HasSuffix(r.URL.Path, "/preview") {
				item, err = contentProvider.GetFilePreview(r.Context(), fileID)
			} else {
				item, err = contentProvider.GetFileContent(r.Context(), fileID)
			}
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			file, err := os.Open(item.AbsPath)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			defer file.Close()
			contentType := item.ContentType
			if contentType == "" {
				buffer := make([]byte, 512)
				n, _ := file.Read(buffer)
				contentType = http.DetectContentType(buffer[:n])
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
			}
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			http.ServeContent(w, r, item.FileName, item.UpdatedAt, file)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/tags") {
			if r.Method != http.MethodPost && r.Method != http.MethodDelete {
				w.Header().Set("Allow", "POST, DELETE")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if fileTagCreator == nil {
				http.Error(w, "file tag creation unavailable", http.StatusNotImplemented)
				return
			}
			fileID, ok := parseFileTagsPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if r.Method == http.MethodPost {
				var input FileTagCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload"})
					return
				}
				if strings.TrimSpace(input.Namespace) == "" || strings.TrimSpace(input.Name) == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "namespace and name are required"})
					return
				}
				if err := fileTagCreator.CreateFileTag(r.Context(), fileID, input); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				w.WriteHeader(http.StatusCreated)
				return
			}
			input := FileTagDeleteRequest{
				Namespace: strings.TrimSpace(r.URL.Query().Get("namespace")),
				Name:      strings.TrimSpace(r.URL.Query().Get("name")),
			}
			if input.Namespace == "" || input.Name == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "namespace and name are required"})
				return
			}
			if err := fileTagCreator.DeleteFileTag(r.Context(), fileID, input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/review-actions") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if fileReviewer == nil {
				http.Error(w, "file review unavailable", http.StatusNotImplemented)
				return
			}
			fileID, ok := parseFileReviewActionsPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			var input FileReviewActionRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload"})
				return
			}
			if err := fileReviewer.ApplyFileReviewAction(r.Context(), fileID, input); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/trash") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if fileTrasher == nil {
				http.Error(w, "file trash unavailable", http.StatusNotImplemented)
				return
			}
			fileID, ok := parseFileTrashPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if err := fileTrasher.TrashFile(r.Context(), fileID); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/reveal") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if fileRevealer == nil {
				http.Error(w, "file reveal unavailable", http.StatusNotImplemented)
				return
			}
			fileID, ok := parseFileRevealPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if err := fileRevealer.RevealFile(r.Context(), fileID); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/recompute-embeddings") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if fileJobRunner == nil {
				http.Error(w, "file recompute unavailable", http.StatusNotImplemented)
				return
			}
			fileID, ok := parseFileRecomputeEmbeddingsPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if detailProvider == nil {
				http.Error(w, "file detail unavailable", http.StatusNotImplemented)
				return
			}
			item, err := detailProvider.GetFileDetail(r.Context(), fileID)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			if err := fileJobRunner.RecomputeFileEmbeddings(r.Context(), fileID, FileRecomputeRequest{
				MediaType: item.MediaType,
			}); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/recluster") {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if fileJobRunner == nil {
				http.Error(w, "file recluster unavailable", http.StatusNotImplemented)
				return
			}
			fileID, ok := parseFileReclusterPath(r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if detailProvider == nil {
				http.Error(w, "file detail unavailable", http.StatusNotImplemented)
				return
			}
			item, err := detailProvider.GetFileDetail(r.Context(), fileID)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			if err := fileJobRunner.ReclusterFile(r.Context(), fileID, FileRecomputeRequest{
				MediaType: item.MediaType,
			}); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if detailProvider == nil {
			http.Error(w, "file detail unavailable", http.StatusNotImplemented)
			return
		}
		fileID, ok := parseFileDetailPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		item, err := detailProvider.GetFileDetail(r.Context(), fileID)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(item)
	}
}

func jobActionHandler(retrier JobRetrier, eventProvider JobEventListProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if jobID, ok := parseJobEventsPath(r.URL.Path); ok {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if eventProvider == nil {
				http.Error(w, "job events unavailable", http.StatusNotImplemented)
				return
			}

			items, err := eventProvider.ListJobEvents(r.Context(), jobID, parseLimit(r.URL.Query().Get("limit")))
			w.Header().Set("Content-Type", "application/json")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(items)
			return
		}

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if retrier == nil {
			http.Error(w, "job retry unavailable", http.StatusNotImplemented)
			return
		}

		jobID, ok := parseRetryPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		item, err := retrier.RetryJob(r.Context(), jobID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			if errors.Is(err, tasks.ErrJobNotRetryable) {
				w.WriteHeader(http.StatusConflict)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(item)
	}
}

func parseLimit(raw string) int {
	if raw == "" {
		return 20
	}
	var value int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 20
		}
		value = value*10 + int(ch-'0')
	}
	if value <= 0 {
		return 20
	}
	if value > 100 {
		return 100
	}
	return value
}

func parseInt64(raw string) int64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func parseOffset(raw string) int {
	if raw == "" {
		return 0
	}
	var value int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0
		}
		value = value*10 + int(ch-'0')
	}
	if value < 0 {
		return 0
	}
	return value
}

func parseRetryPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/jobs/") || !strings.HasSuffix(path, "/retry") {
		return 0, false
	}

	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/jobs/"), "/retry")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" {
		return 0, false
	}

	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseJobEventsPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/jobs/") || !strings.HasSuffix(path, "/events") {
		return 0, false
	}

	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/jobs/"), "/events")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" {
		return 0, false
	}

	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileDetailPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") {
		return 0, false
	}
	idPart := strings.TrimPrefix(path, "/api/files/")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileTrashPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") || !strings.HasSuffix(path, "/trash") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/files/"), "/trash")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileRevealPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") || !strings.HasSuffix(path, "/reveal") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/files/"), "/reveal")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileRecomputeEmbeddingsPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") || !strings.HasSuffix(path, "/recompute-embeddings") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/files/"), "/recompute-embeddings")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileReclusterPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") || !strings.HasSuffix(path, "/recluster") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/files/"), "/recluster")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileTagsPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") || !strings.HasSuffix(path, "/tags") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/files/"), "/tags")
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileContentPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") || (!strings.HasSuffix(path, "/content") && !strings.HasSuffix(path, "/preview")) {
		return 0, false
	}
	idPart := strings.TrimPrefix(path, "/api/files/")
	idPart = strings.TrimSuffix(idPart, "/content")
	idPart = strings.TrimSuffix(idPart, "/preview")
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileFramePreviewPath(path string) (int64, int, bool) {
	if !strings.HasPrefix(path, "/api/files/") || !strings.HasSuffix(path, "/preview") || !strings.Contains(path, "/frames/") {
		return 0, 0, false
	}
	trimmed := strings.TrimPrefix(path, "/api/files/")
	trimmed = strings.TrimSuffix(trimmed, "/preview")
	parts := strings.Split(trimmed, "/frames/")
	if len(parts) != 2 {
		return 0, 0, false
	}
	fileID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || fileID <= 0 {
		return 0, 0, false
	}
	frameIndex, err := strconv.Atoi(parts[1])
	if err != nil || frameIndex < 0 {
		return 0, 0, false
	}
	return fileID, frameIndex, true
}

func parseVolumeScanPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/volumes/") || !strings.HasSuffix(path, "/scan") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/volumes/"), "/scan")
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseClusterDetailPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/clusters/") {
		return 0, false
	}
	idPart := strings.TrimPrefix(path, "/api/clusters/")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseClusterReviewActionsPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/clusters/") || !strings.HasSuffix(path, "/review-actions") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/clusters/"), "/review-actions")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseClusterStatusPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/clusters/") || !strings.HasSuffix(path, "/status") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/clusters/"), "/status")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func parseFileReviewActionsPath(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/api/files/") || !strings.HasSuffix(path, "/review-actions") {
		return 0, false
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/files/"), "/review-actions")
	idPart = strings.TrimSuffix(idPart, "/")
	if idPart == "" || strings.Contains(idPart, "/") {
		return 0, false
	}
	value, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
