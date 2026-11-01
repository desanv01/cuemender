package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

const maxEventSubMessage = 1 << 20

var ErrProtocol = errors.New("invalid eventsub protocol message")

type Socket interface {
	Read(ctx context.Context) ([]byte, error)
	Close() error
}

type Dialer interface {
	Dial(ctx context.Context, endpoint string, header http.Header) (Socket, error)
}

type CoderDialer struct {
	HTTP *http.Client
}

func (d CoderDialer) Dial(ctx context.Context, endpoint string, header http.Header) (Socket, error) {
	client := d.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPClient: client,
		HTTPHeader: header,
	})
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(maxEventSubMessage)
	return &coderSocket{conn: conn}, nil
}

type coderSocket struct {
	conn *websocket.Conn
}

func (s *coderSocket) Read(ctx context.Context) ([]byte, error) {
	typ, data, err := s.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	if typ != websocket.MessageText {
		return nil, ErrProtocol
	}
	if len(data) > maxEventSubMessage {
		return nil, ErrProtocol
	}
	return data, nil
}

func (s *coderSocket) Close() error {
	// Lifecycle cancellation must not wait for an unresponsive close handshake.
	return s.conn.CloseNow()
}

type EventEnvelope struct {
	Metadata EventMetadata   `json:"metadata"`
	Payload  json.RawMessage `json:"payload"`
}

type EventMetadata struct {
	MessageID        string    `json:"message_id"`
	MessageType      string    `json:"message_type"`
	MessageTimestamp time.Time `json:"message_timestamp"`
}

type SessionInfo struct {
	ID                      string `json:"id"`
	Status                  string `json:"status"`
	KeepaliveTimeoutSeconds *int   `json:"keepalive_timeout_seconds"`
	ReconnectURL            string `json:"reconnect_url"`
}

type WelcomePayload struct {
	Session SessionInfo `json:"session"`
}

type ReconnectPayload struct {
	Session SessionInfo `json:"session"`
}

type ChatEvent struct {
	MessageID         string `json:"message_id"`
	BroadcasterUserID string `json:"broadcaster_user_id"`
	ChatterUserID     string `json:"chatter_user_id"`
	ChatterUserLogin  string `json:"chatter_user_login"`
	MessageType       string `json:"message_type"`
	Message           struct {
		Text string `json:"text"`
	} `json:"message"`
}

type ChatNotification struct {
	Subscription struct {
		Type    string `json:"type"`
		Version string `json:"version"`
	} `json:"subscription"`
	Event ChatEvent `json:"event"`
}

func ParseEnvelope(data []byte) (EventEnvelope, error) {
	if len(data) == 0 || len(data) > maxEventSubMessage {
		return EventEnvelope{}, ErrProtocol
	}
	var env EventEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return EventEnvelope{}, ErrProtocol
	}
	switch env.Metadata.MessageType {
	case "session_welcome", "session_keepalive", "session_reconnect", "notification", "revocation":
	default:
		return EventEnvelope{}, ErrProtocol
	}
	if env.Metadata.MessageID == "" {
		return EventEnvelope{}, ErrProtocol
	}
	return env, nil
}

func ParseWelcome(env EventEnvelope) (SessionInfo, error) {
	if env.Metadata.MessageType != "session_welcome" {
		return SessionInfo{}, ErrProtocol
	}
	var payload WelcomePayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return SessionInfo{}, ErrProtocol
	}
	if payload.Session.ID == "" {
		return SessionInfo{}, ErrProtocol
	}
	return payload.Session, nil
}

func ParseReconnect(env EventEnvelope) (SessionInfo, error) {
	if env.Metadata.MessageType != "session_reconnect" {
		return SessionInfo{}, ErrProtocol
	}
	var payload ReconnectPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return SessionInfo{}, ErrProtocol
	}
	if payload.Session.ReconnectURL == "" {
		return SessionInfo{}, ErrProtocol
	}
	return payload.Session, nil
}

func ParseChatNotification(env EventEnvelope) (ChatNotification, error) {
	if env.Metadata.MessageType != "notification" {
		return ChatNotification{}, ErrProtocol
	}
	var payload ChatNotification
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return ChatNotification{}, ErrProtocol
	}
	if payload.Subscription.Type != "channel.chat.message" || payload.Subscription.Version != "1" {
		return ChatNotification{}, ErrProtocol
	}
	if payload.Event.BroadcasterUserID == "" || payload.Event.ChatterUserID == "" || payload.Event.MessageID == "" {
		return ChatNotification{}, ErrProtocol
	}
	if len(payload.Event.Message.Text) > 4096 {
		return ChatNotification{}, ErrProtocol
	}
	return payload, nil
}

// ValidateReconnectURL only permits the secure Twitch EventSub host.
func ValidateReconnectURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "wss" || u.Hostname() != "eventsub.wss.twitch.tv" {
		return fmt.Errorf("%w: unexpected reconnect endpoint", ErrProtocol)
	}
	if u.User != nil || u.Port() != "" || strings.Contains(raw, " ") {
		return ErrProtocol
	}
	return nil
}

func keepaliveDuration(session SessionInfo) time.Duration {
	if session.KeepaliveTimeoutSeconds == nil || *session.KeepaliveTimeoutSeconds <= 0 {
		return 30 * time.Second
	}
	d := time.Duration(*session.KeepaliveTimeoutSeconds) * time.Second
	if d < 5*time.Second {
		return 5 * time.Second
	}
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}
