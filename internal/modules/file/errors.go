package file

import "errors"

var (
	ErrInvalidFile    = errors.New("invalid file")
	ErrTypeNotAllowed = errors.New("file type not allowed")
	ErrTooLarge       = errors.New("file too large")
	ErrInvalidKey     = errors.New("invalid file key")
	ErrNotFound       = errors.New("file not found")
	ErrForbidden      = errors.New("file operation forbidden")
	ErrUnavailable    = errors.New("file storage unavailable")
)
