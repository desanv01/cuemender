package domain

const (
	IntegrationNotConnected = "not_connected"

	SecretsModeOS          = "os_keyring"
	SecretsModeMemory      = "memory_only"
	SecretsModeUnavailable = "unavailable"

	ServiceDisconnected = "disconnected"
)

// IntegrationStatus is a truthful, non-secret connection summary. P01 has no
// Twitch or OBS client, so both remain not_connected.
type IntegrationStatus struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type SecretsStatus struct {
	Mode      string `json:"mode"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

type SetupReadiness struct {
	MicrophoneConfigured   bool `json:"microphone_configured"`
	StreamTrackConfigured  bool `json:"stream_track_configured"`
	VisualConfigured       bool `json:"visual_configured"`
	LiveOBSResolutionReady bool `json:"live_obs_resolution_ready"`
}

type StartupInfo struct {
	ListenAddress            string `json:"listen_address"`
	OBSAddress               string `json:"obs_address"`
	TwitchClientIDConfigured bool   `json:"twitch_client_id_configured"`
}

// PrivateState is the projection served only to an authenticated control
// session. It contains no credentials or bootstrap capability.
type PrivateState struct {
	Mode             string            `json:"mode"`
	StateRevision    uint64            `json:"state_revision"`
	MonitoringActive bool              `json:"monitoring_active"`
	Settings         Settings          `json:"settings"`
	Setup            SetupReadiness    `json:"setup"`
	Secrets          SecretsStatus     `json:"secrets"`
	Twitch           IntegrationStatus `json:"twitch"`
	OBS              IntegrationStatus `json:"obs"`
	Startup          StartupInfo       `json:"startup"`
}

// PublicIncident is the allowlisted public shape for future incidents. P01
// always serves an empty list.
type PublicIncident struct {
	PublicID          string `json:"public_id"`
	Category          string `json:"category"`
	Stage             string `json:"stage"`
	MessageCode       string `json:"message_code"`
	LocalVerification string `json:"local_verification"`
	FeedbackSummary   string `json:"feedback_summary"`
}

// PublicState is the complete public projection. It must never contain
// settings, source names, usernames, endpoints or credentials.
type PublicState struct {
	Mode             string           `json:"mode"`
	StateRevision    uint64           `json:"state_revision"`
	MonitoringActive bool             `json:"monitoring_active"`
	ServiceStatus    string           `json:"service_status"`
	Incidents        []PublicIncident `json:"incidents"`
	DemoLabel        string           `json:"demo_label,omitempty"`
}
