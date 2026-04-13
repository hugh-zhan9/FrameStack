package review_test

import (
	"context"
	"errors"
	"testing"

	"idea/internal/review"
)

func TestServiceApplyFileActionRecordsReviewAction(t *testing.T) {
	store := &recordingStore{}
	service := review.Service{Store: store}

	err := service.ApplyFileAction(context.Background(), 7, review.FileActionInput{
		ActionType: "favorite",
		Note:       "manual favorite",
	})
	if err != nil {
		t.Fatalf("expected review action to succeed: %v", err)
	}
	if store.fileID != 7 || store.input.ActionType != "favorite" || store.input.Note != "manual favorite" {
		t.Fatalf("unexpected store call: %#v %#v", store.fileID, store.input)
	}
}

func TestServiceApplyFileActionRejectsUnsupportedAction(t *testing.T) {
	service := review.Service{Store: &recordingStore{}}

	err := service.ApplyFileAction(context.Background(), 7, review.FileActionInput{
		ActionType: "deleted_to_trash",
	})
	if !errors.Is(err, review.ErrUnsupportedAction) {
		t.Fatalf("expected unsupported action, got %v", err)
	}
}

type recordingStore struct {
	fileID int64
	input  review.FileActionInput
}

func (s *recordingStore) CreateFileAction(_ context.Context, fileID int64, input review.FileActionInput) error {
	s.fileID = fileID
	s.input = input
	return nil
}
