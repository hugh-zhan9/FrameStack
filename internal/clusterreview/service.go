package clusterreview

import (
	"context"
	"errors"
	"strings"

	"idea/internal/clusters"
	"idea/internal/review"
)

var ErrUnsupportedClusterStatus = errors.New("unsupported cluster status")

type ClusterDetailProvider interface {
	GetClusterDetail(ctx context.Context, clusterID int64) (clusters.ClusterDetail, error)
}

type FileActionApplier interface {
	ApplyFileAction(ctx context.Context, fileID int64, input review.FileActionInput) error
}

type StatusStore interface {
	UpdateClusterStatus(ctx context.Context, clusterID int64, status string) error
}

type Service struct {
	Clusters    ClusterDetailProvider
	Actions     FileActionApplier
	StatusStore StatusStore
}

func (s Service) ApplyClusterAction(ctx context.Context, clusterID int64, input review.FileActionInput) error {
	detail, err := s.Clusters.GetClusterDetail(ctx, clusterID)
	if err != nil {
		return err
	}

	seen := make(map[int64]struct{}, len(detail.Members))
	for _, member := range detail.Members {
		if _, ok := seen[member.FileID]; ok {
			continue
		}
		seen[member.FileID] = struct{}{}
		if err := s.Actions.ApplyFileAction(ctx, member.FileID, input); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) UpdateClusterStatus(ctx context.Context, clusterID int64, status string) error {
	status = strings.TrimSpace(status)
	if !isSupportedClusterStatus(status) {
		return ErrUnsupportedClusterStatus
	}
	if s.StatusStore == nil {
		return nil
	}
	return s.StatusStore.UpdateClusterStatus(ctx, clusterID, status)
}

func isSupportedClusterStatus(status string) bool {
	switch status {
	case "candidate", "confirmed", "ignored":
		return true
	default:
		return false
	}
}
