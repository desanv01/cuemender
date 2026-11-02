package domain

import "time"

const (
	TrackRepairInvalidated = "invalidated"
	TrackRepairNoOp        = "no_op"
	TrackRepairVerified    = "verified"
	TrackRepairFailed      = "failed"
	TrackRepairUnknown     = "unknown"
)

// TrackStateRequest pins a fresh read to one manager lifecycle and source.
// Reconciliation supplies the current generation/revision with the original
// persisted source identity; it never retargets the current configured mic.
type TrackStateRequest struct {
	Input          InputRef
	Generation     uint64
	ConfigRevision int64
}

type TrackState struct {
	Input          InputRef        `json:"input"`
	Tracks         map[string]bool `json:"tracks"`
	Generation     uint64          `json:"connection_generation"`
	ConfigRevision int64           `json:"config_revision"`
	ObservedAt     time.Time       `json:"observed_at"`
}

// TrackRepairRequest is an internal approved-action binding, never an HTTP
// request body. Session, expiry and reservation authority belong to actions.
type TrackRepairRequest struct {
	ActionID       string
	Input          InputRef
	Generation     uint64
	ConfigRevision int64
	ExpectedTracks map[string]bool
	SelectedTrack  int
}

type TrackRepairPreflight struct {
	ActionID      string          `json:"action_id"`
	State         TrackState      `json:"state"`
	DesiredTracks map[string]bool `json:"desired_tracks"`
	SelectedTrack int             `json:"selected_track"`
}

// DispatchAdmitted means the authority callback accepted the fresh preflight.
// PossiblyTransmitted is set conservatively at SDK invocation, including when
// cancellation or a transport error makes actual transmission unknowable.
// Results retain their old generation so their action audit can settle even
// after a lifecycle replacement; only current-generation readback can verify.
type TrackRepairResult struct {
	ActionID            string                `json:"action_id"`
	Generation          uint64                `json:"connection_generation"`
	ConfigRevision      int64                 `json:"config_revision"`
	Status              string                `json:"status"`
	DetailCode          string                `json:"detail_code"`
	DispatchAdmitted    bool                  `json:"dispatch_admitted"`
	PossiblyTransmitted bool                  `json:"possibly_transmitted"`
	Preflight           *TrackRepairPreflight `json:"preflight,omitempty"`
	Observed            *TrackState           `json:"observed,omitempty"`
}
