package reveal

import (
	"context"
	"os/exec"
)

type File struct {
	ID      int64
	AbsPath string
	Status  string
}

type Store interface {
	GetFile(ctx context.Context, fileID int64) (File, error)
}

type Revealer interface {
	RevealInFinder(ctx context.Context, path string) error
}

type Opener interface {
	OpenWithDefaultApp(ctx context.Context, path string) error
}

type Service struct {
	Store    Store
	Revealer Revealer
	Opener   Opener
}

func (s Service) RevealFile(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	return s.Revealer.RevealInFinder(ctx, file.AbsPath)
}

func (s Service) OpenFile(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	return s.Opener.OpenWithDefaultApp(ctx, file.AbsPath)
}

type MacOSRevealer struct{}

func (MacOSRevealer) RevealInFinder(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "open", "-R", path)
	return cmd.Run()
}

func (MacOSRevealer) OpenWithDefaultApp(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "open", path)
	return cmd.Run()
}
