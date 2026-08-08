// Package storage abstracts original-document storage so the backend
// (local disk for dev, S3-compatible for prod) can be swapped without
// touching callers (docs/architecture/system-architecture.md, NFR-17).
package storage

import (
	"context"
	"io"
)

type Store interface {
	// Save writes the content under key and returns the location it was stored at.
	Save(ctx context.Context, key string, r io.Reader) (path string, err error)
	// Open returns a reader for the previously stored content at path.
	Open(ctx context.Context, path string) (io.ReadCloser, error)
}
