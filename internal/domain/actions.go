package domain

import (
	"errors"
	"time"
)

var (
	ErrProposalNotFound = errors.New("proposal not found")
	ErrProposalExpired  = errors.New("proposal expired")
	ErrActionConflict   = errors.New("action authority changed")
)

const (
	ProposalProposed    = "proposed"
	ProposalReserved    = "reserved"
	ProposalInvalidated = "invalidated"
	ProposalExpired     = "expired"
	ActionReserved      = "reserved"
	ActionIntent        = "intent_recorded"
)

type Proposal struct {
	ID               string          `json:"id"`
	IncidentID       string          `json:"incident_id"`
	SessionID        string          `json:"session_id"`
	Mode             string          `json:"mode"`
	Kind             string          `json:"kind"`
	Input            InputRef        `json:"input"`
	ExpectedTracks   map[string]bool `json:"expected_tracks"`
	DesiredTracks    map[string]bool `json:"desired_tracks"`
	SelectedTrack    int             `json:"selected_track"`
	ConfigRevision   int64           `json:"config_revision"`
	TwitchGeneration uint64          `json:"twitch_generation"`
	Generation       uint64          `json:"connection_generation"`
	CreatedAt        time.Time       `json:"created_at"`
	ExpiresAt        time.Time       `json:"expires_at"`
	Status           string          `json:"status"`
	ActionID         string          `json:"action_id,omitempty"`
}

type Action struct {
	ID                  string                `json:"id"`
	ProposalID          string                `json:"proposal_id"`
	IncidentID          string                `json:"incident_id"`
	Binding             Proposal              `json:"binding"`
	Status              string                `json:"status"`
	DetailCode          string                `json:"detail_code"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	IntentAt            *time.Time            `json:"intent_at,omitempty"`
	PossiblyTransmitted bool                  `json:"possibly_transmitted"`
	Preflight           *TrackRepairPreflight `json:"preflight,omitempty"`
	Observed            *TrackState           `json:"observed,omitempty"`
	Reconciled          bool                  `json:"reconciled"`
}

func ActionPending(status string) bool {
	return status == ActionReserved || status == ActionIntent || status == "write_sent" || status == TrackRepairUnknown
}
