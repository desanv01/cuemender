package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ScopeReadChat  = "user:read:chat"
	ScopeWriteChat = "user:write:chat"

	credentialKey = "twitch-credentials-v1"
)

var (
	ErrBusy                   = errors.New("twitch lifecycle is busy")
	ErrNoCredentials          = errors.New("no validated twitch credentials")
	ErrCredentialsUnavailable = errors.New("twitch credential storage is unavailable")
	ErrCredentialDeleteFailed = errors.New("twitch credential deletion failed")
	ErrInvalidClientID        = errors.New("invalid twitch public client id")
)

// Endpoints are fixed in production and injectable only for local tests.
type Endpoints struct {
	Device   string
	Token    string
	Validate string
	Helix    string
	EventSub string
}

func DefaultEndpoints() Endpoints {
	return Endpoints{
		Device:   "https://id.twitch.tv/oauth2/device",
		Token:    "https://id.twitch.tv/oauth2/token",
		Validate: "https://id.twitch.tv/oauth2/validate",
		Helix:    "https://api.twitch.tv/helix",
		EventSub: "wss://eventsub.wss.twitch.tv/ws",
	}
}

type ProviderError struct {
	Code       string
	Status     int
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("twitch provider error %s (status %d)", e.Code, e.Status)
}

func (e *ProviderError) RetryAfterDuration() time.Duration { return e.RetryAfter }

type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ClientID     string    `json:"client_id"`
	UserID       string    `json:"user_id"`
	Login        string    `json:"login"`
	Scopes       []string  `json:"scopes"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (c Credentials) HasChatScopes() bool {
	return hasAllScopes(c.Scopes, []string{ScopeReadChat, ScopeWriteChat})
}

type DeviceStart struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       time.Duration
	Interval        time.Duration
}

type TokenInfo struct {
	ClientID  string
	Login     string
	UserID    string
	Scopes    []string
	ExpiresIn time.Duration
}

type OAuth struct {
	HTTP      *http.Client
	Endpoints Endpoints
	Now       func() time.Time
	Sleep     func(context.Context, time.Duration) error
	sendMu    sync.Mutex
	nextSend  time.Time
}

func NewOAuth(httpClient *http.Client, endpoints Endpoints, now func() time.Time) *OAuth {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if now == nil {
		now = time.Now
	}
	return &OAuth{
		HTTP:      httpClient,
		Endpoints: endpoints,
		Now:       now,
		Sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (o *OAuth) StartDevice(ctx context.Context, clientID string) (DeviceStart, error) {
	if !ValidClientID(clientID) {
		return DeviceStart{}, ErrInvalidClientID
	}
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scopes", strings.Join([]string{ScopeReadChat, ScopeWriteChat}, " "))
	var response struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := o.formJSON(ctx, http.MethodPost, o.Endpoints.Device, form, nil, &response); err != nil {
		return DeviceStart{}, err
	}
	if response.DeviceCode == "" || response.UserCode == "" || response.VerificationURI == "" {
		return DeviceStart{}, &ProviderError{Code: "invalid_device_response"}
	}
	if response.ExpiresIn <= 0 {
		response.ExpiresIn = 1800
	}
	if response.Interval <= 0 {
		response.Interval = 5
	}
	return DeviceStart{
		DeviceCode:      response.DeviceCode,
		UserCode:        response.UserCode,
		VerificationURI: response.VerificationURI,
		ExpiresIn:       time.Duration(response.ExpiresIn) * time.Second,
		Interval:        time.Duration(response.Interval) * time.Second,
	}, nil
}

// PollDevice waits for the user to approve the device flow. It honors pending
// and slow_down responses and returns a typed error on denial or expiry.
func (o *OAuth) PollDevice(ctx context.Context, clientID string, start DeviceStart) (Credentials, error) {
	deadline := o.Now().Add(start.ExpiresIn)
	interval := start.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		if !o.Now().Before(deadline) {
			return Credentials{}, &ProviderError{Code: "expired_token"}
		}
		token, retryAfter, err := o.exchangeDeviceCode(ctx, clientID, start.DeviceCode, interval)
		if err == nil {
			return token, nil
		}
		var providerErr *ProviderError
		if !errors.As(err, &providerErr) {
			return Credentials{}, err
		}
		switch providerErr.Code {
		case "authorization_pending":
			// Poll again after the provider interval.
		case "slow_down":
			interval += 5 * time.Second
			if retryAfter > 0 {
				interval = retryAfter
			}
		case "access_denied", "expired_token":
			return Credentials{}, providerErr
		default:
			return Credentials{}, providerErr
		}
		if err := o.Sleep(ctx, interval); err != nil {
			return Credentials{}, err
		}
	}
}

func (o *OAuth) exchangeDeviceCode(ctx context.Context, clientID, deviceCode string, interval time.Duration) (Credentials, time.Duration, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scopes", strings.Join([]string{ScopeReadChat, ScopeWriteChat}, " "))
	form.Set("device_code", deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	var response struct {
		AccessToken  string   `json:"access_token"`
		RefreshToken string   `json:"refresh_token"`
		ExpiresIn    int      `json:"expires_in"`
		Scope        []string `json:"scope"`
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.do(ctx, http.MethodPost, o.Endpoints.Token, []byte(form.Encode()), headers)
	if err != nil {
		return Credentials{}, 0, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return Credentials{}, 0, &ProviderError{Code: "invalid_provider_response"}
	}
	if resp.StatusCode != http.StatusOK {
		return Credentials{}, retryAfter(resp), providerErrorFromBody(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Credentials{}, 0, &ProviderError{Code: "invalid_provider_response"}
	}
	if response.AccessToken == "" || response.RefreshToken == "" {
		return Credentials{}, 0, &ProviderError{Code: "invalid_provider_response"}
	}
	if response.ExpiresIn <= 0 {
		return Credentials{}, 0, &ProviderError{Code: "invalid_token_expiry"}
	}
	creds := Credentials{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		ClientID:     clientID,
		Scopes:       response.Scope,
		ExpiresAt:    o.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
	}
	_ = interval
	return creds, 0, nil
}

func (o *OAuth) Validate(ctx context.Context, accessToken string) (TokenInfo, error) {
	headers := http.Header{}
	headers.Set("Authorization", "OAuth "+accessToken)
	resp, err := o.do(ctx, http.MethodGet, o.Endpoints.Validate, nil, headers)
	if err != nil {
		return TokenInfo{}, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return TokenInfo{}, &ProviderError{Code: "invalid_provider_response"}
	}
	if resp.StatusCode != http.StatusOK {
		return TokenInfo{}, providerErrorFromBody(resp.StatusCode, body)
	}
	var response struct {
		ClientID  string   `json:"client_id"`
		Login     string   `json:"login"`
		UserID    string   `json:"user_id"`
		Scopes    []string `json:"scopes"`
		ExpiresIn int      `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.UserID == "" {
		return TokenInfo{}, &ProviderError{Code: "invalid_provider_response"}
	}
	return TokenInfo{
		ClientID:  response.ClientID,
		Login:     response.Login,
		UserID:    response.UserID,
		Scopes:    response.Scopes,
		ExpiresIn: time.Duration(response.ExpiresIn) * time.Second,
	}, nil
}

func (o *OAuth) Refresh(ctx context.Context, clientID, refreshToken string) (Credentials, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	var response struct {
		AccessToken  string   `json:"access_token"`
		RefreshToken string   `json:"refresh_token"`
		ExpiresIn    int      `json:"expires_in"`
		Scope        []string `json:"scope"`
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.do(ctx, http.MethodPost, o.Endpoints.Token, []byte(form.Encode()), headers)
	if err != nil {
		return Credentials{}, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return Credentials{}, &ProviderError{Code: "invalid_provider_response"}
	}
	if resp.StatusCode != http.StatusOK {
		return Credentials{}, providerErrorFromBody(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Credentials{}, &ProviderError{Code: "invalid_provider_response"}
	}
	if response.AccessToken == "" || response.RefreshToken == "" {
		return Credentials{}, &ProviderError{Code: "invalid_provider_response"}
	}
	if response.ExpiresIn <= 0 {
		return Credentials{}, &ProviderError{Code: "invalid_token_expiry"}
	}
	return Credentials{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		ClientID:     clientID,
		Scopes:       response.Scope,
		ExpiresAt:    o.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
	}, nil
}

func (o *OAuth) SubscribeChat(ctx context.Context, accessToken, clientID, sessionID, broadcasterUserID string) error {
	payload := map[string]any{
		"type":    "channel.chat.message",
		"version": "1",
		"condition": map[string]string{
			"broadcaster_user_id": broadcasterUserID,
			"user_id":             broadcasterUserID,
		},
		"transport": map[string]string{
			"method":     "websocket",
			"session_id": sessionID,
		},
	}
	body, _ := json.Marshal(payload)
	headers := helixHeaders(accessToken, clientID)
	headers.Set("Content-Type", "application/json")
	resp, err := o.do(ctx, http.MethodPost, strings.TrimRight(o.Endpoints.Helix, "/")+"/eventsub/subscriptions", body, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return providerErrorFromBody(resp.StatusCode, nil)
	}
	return nil
}

func (o *OAuth) SendChat(ctx context.Context, creds Credentials, broadcasterUserID, message string) error {
	if message == "" {
		return &ProviderError{Code: "empty_message"}
	}
	// Helix's exhausted bucket also applies after a successful send. Remember
	// the deadline here so the next queue item cannot bypass the reset header.
	o.sendMu.Lock()
	defer o.sendMu.Unlock()
	if delay := o.nextSend.Sub(o.Now()); delay > 0 {
		if err := o.Sleep(ctx, delay); err != nil {
			return err
		}
	}
	payload, _ := json.Marshal(map[string]string{
		"broadcaster_id": broadcasterUserID,
		"sender_id":      creds.UserID,
		"message":        message,
	})
	headers := helixHeaders(creds.AccessToken, creds.ClientID)
	headers.Set("Content-Type", "application/json")
	resp, err := o.do(ctx, http.MethodPost, strings.TrimRight(o.Endpoints.Helix, "/")+"/chat/messages", payload, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	delay := providerDelay(resp, o.Now())
	if until := o.Now().Add(delay); until.After(o.nextSend) {
		o.nextSend = until
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return &ProviderError{Code: "invalid_provider_response"}
	}
	if resp.StatusCode != http.StatusOK {
		err := providerErrorFromBody(resp.StatusCode, body)
		var providerErr *ProviderError
		if errors.As(err, &providerErr) {
			providerErr.RetryAfter = delay
		}
		return err
	}
	var result struct {
		Data []struct {
			IsSent     bool `json:"is_sent"`
			DropReason struct {
				Code string `json:"code"`
			} `json:"drop_reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Data) == 0 {
		return &ProviderError{Code: "invalid_provider_response"}
	}
	if !result.Data[0].IsSent {
		code := result.Data[0].DropReason.Code
		if code == "" {
			code = "message_not_sent"
		}
		return &ProviderError{Code: code, Status: resp.StatusCode}
	}
	return nil
}

func (o *OAuth) chatDelay() time.Duration {
	o.sendMu.Lock()
	defer o.sendMu.Unlock()
	if delay := o.nextSend.Sub(o.Now()); delay > 0 {
		return delay
	}
	return 0
}

func (o *OAuth) formJSON(ctx context.Context, method, endpoint string, form url.Values, headers http.Header, out any) error {
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.do(ctx, method, endpoint, []byte(form.Encode()), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return &ProviderError{Code: "invalid_provider_response"}
	}
	if resp.StatusCode != http.StatusOK {
		return providerErrorFromBody(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &ProviderError{Code: "invalid_provider_response"}
	}
	return nil
}

func (o *OAuth) do(ctx context.Context, method, endpoint string, body []byte, headers http.Header) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, &ProviderError{Code: "invalid_endpoint"}
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp, err := o.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &ProviderError{Code: "network"}
	}
	return resp, nil
}

func helmetHeaders() http.Header { return http.Header{} }

func helixHeaders(accessToken, clientID string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+accessToken)
	h.Set("Client-Id", clientID)
	return h
}

func providerErrorFromBody(status int, body []byte) error {
	code := "provider_error"
	if len(body) > 0 {
		var payload struct {
			Message string `json:"message"`
			Status  int    `json:"status"`
		}
		if json.Unmarshal(body, &payload) == nil {
			switch strings.ToLower(strings.TrimSpace(payload.Message)) {
			case "authorization_pending", "slow_down", "access_denied", "expired_token":
				code = strings.ToLower(strings.TrimSpace(payload.Message))
			case "invalid_grant", "invalid refresh token", "invalid refresh token.":
				code = "invalid_grant"
			}
		}
	}
	return &ProviderError{Code: code, Status: status}
}

func retryAfter(resp *http.Response) time.Duration {
	return providerDelay(resp, time.Now())
}

// Supported Helix metadata: Retry-After (seconds or HTTP date), and
// Ratelimit-Remaining=0 with Ratelimit-Reset (Unix seconds). Use the stricter
// deadline and never trust malformed or already elapsed values.
func providerDelay(resp *http.Response, now time.Time) time.Duration {
	if resp == nil {
		return 0
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	var delay time.Duration
	if seconds, err := time.ParseDuration(value + "s"); err == nil && seconds > 0 {
		delay = seconds
	} else if deadline, err := http.ParseTime(value); err == nil && deadline.After(now) {
		delay = deadline.Sub(now)
	}
	if strings.TrimSpace(resp.Header.Get("Ratelimit-Remaining")) == "0" {
		if epoch, err := strconv.ParseInt(strings.TrimSpace(resp.Header.Get("Ratelimit-Reset")), 10, 64); err == nil {
			if reset := time.Unix(epoch, 0).Sub(now); reset > delay {
				delay = reset
			}
		}
	}
	return delay
}

func ValidClientID(clientID string) bool {
	if len(clientID) == 0 || len(clientID) > 64 {
		return false
	}
	for _, r := range clientID {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func hasAllScopes(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, scope := range have {
		set[scope] = true
	}
	for _, scope := range want {
		if !set[scope] {
			return false
		}
	}
	return true
}
