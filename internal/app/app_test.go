package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"idea/internal/app"
	"idea/internal/config"
	"idea/internal/httpserver"
	"idea/internal/systemstatus"
	"idea/internal/tasks"
)

func TestNewProvidesHealthAndReadinessRoutes(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:        ":8080",
		DatabaseURL:     "postgres://localhost:5432/idea?sslmode=disable",
		DefaultProvider: "ollama",
		MigrationsDir:   "db/migrations",
		WorkerCommand:   "python3",
		WorkerScript:    "worker/main.py",
	}

	application := app.New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /healthz to return 200, got %d", rec.Code)
	}
	var healthPayload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &healthPayload); err != nil {
		t.Fatalf("expected json body for /healthz: %v", err)
	}
	if healthPayload["status"] != "ok" {
		t.Fatalf("expected status ok for /healthz, got %#v", healthPayload["status"])
	}

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec = httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /readyz to return 200, got %d", rec.Code)
	}
	var readyPayload systemstatus.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &readyPayload); err != nil {
		t.Fatalf("expected json body for /readyz: %v", err)
	}
	if readyPayload.Status != "ready" {
		t.Fatalf("expected status ready for /readyz, got %#v", readyPayload.Status)
	}
}

func TestBootstrapDoesNotRunMigrationsWhenDisabled(t *testing.T) {
	cfg := config.Config{
		RunMigrations: false,
	}
	migrator := &recordingMigrator{}
	application := app.New(cfg, migrator)

	if err := application.Bootstrap(context.Background()); err != nil {
		t.Fatalf("expected bootstrap to succeed: %v", err)
	}
	if migrator.calls != 0 {
		t.Fatalf("expected migrator not to be called, got %d calls", migrator.calls)
	}
}

func TestBootstrapRunsMigrationsWhenEnabled(t *testing.T) {
	cfg := config.Config{
		RunMigrations: true,
		MigrationsDir: "db/migrations",
	}
	migrator := &recordingMigrator{}
	application := app.New(cfg, migrator)

	if err := application.Bootstrap(context.Background()); err != nil {
		t.Fatalf("expected bootstrap to succeed: %v", err)
	}
	if migrator.calls != 1 {
		t.Fatalf("expected migrator to be called once, got %d calls", migrator.calls)
	}
	if migrator.dir != "db/migrations" {
		t.Fatalf("expected migrator dir %q, got %q", "db/migrations", migrator.dir)
	}
}

func TestBootstrapChecksWorkerHealthWithoutFailingStartup(t *testing.T) {
	cfg := config.Config{}
	application := app.New(cfg, nil)
	checker := &recordingWorkerChecker{err: errors.New("worker unavailable")}
	application.SetWorkerChecker(checker)

	if err := application.Bootstrap(context.Background()); err != nil {
		t.Fatalf("expected bootstrap to keep succeeding when worker health check fails: %v", err)
	}
	if checker.calls != 1 {
		t.Fatalf("expected worker health check to be called once, got %d", checker.calls)
	}
}

func TestSetTaskSummaryProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetTaskSummaryProvider(staticTaskSummaryProvider{
		summary: tasks.Summary{
			Pending: 2,
			Running: 1,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/task-summary", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload tasks.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json body: %v", err)
	}
	if payload.Pending != 2 || payload.Running != 1 {
		t.Fatalf("unexpected summary: %#v", payload)
	}
}

func TestSetSystemStatusProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetSystemStatusProvider(staticSystemStatusProvider{
		snapshot: systemstatus.Snapshot{
			Status: "ready",
			Checks: []systemstatus.Check{
				{Name: "database", Status: "ready"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/system-status", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload systemstatus.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json body: %v", err)
	}
	if payload.Status != "ready" || len(payload.Checks) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestSetJobListProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetJobListProvider(staticJobListProvider{
		items: []tasks.Job{
			{ID: 9, JobType: "scan_volume", Status: "pending"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload []tasks.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json body: %v", err)
	}
	if len(payload) != 1 || payload[0].ID != 9 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestSetJobCreatorUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetJobCreator(staticJobCreator{
		item: tasks.Job{ID: 10, JobType: "scan_volume", Status: "pending"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(`{"job_type":"scan_volume"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestSetJobRetrierUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetJobRetrier(staticJobRetrier{
		item: tasks.Job{ID: 7, JobType: "infer_tags", Status: "pending"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/7/retry", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSetFileJobRunnerUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileDetailProvider(staticFileDetailProvider{
		item: httpserver.FileDetailDTO{
			FileDTO: httpserver.FileDTO{ID: 7, MediaType: "image"},
		},
	})
	application.SetFileJobRunner(staticFileJobRunner{})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/recompute-embeddings", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestSetJobEventListProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetJobEventListProvider(staticJobEventListProvider{
		items: []tasks.JobEvent{{ID: 1, JobID: 7, Level: "info", Message: "job created"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/7/events", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSetVolumeListProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetVolumeListProvider(staticVolumeListProvider{
		items: []httpserver.VolumeDTO{{ID: 7, DisplayName: "Media", MountPath: "/Volumes/media", IsOnline: true}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSetVolumeCreatorUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetVolumeCreator(staticVolumeCreator{
		item: httpserver.VolumeDTO{ID: 8, DisplayName: "Disk 2", MountPath: "/Volumes/disk2", IsOnline: true},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/volumes", strings.NewReader(`{"display_name":"Disk 2","mount_path":"/Volumes/disk2"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestSetVolumeScannerUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetVolumeScanner(staticVolumeScanner{})

	req := httptest.NewRequest(http.MethodPost, "/api/volumes/7/scan", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestSetClusterSummaryProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetClusterSummaryProvider(staticClusterSummaryProvider{
		items: []httpserver.ClusterSummaryDTO{
			{ClusterType: "same_content", Status: "candidate", ClusterCount: 4, MemberCount: 9},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cluster-summary", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload []httpserver.ClusterSummaryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json body: %v", err)
	}
	if len(payload) != 1 || payload[0].ClusterCount != 4 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestSetClusterReviewerUpdatesStatusEndpoint(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetClusterReviewer(&staticClusterReviewer{})

	req := httptest.NewRequest(http.MethodPost, "/api/clusters/7/status", strings.NewReader(`{"status":"confirmed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSetFileListProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileListProvider(staticFileListProvider{
		items: []httpserver.FileDTO{{ID: 1, FileName: "photo.jpg", MediaType: "image", Status: "active"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSetFileDetailProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileDetailProvider(staticFileDetailProvider{
		item: httpserver.FileDetailDTO{
			FileDTO: httpserver.FileDTO{ID: 7, FileName: "photo.jpg", MediaType: "image", Status: "active"},
			PathHistory: []httpserver.PathHistoryDTO{
				{AbsPath: "/Volumes/media/photo.jpg", EventType: "discovered", SeenAt: "2026-04-09T20:00:00Z"},
			},
			CurrentAnalyses: []httpserver.CurrentAnalysisDTO{
				{AnalysisType: "search_doc", Status: "succeeded", Summary: "photo jpg", CreatedAt: "2026-04-09T20:01:00Z"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files/7", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSetTagListProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetTagListProvider(staticTagListProvider{
		items: []httpserver.TagDTO{{Namespace: "content", Name: "单人写真", DisplayName: "单人写真", FileCount: 12}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSetFileTrasherUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileTrasher(staticFileTrashProvider{})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/trash", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestSetFileRevealerUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileRevealer(staticFileRevealProvider{})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/reveal", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestSetFileOpenerUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileOpener(staticFileOpenProvider{})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/open", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestSetFileTagCreatorUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileTagCreator(staticFileTagCreator{})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/tags", strings.NewReader(`{"namespace":"person","name":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestSetFileContentProviderUpdatesHandlerDependencies(t *testing.T) {
	application := app.New(config.Config{}, nil)
	application.SetFileContentProvider(staticFileContentProvider{
		item: httpserver.FileContent{
			AbsPath:     "/tmp/example.jpg",
			FileName:    "example.jpg",
			ContentType: "image/jpeg",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files/7/content", nil)
	rec := httptest.NewRecorder()

	application.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusNotImplemented {
		t.Fatalf("expected file content provider to be wired")
	}
}

type recordingMigrator struct {
	calls int
	dir   string
}

func (m *recordingMigrator) Run(_ context.Context, dir string) error {
	m.calls++
	m.dir = dir
	return nil
}

type recordingWorkerChecker struct {
	calls int
	err   error
}

func (w *recordingWorkerChecker) CheckWorkerHealth(_ context.Context) error {
	w.calls++
	return w.err
}

type staticTaskSummaryProvider struct {
	summary tasks.Summary
}

func (s staticTaskSummaryProvider) TaskSummary(_ context.Context) (tasks.Summary, error) {
	return s.summary, nil
}

type staticSystemStatusProvider struct {
	snapshot systemstatus.Snapshot
}

func (s staticSystemStatusProvider) SystemStatus(_ context.Context) (systemstatus.Snapshot, error) {
	return s.snapshot, nil
}

type staticJobListProvider struct {
	items []tasks.Job
}

func (s staticJobListProvider) ListJobs(_ context.Context, _ tasks.JobListOptions) ([]tasks.Job, error) {
	return s.items, nil
}

type staticJobCreator struct {
	item tasks.Job
}

func (s staticJobCreator) CreateJob(_ context.Context, _ tasks.CreateJobInput) (tasks.Job, error) {
	return s.item, nil
}

type staticJobRetrier struct {
	item tasks.Job
}

func (s staticJobRetrier) RetryJob(_ context.Context, _ int64) (tasks.Job, error) {
	return s.item, nil
}

type staticJobEventListProvider struct {
	items []tasks.JobEvent
}

func (s staticJobEventListProvider) ListJobEvents(_ context.Context, _ int64, _ int) ([]tasks.JobEvent, error) {
	return s.items, nil
}

type staticVolumeListProvider struct {
	items []httpserver.VolumeDTO
}

func (s staticVolumeListProvider) ListVolumes(_ context.Context) ([]httpserver.VolumeDTO, error) {
	return s.items, nil
}

type staticVolumeCreator struct {
	item httpserver.VolumeDTO
}

func (s staticVolumeCreator) CreateVolume(_ context.Context, _ httpserver.CreateVolumeRequest) (httpserver.VolumeDTO, error) {
	return s.item, nil
}

type staticVolumeScanner struct{}

func (staticVolumeScanner) EnqueueVolumeScan(_ context.Context, volumeID int64) (tasks.Job, error) {
	return tasks.Job{ID: 1, JobType: "scan_volume", Status: "pending", TargetType: "volume", TargetID: volumeID}, nil
}

type staticFileListProvider struct {
	items []httpserver.FileDTO
}

func (s staticFileListProvider) ListFiles(_ context.Context, _ httpserver.FileListRequest) (httpserver.FileListResponse, error) {
	return httpserver.FileListResponse{Items: s.items}, nil
}

type staticFileDetailProvider struct {
	item httpserver.FileDetailDTO
}

func (s staticFileDetailProvider) GetFileDetail(_ context.Context, _ int64) (httpserver.FileDetailDTO, error) {
	return s.item, nil
}

type staticFileContentProvider struct {
	item httpserver.FileContent
}

func (s staticFileContentProvider) GetFileContent(_ context.Context, _ int64) (httpserver.FileContent, error) {
	return s.item, nil
}

func (s staticFileContentProvider) GetFilePreview(_ context.Context, _ int64) (httpserver.FileContent, error) {
	return s.item, nil
}

func (s staticFileContentProvider) GetVideoFramePreview(_ context.Context, _ int64, _ int) (httpserver.FileContent, error) {
	return s.item, nil
}

type staticTagListProvider struct {
	items []httpserver.TagDTO
}

func (s staticTagListProvider) ListTags(_ context.Context, _ httpserver.TagListRequest) ([]httpserver.TagDTO, error) {
	return s.items, nil
}

type staticClusterSummaryProvider struct {
	items []httpserver.ClusterSummaryDTO
}

func (s staticClusterSummaryProvider) SummarizeClusters(_ context.Context) ([]httpserver.ClusterSummaryDTO, error) {
	return s.items, nil
}

type staticFileTrashProvider struct{}

func (staticFileTrashProvider) TrashFile(_ context.Context, _ int64) error {
	return nil
}

type staticFileRevealProvider struct{}

func (staticFileRevealProvider) RevealFile(_ context.Context, _ int64) error {
	return nil
}

type staticFileOpenProvider struct{}

func (staticFileOpenProvider) OpenFile(_ context.Context, _ int64) error {
	return nil
}

type staticFileTagCreator struct{}

func (staticFileTagCreator) CreateFileTag(_ context.Context, _ int64, _ httpserver.FileTagCreateRequest) error {
	return nil
}

func (staticFileTagCreator) DeleteFileTag(_ context.Context, _ int64, _ httpserver.FileTagDeleteRequest) error {
	return nil
}

type staticFileJobRunner struct{}

func (staticFileJobRunner) RecomputeFileEmbeddings(_ context.Context, _ int64, _ httpserver.FileRecomputeRequest) error {
	return nil
}

func (staticFileJobRunner) ReclusterFile(_ context.Context, _ int64, _ httpserver.FileRecomputeRequest) error {
	return nil
}

type staticClusterReviewer struct{}

func (*staticClusterReviewer) ApplyClusterReviewAction(_ context.Context, _ int64, _ httpserver.FileReviewActionRequest) error {
	return nil
}

func (*staticClusterReviewer) UpdateClusterStatus(_ context.Context, _ int64, _ string) error {
	return nil
}
