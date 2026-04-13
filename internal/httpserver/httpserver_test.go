package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"idea/internal/httpserver"
	"idea/internal/systemstatus"
	"idea/internal/tasks"
)

func TestTaskSummaryEndpointReturnsProviderSummary(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		TaskSummaryProvider: staticTaskSummaryProvider{
			summary: tasks.Summary{
				Pending:   1,
				Running:   2,
				Failed:    1,
				Dead:      1,
				Succeeded: 3,
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/task-summary", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload tasks.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.Running != 2 {
		t.Fatalf("expected running 2, got %d", payload.Running)
	}
}

func TestReadyzEndpointReturnsReadyStatus(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		SystemStatusProvider: staticSystemStatusProvider{
			snapshot: systemstatus.Snapshot{
				Status: "ready",
				Checks: []systemstatus.Check{
					{Name: "database", Status: "ready"},
					{Name: "worker", Status: "ready"},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload systemstatus.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.Status != "ready" || len(payload.Checks) != 2 {
		t.Fatalf("unexpected ready payload: %#v", payload)
	}
}

func TestReadyzEndpointReturns503WhenSystemNotReady(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		SystemStatusProvider: staticSystemStatusProvider{
			snapshot: systemstatus.Snapshot{
				Status: "not_ready",
				Checks: []systemstatus.Check{
					{Name: "database", Status: "ready"},
					{Name: "worker", Status: "not_ready", Message: "worker unavailable"},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var payload systemstatus.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.Status != "not_ready" {
		t.Fatalf("unexpected ready payload: %#v", payload)
	}
}

func TestSystemStatusEndpointReturnsProviderSnapshot(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		SystemStatusProvider: staticSystemStatusProvider{
			snapshot: systemstatus.Snapshot{
				Status: "ready",
				Checks: []systemstatus.Check{
					{Name: "database", Status: "disabled", Message: "database disabled"},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/system-status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload systemstatus.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.Status != "ready" || len(payload.Checks) != 1 || payload.Checks[0].Name != "database" {
		t.Fatalf("unexpected system status payload: %#v", payload)
	}
}

func TestDirectoryPickerEndpointReturnsPath(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		DirectoryPicker: staticDirectoryPicker{
			path: "/Volumes/media",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/system/pick-directory", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload["path"] != "/Volumes/media" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestRootPageReturnsDashboardShell(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Fatal("expected html body")
	}
	if !contains(body, "Local Media Governance") {
		t.Fatalf("expected dashboard title in response body, got %q", body)
	}
	if !contains(body, "Job Timeline") {
		t.Fatalf("expected job timeline section in response body, got %q", body)
	}
	if !contains(body, "System Status") {
		t.Fatalf("expected system status section in response body, got %q", body)
	}
	if !contains(body, "Volumes") {
		t.Fatalf("expected volumes section in response body, got %q", body)
	}
	if !contains(body, "Recent Files") {
		t.Fatalf("expected recent files section in response body, got %q", body)
	}
	if !contains(body, "Top Tags") {
		t.Fatalf("expected tags section in response body, got %q", body)
	}
}

func TestRootPagePrefersReactDistIndexWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/index.html", []byte("<!doctype html><html><body><div id=\"root\">react app</div></body></html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	mux := httpserver.NewMux(httpserver.Dependencies{FrontendDistDir: dir})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !contains(rec.Body.String(), "react app") {
		t.Fatalf("expected react dist index, got %q", rec.Body.String())
	}
}

func TestMuxServesReactDistAssetWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/assets", 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(dir+"/assets/app.js", []byte("console.log('react');"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	mux := httpserver.NewMux(httpserver.Dependencies{FrontendDistDir: dir})

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !contains(rec.Body.String(), "react") {
		t.Fatalf("expected dist asset content, got %q", rec.Body.String())
	}
}

func TestTaskSummaryEndpointReturns500WhenProviderFails(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		TaskSummaryProvider: staticTaskSummaryProvider{
			err: errors.New("db down"),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/task-summary", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestJobsEndpointReturnsProviderJobs(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobListProvider: staticJobListProvider{
			items: []tasks.Job{
				{
					ID:         1,
					JobType:    "scan_volume",
					Status:     "pending",
					TargetType: "volume",
					TargetID:   3,
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs?status=pending&limit=10", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload []tasks.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 job, got %d", len(payload))
	}
	if payload[0].JobType != "scan_volume" {
		t.Fatalf("unexpected job payload: %#v", payload[0])
	}
}

func TestJobsEndpointReturns500WhenProviderFails(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobListProvider: staticJobListProvider{
			err: errors.New("db down"),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestJobsEndpointCreatesJob(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobCreator: staticJobCreator{
			item: tasks.Job{
				ID:         5,
				JobType:    "scan_volume",
				Status:     "pending",
				TargetType: "volume",
				TargetID:   2,
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(`{"job_type":"scan_volume","target_type":"volume","target_id":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var payload tasks.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.ID != 5 || payload.JobType != "scan_volume" {
		t.Fatalf("unexpected created job: %#v", payload)
	}
}

func TestJobsEndpointReturns400ForInvalidPayload(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobCreator: staticJobCreator{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(`{"job_type":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRetryJobEndpointRequeuesJob(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobRetrier: staticJobRetrier{
			item: tasks.Job{ID: 7, JobType: "infer_tags", Status: "pending"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/7/retry", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload tasks.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.ID != 7 || payload.Status != "pending" {
		t.Fatalf("unexpected retried job: %#v", payload)
	}
}

func TestRetryJobEndpointReturns409ForNotRetryableJob(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobRetrier: staticJobRetrier{
			err: tasks.ErrJobNotRetryable,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/7/retry", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestClusterReviewActionEndpointAppliesAction(t *testing.T) {
	reviewer := &staticClusterReviewer{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		ClusterReviewer: reviewer,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/clusters/7/review-actions", strings.NewReader(`{"action_type":"keep","note":"cluster keep"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if reviewer.clusterID != 7 {
		t.Fatalf("expected cluster reviewer to receive 7, got %d", reviewer.clusterID)
	}
	if reviewer.input.ActionType != "keep" || reviewer.input.Note != "cluster keep" {
		t.Fatalf("unexpected cluster review input: %#v", reviewer.input)
	}
}

func TestClusterStatusEndpointUpdatesStatus(t *testing.T) {
	reviewer := &staticClusterReviewer{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		ClusterReviewer: reviewer,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/clusters/7/status", strings.NewReader(`{"status":"confirmed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if reviewer.statusClusterID != 7 || reviewer.status != "confirmed" {
		t.Fatalf("unexpected cluster status update: %#v", reviewer)
	}
}

func TestJobEventsEndpointReturnsEvents(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobEventListProvider: staticJobEventListProvider{
			items: []tasks.JobEvent{
				{ID: 1, JobID: 7, Level: "info", Message: "job created"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/7/events?limit=5", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload []tasks.JobEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if len(payload) != 1 || payload[0].JobID != 7 {
		t.Fatalf("unexpected events payload: %#v", payload)
	}
}

func TestJobEventsEndpointReturns500WhenProviderFails(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		JobEventListProvider: staticJobEventListProvider{
			err: errors.New("db down"),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/7/events", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestVolumesEndpointReturnsProviderVolumes(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		VolumeListProvider: staticVolumeListProvider{
			items: []httpserver.VolumeDTO{
				{ID: 7, DisplayName: "Media", MountPath: "/Volumes/media", IsOnline: true},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestVolumesEndpointCreatesVolume(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		VolumeCreator: staticVolumeCreator{
			item: httpserver.VolumeDTO{ID: 8, DisplayName: "Disk 2", MountPath: "/Volumes/disk2", IsOnline: true},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/volumes", strings.NewReader(`{"display_name":"Disk 2","mount_path":"/Volumes/disk2"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestVolumeScanEndpointInvokesProvider(t *testing.T) {
	provider := &staticVolumeScanner{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		VolumeScanner: provider,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/volumes/7/scan", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if provider.volumeID != 7 {
		t.Fatalf("expected volume scanner to receive 7, got %d", provider.volumeID)
	}
}

func TestFilesEndpointReturnsProviderFiles(t *testing.T) {
	provider := staticFileListProvider{
		items: []httpserver.FileDTO{
			{
				ID:           1,
				FileName:     "photo.jpg",
				MediaType:    "image",
				Status:       "active",
				AbsPath:      "/Volumes/media/photo.jpg",
				Width:        intPtr(320),
				Height:       intPtr(180),
				Format:       "jpg",
				QualityScore: float64Ptr(82),
				QualityTier:  "high",
				ReviewAction: "favorite",
				DurationMS:   nil,
				TagNames:     []string{"单人写真", "室内"},
			},
		},
	}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileListProvider: &provider,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files?limit=10", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload []httpserver.FileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if len(payload) != 1 || payload[0].Width == nil || *payload[0].Width != 320 || payload[0].Format != "jpg" || payload[0].QualityScore == nil || *payload[0].QualityScore != 82 || payload[0].QualityTier != "high" || payload[0].ReviewAction != "favorite" || len(payload[0].TagNames) != 2 {
		t.Fatalf("unexpected files payload: %#v", payload)
	}
	if provider.lastRequest.Query != "" {
		t.Fatalf("expected empty query, got %#v", provider.lastRequest)
	}
}

func TestFilesEndpointPassesSearchQuery(t *testing.T) {
	provider := staticFileListProvider{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileListProvider: &provider,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files?q=poster&limit=5", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if provider.lastRequest.Query != "poster" || provider.lastRequest.Limit != 5 {
		t.Fatalf("unexpected request: %#v", provider.lastRequest)
	}
}

func TestFilesEndpointPassesStructuredFilters(t *testing.T) {
	provider := staticFileListProvider{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileListProvider: &provider,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files?q=poster&media_type=video&quality_tier=high&review_action=favorite&status=missing&volume_id=9&tag_namespace=content&tag=%E5%8D%95%E4%BA%BA%E5%86%99%E7%9C%9F&limit=6", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if provider.lastRequest.Query != "poster" {
		t.Fatalf("unexpected query: %#v", provider.lastRequest)
	}
	if provider.lastRequest.MediaType != "video" || provider.lastRequest.QualityTier != "high" || provider.lastRequest.ReviewAction != "favorite" || provider.lastRequest.Status != "missing" || provider.lastRequest.VolumeID != 9 || provider.lastRequest.TagNamespace != "content" || provider.lastRequest.Tag != "单人写真" || provider.lastRequest.Limit != 6 {
		t.Fatalf("unexpected request: %#v", provider.lastRequest)
	}
}

func TestFilesEndpointPassesClusterFilters(t *testing.T) {
	provider := staticFileListProvider{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileListProvider: &provider,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files?cluster_type=same_series&cluster_status=candidate&limit=6", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if provider.lastRequest.ClusterType != "same_series" || provider.lastRequest.ClusterStatus != "candidate" || provider.lastRequest.Limit != 6 {
		t.Fatalf("unexpected request: %#v", provider.lastRequest)
	}
}

func TestFilesEndpointPassesOffsetAndSort(t *testing.T) {
	provider := staticFileListProvider{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileListProvider: &provider,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files?limit=12&offset=24&sort=size_desc", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if provider.lastRequest.Limit != 12 || provider.lastRequest.Offset != 24 || provider.lastRequest.Sort != "size_desc" {
		t.Fatalf("unexpected request: %#v", provider.lastRequest)
	}
}

func TestTagsEndpointReturnsProviderTags(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		TagListProvider: staticTagListProvider{
			items: []httpserver.TagDTO{
				{Namespace: "content", Name: "单人写真", DisplayName: "单人写真", FileCount: 12},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tags?namespace=content&limit=8", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload []httpserver.TagDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if len(payload) != 1 || payload[0].Name != "单人写真" || payload[0].FileCount != 12 {
		t.Fatalf("unexpected tags payload: %#v", payload)
	}
}

func TestClustersEndpointReturnsProviderClusters(t *testing.T) {
	provider := staticClusterListProvider{
		items: []httpserver.ClusterDTO{
			{ID: 7, ClusterType: "same_person", Title: "Candidate person group", Status: "candidate", MemberCount: 3, StrongMemberCount: 2, TopMemberScore: float64Ptr(0.96), PersonVisualCount: 2, GenericVisualCount: 1, TopEvidenceType: "person_visual"},
		},
	}
	mux := httpserver.NewMux(httpserver.Dependencies{
		ClusterListProvider: &provider,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/clusters?cluster_type=same_person&status=candidate&limit=5", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if provider.lastRequest.ClusterType != "same_person" || provider.lastRequest.Status != "candidate" || provider.lastRequest.Limit != 5 {
		t.Fatalf("unexpected request: %#v", provider.lastRequest)
	}
	var payload []httpserver.ClusterDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if len(payload) != 1 || payload[0].PersonVisualCount != 2 || payload[0].GenericVisualCount != 1 || payload[0].TopEvidenceType != "person_visual" {
		t.Fatalf("expected cluster list evidence fields, got %#v", payload)
	}
}

func TestClusterSummaryEndpointReturnsSummary(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		ClusterSummaryProvider: staticClusterSummaryProvider{
			items: []httpserver.ClusterSummaryDTO{
				{ClusterType: "same_content", Status: "candidate", ClusterCount: 4, MemberCount: 9},
				{ClusterType: "same_series", Status: "candidate", ClusterCount: 2, MemberCount: 5},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cluster-summary", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload []httpserver.ClusterSummaryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if len(payload) != 2 || payload[0].ClusterType != "same_content" || payload[1].MemberCount != 5 {
		t.Fatalf("unexpected cluster summary payload: %#v", payload)
	}
}

func TestClusterDetailEndpointReturnsDetail(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		ClusterDetailProvider: staticClusterDetailProvider{
			item: httpserver.ClusterDetailDTO{
				ClusterDTO: httpserver.ClusterDTO{ID: 7, ClusterType: "same_series", Title: "Series A", Status: "candidate", MemberCount: 2, StrongMemberCount: 1, TopMemberScore: float64Ptr(0.99)},
				PersonVisualCount: 1,
				GenericVisualCount: 0,
				TopEvidenceType: "person_visual",
				Members: []httpserver.ClusterMemberDTO{
					{FileID: 15, FileName: "a.jpg", MediaType: "image", Role: "cover", QualityTier: "high", HasFace: true, SubjectCount: "single", CaptureType: "selfie", EmbeddingType: "person_visual", EmbeddingProvider: "semantic", EmbeddingModel: "semantic-ollama-qwen3-vl-8b-v1", EmbeddingVectorCount: 1},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/clusters/7", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload httpserver.ClusterDetailDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.ID != 7 || len(payload.Members) != 1 || payload.Members[0].Role != "cover" {
		t.Fatalf("unexpected cluster detail payload: %#v", payload)
	}
	if payload.TopMemberScore == nil || *payload.TopMemberScore != 0.99 || payload.StrongMemberCount != 1 {
		t.Fatalf("expected cluster strength fields, got %#v", payload)
	}
	if payload.PersonVisualCount != 1 || payload.GenericVisualCount != 0 || payload.TopEvidenceType != "person_visual" {
		t.Fatalf("expected evidence summary fields, got %#v", payload)
	}
	if !payload.Members[0].HasFace || payload.Members[0].SubjectCount != "single" || payload.Members[0].CaptureType != "selfie" {
		t.Fatalf("expected structured member evidence fields, got %#v", payload.Members[0])
	}
	if payload.Members[0].EmbeddingType != "person_visual" || payload.Members[0].EmbeddingProvider != "semantic" || payload.Members[0].EmbeddingModel == "" {
		t.Fatalf("expected member embedding detail, got %#v", payload.Members[0])
	}
}

func TestFileDetailEndpointReturnsClusterMemberships(t *testing.T) {
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileDetailProvider: staticFileDetailProvider{
			item: httpserver.FileDetailDTO{
				FileDTO: httpserver.FileDTO{
					ID:       7,
					FileName: "poster.jpg",
				},
				Clusters: []httpserver.FileClusterDTO{
					{ID: 31, ClusterType: "same_content", Title: "same_content:abc", Status: "candidate"},
					{ID: 32, ClusterType: "same_series", Title: "same_series:set-a", Status: "candidate"},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files/7", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload httpserver.FileDetailDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if len(payload.Clusters) != 2 || payload.Clusters[1].ClusterType != "same_series" {
		t.Fatalf("expected cluster memberships, got %#v", payload.Clusters)
	}
}

func TestFileDetailEndpointReturnsDetail(t *testing.T) {
	provider := staticFileDetailProvider{
		item: httpserver.FileDetailDTO{
			FileDTO: httpserver.FileDTO{
				ID:         7,
				FileName:   "poster.jpg",
				MediaType:  "video",
				Status:     "active",
				AbsPath:    "/Volumes/media/photos/poster.mp4",
				FPS:        float64Ptr(29.97),
				Bitrate:    int64Ptr(8_000_000),
				VideoCodec: "h264",
				AudioCodec: "aac",
			},
			PathHistory: []httpserver.PathHistoryDTO{
				{AbsPath: "/Volumes/media/photos/poster.jpg", EventType: "discovered", SeenAt: "2026-04-09T20:00:00Z"},
			},
			CurrentAnalyses: []httpserver.CurrentAnalysisDTO{
				{AnalysisType: "quality", Status: "succeeded", Summary: "image quality high, 1920x1080 jpg.", QualityScore: float64Ptr(82), QualityTier: "high", CreatedAt: "2026-04-09T20:01:00Z"},
			},
			Tags: []httpserver.FileTagDTO{
				{Namespace: "content", Name: "单人写真", DisplayName: "单人写真", Source: "ai"},
			},
			ReviewActions: []httpserver.ReviewActionDTO{
				{ActionType: "favorite", Note: "manual favorite", CreatedAt: "2026-04-09T20:02:00Z"},
			},
			Embeddings: []httpserver.EmbeddingInfoDTO{
				{EmbeddingType: "image_visual", Provider: "semantic", ModelName: "semantic-ollama-qwen3-vl-8b-v1", VectorCount: 1},
			},
			VideoFrames: []httpserver.VideoFrameDTO{
				{TimestampMS: 5_000, FrameRole: "understanding", PHash: "frame-a"},
			},
		},
	}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileListProvider:   &staticFileListProvider{},
		FileDetailProvider: provider,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files/7", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload httpserver.FileDetailDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload.ID != 7 || len(payload.PathHistory) != 1 || len(payload.CurrentAnalyses) != 1 || len(payload.Tags) != 1 || len(payload.ReviewActions) != 1 {
		t.Fatalf("unexpected detail payload: %#v", payload)
	}
	if payload.FPS == nil || *payload.FPS != 29.97 || payload.Bitrate == nil || *payload.Bitrate != 8_000_000 || payload.VideoCodec != "h264" || payload.AudioCodec != "aac" {
		t.Fatalf("unexpected video metadata payload: %#v", payload)
	}
	if payload.CurrentAnalyses[0].QualityScore == nil || *payload.CurrentAnalyses[0].QualityScore != 82 || payload.CurrentAnalyses[0].QualityTier != "high" {
		t.Fatalf("unexpected quality analysis payload: %#v", payload.CurrentAnalyses[0])
	}
	if len(payload.Embeddings) != 1 || payload.Embeddings[0].Provider != "semantic" {
		t.Fatalf("unexpected embedding payload: %#v", payload.Embeddings)
	}
	if len(payload.VideoFrames) != 1 || payload.VideoFrames[0].PHash != "frame-a" {
		t.Fatalf("unexpected video frame payload: %#v", payload.VideoFrames)
	}
}

func TestFileReviewActionEndpointInvokesProvider(t *testing.T) {
	provider := &staticFileReviewer{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileReviewer: provider,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/review-actions", strings.NewReader(`{"action_type":"favorite","note":"manual favorite"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if provider.fileID != 7 || provider.input.ActionType != "favorite" || provider.input.Note != "manual favorite" {
		t.Fatalf("unexpected review action request: %#v %#v", provider.fileID, provider.input)
	}
}

func TestTrashFileEndpointInvokesProvider(t *testing.T) {
	provider := &staticFileTrashProvider{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileTrasher: provider,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/trash", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if provider.fileID != 7 {
		t.Fatalf("expected trash provider to receive 7, got %d", provider.fileID)
	}
}

func TestRevealFileEndpointInvokesProvider(t *testing.T) {
	provider := &staticFileRevealProvider{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileRevealer: provider,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/reveal", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if provider.fileID != 7 {
		t.Fatalf("expected reveal provider to receive 7, got %d", provider.fileID)
	}
}

func TestRecomputeEmbeddingsEndpointInvokesProvider(t *testing.T) {
	provider := &staticFileJobRunner{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileDetailProvider: staticFileDetailProvider{
			item: httpserver.FileDetailDTO{
				FileDTO: httpserver.FileDTO{ID: 7, MediaType: "image"},
			},
		},
		FileJobRunner: provider,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/recompute-embeddings", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if provider.embeddingFileID != 7 {
		t.Fatalf("expected recompute embeddings provider to receive 7, got %d", provider.embeddingFileID)
	}
	if provider.embeddingInput.MediaType != "image" {
		t.Fatalf("expected image media type, got %#v", provider.embeddingInput)
	}
}

func TestReclusterFileEndpointInvokesProvider(t *testing.T) {
	provider := &staticFileJobRunner{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileDetailProvider: staticFileDetailProvider{
			item: httpserver.FileDetailDTO{
				FileDTO: httpserver.FileDTO{ID: 7, MediaType: "video"},
			},
		},
		FileJobRunner: provider,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/recluster", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if provider.reclusterFileID != 7 {
		t.Fatalf("expected recluster provider to receive 7, got %d", provider.reclusterFileID)
	}
	if provider.reclusterInput.MediaType != "video" {
		t.Fatalf("expected video media type, got %#v", provider.reclusterInput)
	}
}

func TestCreateFileTagEndpointInvokesProvider(t *testing.T) {
	provider := &staticFileTagCreator{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileTagCreator: provider,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/7/tags", strings.NewReader(`{"namespace":"person","name":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if provider.fileID != 7 {
		t.Fatalf("expected tag creator to receive 7, got %d", provider.fileID)
	}
	if provider.input.Namespace != "person" || provider.input.Name != "alice" {
		t.Fatalf("unexpected file tag input: %#v", provider.input)
	}
}

func TestDeleteFileTagEndpointInvokesProvider(t *testing.T) {
	provider := &staticFileTagCreator{}
	mux := httpserver.NewMux(httpserver.Dependencies{
		FileTagCreator: provider,
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/files/7/tags?namespace=person&name=alice", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if provider.deletedFileID != 7 {
		t.Fatalf("expected tag creator to receive 7, got %d", provider.deletedFileID)
	}
	if provider.deleted.Namespace != "person" || provider.deleted.Name != "alice" {
		t.Fatalf("unexpected delete input: %#v", provider.deleted)
	}
}

func TestFileContentEndpointStreamsProviderContent(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "preview-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer tmp.Close()
	if _, err := tmp.WriteString("hello preview"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mux := httpserver.NewMux(httpserver.Dependencies{
		FileContentProvider: staticFileContentProvider{
			item: httpserver.FileContent{
				AbsPath:     tmp.Name(),
				FileName:    "preview.txt",
				ContentType: "text/plain; charset=utf-8",
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files/7/content", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", got)
	}
	if body := rec.Body.String(); body != "hello preview" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestFileFramePreviewEndpointStreamsProviderContent(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "frame-*.jpg")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer tmp.Close()
	if _, err := tmp.WriteString("frame preview"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mux := httpserver.NewMux(httpserver.Dependencies{
		FileContentProvider: staticFileContentProvider{
			item: httpserver.FileContent{
				AbsPath:     tmp.Name(),
				FileName:    "frame.jpg",
				ContentType: "image/jpeg",
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/files/7/frames/0/preview", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "image/jpeg") {
		t.Fatalf("expected image/jpeg content type, got %q", got)
	}
	if body := rec.Body.String(); body != "frame preview" {
		t.Fatalf("unexpected body: %q", body)
	}
}

type staticTaskSummaryProvider struct {
	summary tasks.Summary
	err     error
}

func (s staticTaskSummaryProvider) TaskSummary(_ context.Context) (tasks.Summary, error) {
	return s.summary, s.err
}

type staticSystemStatusProvider struct {
	snapshot systemstatus.Snapshot
	err      error
}

func (s staticSystemStatusProvider) SystemStatus(_ context.Context) (systemstatus.Snapshot, error) {
	return s.snapshot, s.err
}

type staticDirectoryPicker struct {
	path string
	err  error
}

func (s staticDirectoryPicker) PickDirectory(_ context.Context) (string, error) {
	return s.path, s.err
}

type staticJobListProvider struct {
	items []tasks.Job
	err   error
}

func (s staticJobListProvider) ListJobs(_ context.Context, _ tasks.JobListOptions) ([]tasks.Job, error) {
	return s.items, s.err
}

type staticJobCreator struct {
	item tasks.Job
	err  error
}

func (s staticJobCreator) CreateJob(_ context.Context, _ tasks.CreateJobInput) (tasks.Job, error) {
	return s.item, s.err
}

type staticJobRetrier struct {
	item tasks.Job
	err  error
}

func (s staticJobRetrier) RetryJob(_ context.Context, _ int64) (tasks.Job, error) {
	return s.item, s.err
}

type staticJobEventListProvider struct {
	items []tasks.JobEvent
	err   error
}

func (s staticJobEventListProvider) ListJobEvents(_ context.Context, _ int64, _ int) ([]tasks.JobEvent, error) {
	return s.items, s.err
}

type staticVolumeListProvider struct {
	items []httpserver.VolumeDTO
	err   error
}

func (s staticVolumeListProvider) ListVolumes(_ context.Context) ([]httpserver.VolumeDTO, error) {
	return s.items, s.err
}

type staticVolumeCreator struct {
	item httpserver.VolumeDTO
	err  error
}

func (s staticVolumeCreator) CreateVolume(_ context.Context, _ httpserver.CreateVolumeRequest) (httpserver.VolumeDTO, error) {
	return s.item, s.err
}

type staticVolumeScanner struct {
	volumeID int64
	err      error
}

func (s *staticVolumeScanner) EnqueueVolumeScan(_ context.Context, volumeID int64) (tasks.Job, error) {
	s.volumeID = volumeID
	return tasks.Job{ID: 11, JobType: "scan_volume", Status: "pending", TargetType: "volume", TargetID: volumeID}, s.err
}

type staticFileListProvider struct {
	items       []httpserver.FileDTO
	err         error
	lastRequest httpserver.FileListRequest
}

func (s *staticFileListProvider) ListFiles(_ context.Context, input httpserver.FileListRequest) ([]httpserver.FileDTO, error) {
	s.lastRequest = input
	return s.items, s.err
}

type staticClusterListProvider struct {
	items       []httpserver.ClusterDTO
	err         error
	lastRequest httpserver.ClusterListRequest
}

func (s *staticClusterListProvider) ListClusters(_ context.Context, input httpserver.ClusterListRequest) ([]httpserver.ClusterDTO, error) {
	s.lastRequest = input
	return s.items, s.err
}

type staticClusterDetailProvider struct {
	item httpserver.ClusterDetailDTO
	err  error
}

func (s staticClusterDetailProvider) GetClusterDetail(_ context.Context, _ int64) (httpserver.ClusterDetailDTO, error) {
	return s.item, s.err
}

type staticClusterSummaryProvider struct {
	items []httpserver.ClusterSummaryDTO
	err   error
}

func (s staticClusterSummaryProvider) SummarizeClusters(_ context.Context) ([]httpserver.ClusterSummaryDTO, error) {
	return s.items, s.err
}

type staticFileDetailProvider struct {
	item httpserver.FileDetailDTO
	err  error
}

func (s staticFileDetailProvider) GetFileDetail(_ context.Context, _ int64) (httpserver.FileDetailDTO, error) {
	return s.item, s.err
}

type staticFileContentProvider struct {
	item httpserver.FileContent
	err  error
}

func (s staticFileContentProvider) GetFileContent(_ context.Context, _ int64) (httpserver.FileContent, error) {
	return s.item, s.err
}

func (s staticFileContentProvider) GetFilePreview(_ context.Context, _ int64) (httpserver.FileContent, error) {
	return s.item, s.err
}

func (s staticFileContentProvider) GetVideoFramePreview(_ context.Context, _ int64, _ int) (httpserver.FileContent, error) {
	return s.item, s.err
}

type staticTagListProvider struct {
	items []httpserver.TagDTO
	err   error
}

func (s staticTagListProvider) ListTags(_ context.Context, _ httpserver.TagListRequest) ([]httpserver.TagDTO, error) {
	return s.items, s.err
}

type staticFileTrashProvider struct {
	fileID int64
	err    error
}

func (s *staticFileTrashProvider) TrashFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}

type staticFileRevealProvider struct {
	fileID int64
	err    error
}

func (s *staticFileRevealProvider) RevealFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return s.err
}

type staticFileTagCreator struct {
	fileID        int64
	input         httpserver.FileTagCreateRequest
	deletedFileID int64
	deleted       httpserver.FileTagDeleteRequest
	err           error
}

func (s *staticFileTagCreator) CreateFileTag(_ context.Context, fileID int64, input httpserver.FileTagCreateRequest) error {
	s.fileID = fileID
	s.input = input
	return s.err
}

func (s *staticFileTagCreator) DeleteFileTag(_ context.Context, fileID int64, input httpserver.FileTagDeleteRequest) error {
	s.deletedFileID = fileID
	s.deleted = input
	return s.err
}

type staticFileJobRunner struct {
	embeddingFileID int64
	embeddingInput  httpserver.FileRecomputeRequest
	reclusterFileID int64
	reclusterInput  httpserver.FileRecomputeRequest
	err             error
}

func (s *staticFileJobRunner) RecomputeFileEmbeddings(_ context.Context, fileID int64, input httpserver.FileRecomputeRequest) error {
	s.embeddingFileID = fileID
	s.embeddingInput = input
	return s.err
}

func (s *staticFileJobRunner) ReclusterFile(_ context.Context, fileID int64, input httpserver.FileRecomputeRequest) error {
	s.reclusterFileID = fileID
	s.reclusterInput = input
	return s.err
}

type staticFileReviewer struct {
	fileID int64
	input  httpserver.FileReviewActionRequest
	err    error
}

func (s *staticFileReviewer) ApplyFileReviewAction(_ context.Context, fileID int64, input httpserver.FileReviewActionRequest) error {
	s.fileID = fileID
	s.input = input
	return s.err
}

type staticClusterReviewer struct {
	clusterID       int64
	input           httpserver.FileReviewActionRequest
	statusClusterID int64
	status          string
	err             error
}

func (s *staticClusterReviewer) ApplyClusterReviewAction(_ context.Context, clusterID int64, input httpserver.FileReviewActionRequest) error {
	s.clusterID = clusterID
	s.input = input
	return s.err
}

func (s *staticClusterReviewer) UpdateClusterStatus(_ context.Context, clusterID int64, status string) error {
	s.statusClusterID = clusterID
	s.status = status
	return s.err
}

func contains(haystack string, needle string) bool {
	return strings.Contains(haystack, needle)
}

func intPtr(v int) *int {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
