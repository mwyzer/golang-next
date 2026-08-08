package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStore stores files on disk under a root directory. Intended for
// local development; production should use an S3-compatible Store.
type LocalStore struct {
	Root string
}

func NewLocalStore(root string) (*LocalStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return &LocalStore{Root: root}, nil
}

func (s *LocalStore) Save(_ context.Context, key string, r io.Reader) (string, error) {
	path := filepath.Join(s.Root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create storage dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return path, nil
}

func (s *LocalStore) Open(_ context.Context, path string) (io.ReadCloser, error) {
	return os.Open(path)
}
