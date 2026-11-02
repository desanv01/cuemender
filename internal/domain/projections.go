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
	ChatConnected          bool `json:"chat_connected"`
	MicrophoneConfigured   bool `json:"microphone_configured"`
	StreamTrackConfigured  bool `json:"stream_track_configured"`
	VisualConfigured       bool `json:"visual_configured"`
	TwitchAuthorized       bool `json:"twitch_authorized"`
	OBSConnected           bool `json:"obs_connected"`
	OBSDiscoveryReady      bool `json:"obs_discovery_ready"`
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
	Proposals         []Proposal             `json:"proposals"`
	Actions           []Action               `json:"actions"`
	Mode              string                 `json:"mode"`
	StateRevision     uint64                 `json:"state_revision"`
	MonitoringActive  bool                   `json:"monitoring_active"`
	Settings          Settings               `json:"settings"`
	MonitoringSession *MonitoringSessionInfo `json:"monitoring_session,omitempty"`
	Incidents         []Incident             `json:"incidents"`
	Setup             SetupReadiness         `json:"setup"`
	Secrets           SecretsStatus          `json:"secrets"`
	Twitch            TwitchConnection       `json:"twitch"`
	OBS               OBSConnection          `json:"obs"`
	OBSDiscovery      *OBSDiscovery          `json:"obs_discovery,omitempty"`
	OBSObservation    *OBSObservation        `json:"obs_observation,omitempty"`
	Startup           StartupInfo            `json:"startup"`
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
	ClosureOutcome    string `json:"closure_outcome,omitempty"`
}

// PublicState is the complete public projection. It must never contain
// settings, source names, usernames, endpoints or credentials.
type PublicState struct {
	ChatMode         string           `json:"chat_mode,omitempty"`
	Mode             string           `json:"mode"`
	StateRevision    uint64           `json:"state_revision"`
	MonitoringActive bool             `json:"monitoring_active"`
	ServiceStatus    string           `json:"service_status"`
	Incidents        []PublicIncident `json:"incidents"`
	DemoLabel        string           `json:"demo_label,omitempty"`
}
