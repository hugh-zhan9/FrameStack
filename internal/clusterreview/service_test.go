package clusterreview_test

import (
	"context"
	"testing"

	"idea/internal/clusterreview"
	"idea/internal/clusters"
	"idea/internal/review"
)

func TestServiceApplyClusterActionAppliesToAllMembers(t *testing.T) {
	store := &recordingClusterStore{
		detail: clusters.ClusterDetail{
			Cluster: clusters.Cluster{ID: 7},
			Members: []clusters.ClusterMember{
				{FileID: 11},
				{FileID: 12},
				{FileID: 11},
			},
		},
	}
	applier := &recordingActionApplier{}
	service := clusterreview.Service{
		Clusters: store,
		Actions:  applier,
	}

	if err := service.ApplyClusterAction(context.Background(), 7, review.FileActionInput{
		ActionType: "keep",
		Note:       "cluster keep",
	}); err != nil {
		t.Fatalf("expected cluster action to succeed: %v", err)
	}
	if len(applier.calls) != 2 {
		t.Fatalf("expected 2 unique file actions, got %#v", applier.calls)
	}
	if applier.calls[0].fileID != 11 || applier.calls[1].fileID != 12 {
		t.Fatalf("unexpected file ids: %#v", applier.calls)
	}
}

func TestServiceApplyClusterActionSkipsEmptyCluster(t *testing.T) {
	service := clusterreview.Service{
		Clusters: recordingClusterStore{
			detail: clusters.ClusterDetail{Cluster: clusters.Cluster{ID: 7}},
		},
		Actions: &recordingActionApplier{},
	}

	if err := service.ApplyClusterAction(context.Background(), 7, review.FileActionInput{
		ActionType: "keep",
	}); err != nil {
		t.Fatalf("expected empty cluster action to succeed: %v", err)
	}
}

func TestServiceUpdateClusterStatusAcceptsSupportedStatus(t *testing.T) {
	store := &recordingStatusStore{}
	service := clusterreview.Service{
		StatusStore: store,
	}

	if err := service.UpdateClusterStatus(context.Background(), 7, "confirmed"); err != nil {
		t.Fatalf("expected status update to succeed: %v", err)
	}
	if store.clusterID != 7 || store.status != "confirmed" {
		t.Fatalf("unexpected status update: %#v", store)
	}
}

func TestServiceUpdateClusterStatusRejectsUnsupportedStatus(t *testing.T) {
	service := clusterreview.Service{}

	if err := service.UpdateClusterStatus(context.Background(), 7, "done"); err == nil {
		t.Fatal("expected unsupported status to fail")
	}
}

type recordingClusterStore struct {
	detail clusters.ClusterDetail
}

func (s recordingClusterStore) GetClusterDetail(_ context.Context, _ int64) (clusters.ClusterDetail, error) {
	return s.detail, nil
}

type recordingActionApplier struct {
	calls []recordedAction
}

type recordingStatusStore struct {
	clusterID int64
	status    string
}

type recordedAction struct {
	fileID int64
	input  review.FileActionInput
}

func (a *recordingActionApplier) ApplyFileAction(_ context.Context, fileID int64, input review.FileActionInput) error {
	a.calls = append(a.calls, recordedAction{fileID: fileID, input: input})
	return nil
}

func (s *recordingStatusStore) UpdateClusterStatus(_ context.Context, clusterID int64, status string) error {
	s.clusterID = clusterID
	s.status = status
	return nil
}
