package systemconfig

import "errors"

var (
	ErrInvalidInput = errors.New("invalid system config input")
	ErrInvalidType  = errors.New("invalid system config type")
	ErrSensitiveKey = errors.New("sensitive system config key")
	ErrNotFound     = errors.New("system config not found")
	ErrConflict     = errors.New("system config conflict")
	ErrPostCommit   = errors.New("system config committed with side effect failure")
)
