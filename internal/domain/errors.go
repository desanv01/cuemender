package domain

import "errors"

var (
	ErrBusy              = errors.New("integration lifecycle is busy")
	ErrUnavailable       = errors.New("integration is unavailable")
	ErrInvalidInput      = errors.New("invalid integration input")
	ErrCredentialStorage = errors.New("credential storage is unavailable")
	ErrNotReady          = errors.New("monitoring prerequisites are not ready")
	ErrIncidentNotFound  = errors.New("incident not found")
	ErrTerminalIncident  = errors.New("incident is terminal")
	ErrTooManyReports    = errors.New("report detail bound reached")
	ErrJobLimit          = errors.New("diagnostic job limit reached")
)
