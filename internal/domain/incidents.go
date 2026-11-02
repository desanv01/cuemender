package domain

import "time"

const (
	ModeMonitoring = "monitoring"

	StageReported                 = "reported"
	StageInvestigating            = "investigating"
	StageFindingAvailable         = "finding_available"
	StageNoSupportedFinding       = "no_supported_finding"
	StageInsufficientInfo         = "insufficient_information"
	StageInterrupted              = "interrupted"
	StageAwaitingApproval         = "awaiting_approval"
	StageAwaitingFeedback         = "awaiting_feedback"
	StageUnresolved               = "unresolved"
	StageDismissed                = "dismissed"
	StageClosed                   = "closed"
	StageExpired                  = "expired"
	StageInvalidated              = "invalidated"
	StageApplying                 = "applying"
	StageActionUnknown            = "action_unknown"
	StageApplyingPlaceholder      = "applying"
	StageActionUnknownPlaceholder = "action_unknown"
)

func StageTerminal(stage string) bool {
	switch stage {
	case StageDismissed, StageClosed, StageExpired, StageInvalidated:
		return true
	default:
		return false
	}
}

const (
	EvidenceKnown   = "known"
	EvidenceUnknown = "unknown"
	EvidenceStale   = "stale"

	FindingMICMuted        = "MIC_MUTED"
	FindingMICTrackMissing = "MIC_TRACK_MISSING"
	FindingVisualHidden    = "VISUAL_ITEM_HIDDEN"

	ActionAddMicToTrack = "ADD_MIC_TO_TRACK"
	LocalNotAttempted   = "not_attempted"
	LocalVerified       = "verified"
	LocalFailed         = "failed"
	LocalUnknown        = "unknown"

	OriginManual = "manual"
	OriginViewer = "viewer"
)

type DiagnosticCheck struct {
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Status      string         `json:"status"` // ok, conflict, unknown, not_configured
	Explanation string         `json:"explanation_code"`
	Observed    map[string]any `json:"observed,omitempty"`
	Unavailable string         `json:"unavailable_reason,omitempty"`
}

type Finding struct {
	ID                   string         `json:"id"`
	IncidentID           string         `json:"incident_id"`
	Code                 string         `json:"code"`
	Category             string         `json:"category"`
	EvidenceStatus       string         `json:"evidence_status"`
	ExplanationCode      string         `json:"explanation_code"`
	ExpectationRevision  int64          `json:"expectation_revision"`
	ConnectionGeneration uint64         `json:"connection_generation"`
	Context              string         `json:"context,omitempty"`
	ObservedAt           time.Time      `json:"observed_at"`
	Observed             map[string]any `json:"observed,omitempty"`
	ManualGuidance       string         `json:"manual_guidance,omitempty"`
	SupportedActionKind  string         `json:"supported_action_kind,omitempty"`
}

type FeedbackCounts struct {
	Positive int `json:"positive"`
	Negative int `json:"negative"`
}

type Incident struct {
	ID                string            `json:"id"`
	PublicID          string            `json:"public_id"`
	SessionID         string            `json:"session_id"`
	Mode              string            `json:"mode"`
	Origin            string            `json:"origin"`
	Category          string            `json:"category"`
	Stage             string            `json:"stage"`
	OpenedAt          time.Time         `json:"opened_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	ReportCount       int               `json:"report_count"`
	DistinctReporters int               `json:"distinct_reporters"`
	Findings          []Finding         `json:"findings"`
	Checks            []DiagnosticCheck `json:"checks,omitempty"`
	LocalVerification string            `json:"local_verification"`
	Feedback          FeedbackCounts    `json:"feedback"`
	ClosedAt          *time.Time        `json:"closed_at,omitempty"`
	EvidenceAt        *time.Time        `json:"evidence_at,omitempty"`
	Streaming         *bool             `json:"streaming,omitempty"`
	RepairAvailable   bool              `json:"repair_available"`
}

type MonitoringSessionInfo struct {
	ID        string     `json:"id"`
	Active    bool       `json:"active"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
}
