package filehash

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
)

const quickHashWindow = 64 * 1024

type File struct {
	ID      int64
	AbsPath string
}

type HashInput struct {
	FileID    int64
	SHA256    string
	QuickHash string
}

type Store interface {
	GetFile(ctx context.Context, fileID int64) (File, error)
	UpdateHashes(ctx context.Context, input HashInput) error
}

type SameContentEnqueuer interface {
	EnqueueSameContent(ctx context.Context, fileID int64) error
}

type Service struct {
	Store               Store
	SameContentEnqueuer SameContentEnqueuer
}

func (s Service) HashFile(ctx context.Context, fileID int64) error {
	file, err := s.Store.GetFile(ctx, fileID)
	if err != nil {
		return err
	}
	shaValue, err := computeSHA256(file.AbsPath)
	if err != nil {
		return err
	}
	quickValue, err := computeQuickHash(file.AbsPath)
	if err != nil {
		return err
	}
	if err := s.Store.UpdateHashes(ctx, HashInput{
		FileID:    file.ID,
		SHA256:    shaValue,
		QuickHash: quickValue,
	}); err != nil {
		return err
	}
	if s.SameContentEnqueuer != nil {
		return s.SameContentEnqueuer.EnqueueSameContent(ctx, file.ID)
	}
	return nil
}

func computeSHA256(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, handle); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func computeQuickHash(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()

	info, err := handle.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()
	hasher := sha256.New()

	var sizeBuf [8]byte
	binary.LittleEndian.PutUint64(sizeBuf[:], uint64(size))
	if _, err := hasher.Write(sizeBuf[:]); err != nil {
		return "", err
	}

	head := make([]byte, quickHashWindow)
	headBytes, err := io.ReadFull(handle, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	if _, err := hasher.Write(head[:headBytes]); err != nil {
		return "", err
	}

	if size > quickHashWindow {
		tailSize := quickHashWindow
		if size < int64(tailSize) {
			tailSize = int(size)
		}
		if _, err := handle.Seek(-int64(tailSize), io.SeekEnd); err != nil {
			return "", err
		}
		tail := make([]byte, tailSize)
		tailBytes, err := io.ReadFull(handle, tail)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return "", err
		}
		if _, err := hasher.Write(tail[:tailBytes]); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
