package trash

import "context"

type File struct {
	ID       int64
	AbsPath  string
	Status   string
	VolumeID int64
}

type Store interface {
	GetFile(ctx context.Context, fileID int64) (File, error)
	MarkFileTrashed(ctx context.Context, fileID int64) error
}

type Mover interface {
	MoveToTrash(ctx context.Context, path string) error
}

type Service struct {
	Store Store
	Mover Mover
}

func (s Service) TrashFile(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	if file.Status == "trashed" {
		return nil
	}
	if err := s.Mover.MoveToTrash(ctx, file.AbsPath); err != nil {
		return err
	}
	return s.Store.MarkFileTrashed(ctx, file.ID)
}
