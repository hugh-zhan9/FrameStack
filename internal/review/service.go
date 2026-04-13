package review

import (
	"context"
	"errors"
	"strings"
)

var ErrUnsupportedAction = errors.New("unsupported review action")

type FileActionInput struct {
	ActionType string
	Note       string
}

type Store interface {
	CreateFileAction(ctx context.Context, fileID int64, input FileActionInput) error
}

type Service struct {
	Store Store
}

func (s Service) ApplyFileAction(ctx context.Context, fileID int64, input FileActionInput) error {
	input.ActionType = strings.TrimSpace(input.ActionType)
	input.Note = strings.TrimSpace(input.Note)
	if !isSupportedAction(input.ActionType) {
		return ErrUnsupportedAction
	}
	return s.Store.CreateFileAction(ctx, fileID, input)
}

func isSupportedAction(actionType string) bool {
	switch actionType {
	case "keep", "favorite", "ignore", "hide", "trash_candidate":
		return true
	default:
		return false
	}
}
