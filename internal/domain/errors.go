package domain

import "errors"

var (
	ErrBusy              = errors.New("integration lifecycle is busy")
	ErrUnavailable       = errors.New("integration is unavailable")
	ErrInvalidInput      = errors.New("invalid integration input")
	ErrCredentialStorage = errors.New("credential storage is unavailable")
)
