package filetags

import (
	"context"
	"strings"
)

type CreateInput struct {
	Namespace   string
	Name        string
	DisplayName string
}

type DeleteInput struct {
	Namespace string
	Name      string
}

type Store interface {
	UpsertManualTag(ctx context.Context, fileID int64, input CreateInput) error
	DeleteManualTag(ctx context.Context, fileID int64, input DeleteInput) error
}

type SamePersonClusterer interface {
	ClusterFile(ctx context.Context, fileID int64) error
}

type Service struct {
	Store      Store
	SamePerson SamePersonClusterer
}

func (s Service) CreateFileTag(ctx context.Context, fileID int64, input CreateInput) error {
	input.Namespace = strings.TrimSpace(input.Namespace)
	input.Name = strings.TrimSpace(input.Name)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.DisplayName == "" {
		input.DisplayName = input.Name
	}
	if err := s.Store.UpsertManualTag(ctx, fileID, input); err != nil {
		return err
	}
	if input.Namespace == "person" && s.SamePerson != nil {
		return s.SamePerson.ClusterFile(ctx, fileID)
	}
	return nil
}

func (s Service) DeleteFileTag(ctx context.Context, fileID int64, input DeleteInput) error {
	input.Namespace = strings.TrimSpace(input.Namespace)
	input.Name = strings.TrimSpace(input.Name)
	if err := s.Store.DeleteManualTag(ctx, fileID, input); err != nil {
		return err
	}
	if input.Namespace == "person" && s.SamePerson != nil {
		return s.SamePerson.ClusterFile(ctx, fileID)
	}
	return nil
}
