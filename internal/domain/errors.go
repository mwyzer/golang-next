package domain

import "errors"

var (
	ErrUnsupportedFileType = errors.New("unsupported file type")
	ErrFileTooLarge        = errors.New("file exceeds maximum allowed size")
	ErrDocumentNotFound    = errors.New("document not found")
)
