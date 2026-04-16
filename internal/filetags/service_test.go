package filetags_test

import (
	"context"
	"testing"

	"idea/internal/filetags"
)

func TestServiceCreatesManualTag(t *testing.T) {
	store := &recordingStore{}
	service := filetags.Service{Store: store}

	if err := service.CreateFileTag(context.Background(), 7, filetags.CreateInput{
		Namespace: "content",
		Name:      "单人写真",
	}); err != nil {
		t.Fatalf("expected create file tag to succeed: %v", err)
	}
	if store.fileID != 7 || store.input.Namespace != "content" || store.input.Name != "单人写真" {
		t.Fatalf("unexpected store call: %#v %#v", store.fileID, store.input)
	}
}

func TestServiceReclustersSamePersonWhenManualPersonTagAdded(t *testing.T) {
	store := &recordingStore{}
	clusterer := &recordingSamePersonClusterer{}
	service := filetags.Service{
		Store:      store,
		SamePerson: clusterer,
	}

	if err := service.CreateFileTag(context.Background(), 7, filetags.CreateInput{
		Namespace: "person",
		Name:      "alice",
	}); err != nil {
		t.Fatalf("expected create file tag to succeed: %v", err)
	}
	if clusterer.fileID != 7 {
		t.Fatalf("expected same person recluster for file 7, got %d", clusterer.fileID)
	}
}

func TestServiceDeletesManualTag(t *testing.T) {
	store := &recordingStore{}
	service := filetags.Service{Store: store}

	if err := service.DeleteFileTag(context.Background(), 7, filetags.DeleteInput{
		Namespace: "content",
		Name:      "单人写真",
	}); err != nil {
		t.Fatalf("expected delete file tag to succeed: %v", err)
	}
	if store.deletedFileID != 7 || store.deleted.Namespace != "content" || store.deleted.Name != "单人写真" {
		t.Fatalf("unexpected delete call: %#v %#v", store.deletedFileID, store.deleted)
	}
}

func TestServiceReclustersSamePersonWhenManualPersonTagDeleted(t *testing.T) {
	store := &recordingStore{}
	clusterer := &recordingSamePersonClusterer{}
	service := filetags.Service{
		Store:      store,
		SamePerson: clusterer,
	}

	if err := service.DeleteFileTag(context.Background(), 7, filetags.DeleteInput{
		Namespace: "person",
		Name:      "alice",
	}); err != nil {
		t.Fatalf("expected delete file tag to succeed: %v", err)
	}
	if clusterer.fileID != 7 {
		t.Fatalf("expected same person recluster for file 7, got %#v", clusterer)
	}
}

func TestServiceReplacesFileTag(t *testing.T) {
	store := &recordingStore{}
	service := filetags.Service{Store: store}

	if err := service.ReplaceFileTag(context.Background(), 7, filetags.ReplaceInput{
		CurrentNamespace: "content",
		CurrentName:      "室内",
		Namespace:        "content",
		Name:             "酒店场景",
		DisplayName:      "酒店场景",
	}); err != nil {
		t.Fatalf("expected replace file tag to succeed: %v", err)
	}
	if store.replacedFileID != 7 {
		t.Fatalf("expected replace file tag to receive file 7, got %d", store.replacedFileID)
	}
	if store.replaced.CurrentName != "室内" || store.replaced.Name != "酒店场景" {
		t.Fatalf("unexpected replace input: %#v", store.replaced)
	}
}

func TestServiceReclustersSamePersonWhenFileTagReplaced(t *testing.T) {
	store := &recordingStore{}
	clusterer := &recordingSamePersonClusterer{}
	service := filetags.Service{
		Store:      store,
		SamePerson: clusterer,
	}

	if err := service.ReplaceFileTag(context.Background(), 7, filetags.ReplaceInput{
		CurrentNamespace: "content",
		CurrentName:      "单人",
		Namespace:        "person",
		Name:             "alice",
	}); err != nil {
		t.Fatalf("expected replace file tag to succeed: %v", err)
	}
	if clusterer.fileID != 7 {
		t.Fatalf("expected same person recluster for file 7, got %d", clusterer.fileID)
	}
}

type recordingStore struct {
	fileID        int64
	input         filetags.CreateInput
	deletedFileID int64
	deleted       filetags.DeleteInput
	replacedFileID int64
	replaced       filetags.ReplaceInput
}

func (s *recordingStore) UpsertManualTag(_ context.Context, fileID int64, input filetags.CreateInput) error {
	s.fileID = fileID
	s.input = input
	return nil
}

func (s *recordingStore) DeleteManualTag(_ context.Context, fileID int64, input filetags.DeleteInput) error {
	s.deletedFileID = fileID
	s.deleted = input
	return nil
}

func (s *recordingStore) ReplaceTag(_ context.Context, fileID int64, input filetags.ReplaceInput) error {
	s.replacedFileID = fileID
	s.replaced = input
	return nil
}

type recordingSamePersonClusterer struct {
	fileID int64
}

func (s *recordingSamePersonClusterer) ClusterFile(_ context.Context, fileID int64) error {
	s.fileID = fileID
	return nil
}
