package datalock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrLocked = errors.New("data directory is already in use")

type Lock struct {
	file *os.File
	path string
}

func Acquire(dir string) (*Lock, error) {
	path := filepath.Join(dir, ".cuemender.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open data-directory lock: %w", err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		if errors.Is(err, os.ErrExist) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("%w: %v", ErrLocked, err)
	}
	_ = f.Truncate(0)
	_, _ = f.WriteString("CueMender data-directory lock\n")
	return &Lock{file: f, path: path}, nil
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlockFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}
