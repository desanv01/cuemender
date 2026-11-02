package domain

import "time"

const (
	ConnectionNotConnected   = "not_connected"
	ConnectionConnecting     = "connecting"
	ConnectionAuthorizing    = "authorizing"
	ConnectionConnected      = "connected"
	ConnectionReconnecting   = "reconnecting"
	ConnectionNeedsAttention = "needs_attention"
)

// DeviceAuthorization is private authorization guidance. Device code and
// tokens never appear in a projection.
type DeviceAuthorization struct {
	VerificationURI string    `json:"verification_uri"`
	UserCode        string    `json:"user_code"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type CommandReceipt struct {
	Accepted     int        `json:"accepted"`
	LastCommand  string     `json:"last_command,omitempty"`
	LastCategory string     `json:"last_category,omitempty"`
	LastAt       *time.Time `json:"last_at,omitempty"`
}

// TwitchConnection is the private Twitch setup projection. Detail fields are
// fixed safe codes/copy; no provider error body or token is included.
type TwitchConnection struct {
	Provider                      string               `json:"provider,omitempty"`
	ReceiveOnly                   bool                 `json:"receive_only"`
	Status                        string               `json:"status"`
	DetailCode                    string               `json:"detail_code,omitempty"`
	Detail                        string               `json:"detail,omitempty"`
	Login                         string               `json:"login,omitempty"`
	ClientIDConfigured            bool                 `json:"client_id_configured"`
	CredentialPersistenceDegraded bool                 `json:"credential_persistence_degraded"`
	ConnectionGeneration          uint64               `json:"connection_generation"`
	Device                        *DeviceAuthorization `json:"device,omitempty"`
	Receipt                       CommandReceipt       `json:"receipt"`
}

type OBSConnection struct {
	Status          string     `json:"status"`
	DetailCode      string     `json:"detail_code,omitempty"`
	Detail          string     `json:"detail,omitempty"`
	Address         string     `json:"address,omitempty"`
	Generation      uint64     `json:"generation"`
	ConfigRevision  int64      `json:"config_revision"`
	SceneCollection string     `json:"scene_collection,omitempty"`
	Broadcasting    *bool      `json:"broadcasting,omitempty"`
	ObservedAt      *time.Time `json:"observed_at,omitempty"`
	Stale           bool       `json:"stale"`
}

type OBSInput struct {
	Name string `json:"name"`
	UUID string `json:"uuid,omitempty"`
	Kind string `json:"kind,omitempty"`
}

type OBSSceneItem struct {
	ID         int    `json:"id"`
	SourceName string `json:"source_name"`
	SourceUUID string `json:"source_uuid,omitempty"`
	Enabled    bool   `json:"enabled"`
	IsGroup    bool   `json:"is_group"`
}

type OBSScene struct {
	Name  string         `json:"name"`
	UUID  string         `json:"uuid,omitempty"`
	Items []OBSSceneItem `json:"items"`
}

type OBSDiscovery struct {
	Generation      uint64     `json:"generation"`
	ConfigRevision  int64      `json:"config_revision"`
	SceneCollection string     `json:"scene_collection,omitempty"`
	CollectionKnown bool       `json:"collection_known"`
	CapturedAt      time.Time  `json:"captured_at"`
	Inputs          []OBSInput `json:"inputs"`
	Scenes          []OBSScene `json:"scenes"`
}

// OBSObservation has independently-unknown fields. A nil mute, track map,
// visual item or streaming value means the field could not be read, never
// false. A complete track map has track1..track6 when known.
type OBSObservation struct {
	Generation        uint64          `json:"generation"`
	ConfigRevision    int64           `json:"config_revision"`
	SceneCollection   string          `json:"scene_collection,omitempty"`
	ObservedAt        time.Time       `json:"observed_at"`
	Microphone        *InputRef       `json:"microphone,omitempty"`
	Mute              *bool           `json:"mute,omitempty"`
	Tracks            map[string]bool `json:"tracks,omitempty"`
	ProgramScene      string          `json:"program_scene,omitempty"`
	ProgramSceneUUID  string          `json:"program_scene_uuid,omitempty"`
	VisualItemEnabled *bool           `json:"visual_item_enabled,omitempty"`
	Streaming         *bool           `json:"streaming,omitempty"`
	Partial           []string        `json:"partial,omitempty"`
}
