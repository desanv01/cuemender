package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/httpui"
	"github.com/desanv01/cuemender/internal/secrets"
	"github.com/desanv01/cuemender/internal/twitch"
)

type fixtureOSStore struct {
	*secrets.Memory
	available bool
}

func (s *fixtureOSStore) Available(context.Context) bool { return s.available }

func privateRequest(t *testing.T, a *App, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	boot := httptest.NewRecorder()
	token, _ := json.Marshal(map[string]string{"token": a.ui.BootstrapToken()})
	req := httptest.NewRequest(http.MethodPost, a.BaseURL()+"/api/v1/session/bootstrap", strings.NewReader(string(token)))
	req.Header.Set("Origin", a.BaseURL())
	req.Header.Set("Content-Type", "application/json")
	a.ui.Handler().ServeHTTP(boot, req)
	if boot.Code != http.StatusOK {
		t.Fatalf("private bootstrap status=%d", boot.Code)
	}
	var auth struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(boot.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, a.BaseURL()+path, strings.NewReader(body))
	req.Header.Set("Origin", a.BaseURL())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpui.CSRFHeader, auth.Data.CSRF)
	for _, cookie := range boot.Result().Cookies() {
		req.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	a.ui.Handler().ServeHTTP(response, req)
	return response
}

func TestAppStartupCredentialBoundaries(t *testing.T) {
	for _, scenario := range []string{"saved", "missing", "malformed", "unavailable", "explicit_memory"} {
		t.Run(scenario, func(t *testing.T) {
			var validation, subscriptions, unexpected, osFactories atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/validate":
					validation.Add(1)
					if r.Header.Get("Authorization") != "OAuth APP-FIXTURE-ACCESS" {
						t.Error("startup validation used an unexpected credential")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "fixtureclient", "user_id": "fixture-user", "login": "fixture", "scopes": []string{twitch.ScopeReadChat, twitch.ScopeWriteChat}, "expires_in": 3600})
				case "/helix/eventsub/subscriptions":
					subscriptions.Add(1)
					var body struct {
						Condition map[string]string `json:"condition"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body.Condition["broadcaster_user_id"] != "fixture-user" || body.Condition["user_id"] != "fixture-user" {
						t.Error("startup subscription lost own-channel binding")
					}
					w.WriteHeader(http.StatusAccepted)
				case "/events":
					conn, err := websocket.Accept(w, r, nil)
					if err != nil {
						return
					}
					defer conn.CloseNow()
					_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"metadata":{"message_id":"startup","message_type":"session_welcome"},"payload":{"session":{"id":"fixture-session","keepalive_timeout_seconds":10}}}`))
					_, _, _ = conn.Read(r.Context())
				default:
					unexpected.Add(1)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			memory := secrets.NewMemory()
			if scenario == "saved" || scenario == "explicit_memory" {
				if err := twitch.NewCredentialStore(memory).Save(context.Background(), twitch.Credentials{AccessToken: "APP-FIXTURE-ACCESS", RefreshToken: "APP-FIXTURE-REFRESH", ClientID: "fixtureclient", UserID: "fixture-user"}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "malformed" {
				_ = memory.Set(context.Background(), "twitch-credentials-v1", "{")
			}
			dataDir := t.TempDir()
			cfg := DefaultConfig()
			cfg.TwitchClientID = "fixtureclient"
			cfg.OBSAddress = "ws" + strings.TrimPrefix(server.URL, "http") + "/obs"
			// OBS accepts no path; the same fixture port is sufficient to detect
			// an accidental startup dial, without contacting a real OBS instance.
			cfg.OBSAddress = strings.TrimSuffix(cfg.OBSAddress, "/obs")
			if err := SaveConfig(dataDir, cfg); err != nil {
				t.Fatal(err)
			}
			a, err := newWithDependencies(context.Background(), Options{DataDir: dataDir, ListenAddress: fmt.Sprintf("127.0.0.1:%d", freePort(t)), NoBrowser: true, SessionOnlySecrets: scenario == "explicit_memory", Stdout: io.Discard, Stderr: io.Discard}, appDependencies{
				newOSStore: func() osSecretStore {
					osFactories.Add(1)
					return &fixtureOSStore{Memory: memory, available: scenario != "unavailable"}
				},
				configureTwitch: func(opts *twitch.Options) {
					opts.Endpoints = twitch.Endpoints{Device: server.URL + "/device", Token: server.URL + "/token", Validate: server.URL + "/validate", Helix: server.URL + "/helix", EventSub: "ws" + strings.TrimPrefix(server.URL, "http") + "/events"}
					opts.OAuth = twitch.NewOAuth(server.Client(), opts.Endpoints, opts.Now)
					opts.Dialer = twitch.CoderDialer{HTTP: server.Client()}
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			runDone := make(chan error, 1)
			go func() { runDone <- a.Run(ctx) }()
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-runDone:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(8 * time.Second):
					t.Error("startup fixture did not shut down")
				}
				a.Close()
			})
			want := "not_authorized"
			if scenario == "saved" {
				want = "connected"
			}
			if scenario == "malformed" {
				want = "authorization_required"
			}
			if scenario == "unavailable" {
				want = "credential_storage_unavailable"
			}
			deadline := time.Now().Add(3 * time.Second)
			for a.twitchMgr.Snapshot().DetailCode != want && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			state, _ := a.PrivateSnapshot(ctx)
			if state.Twitch.DetailCode != want {
				t.Fatalf("startup=%s/%s, expected %s", state.Twitch.Status, state.Twitch.DetailCode, want)
			}
			if state.OBS.Status != domain.ConnectionNotConnected || unexpected.Load() != 0 {
				t.Fatal("startup began OBS/device work without explicit setup")
			}
			if scenario == "saved" {
				if validation.Load() < 2 || subscriptions.Load() != 1 {
					t.Fatal("saved authorization did not validate and subscribe through app startup")
				}
			} else if validation.Load() != 0 || subscriptions.Load() != 0 {
				t.Fatal("unusable startup credential reached the provider")
			}
			if scenario == "explicit_memory" && (osFactories.Load() != 0 || state.Secrets.Mode != domain.SecretsModeMemory) {
				t.Fatal("explicit session-only mode consulted OS storage")
			}
			if scenario == "unavailable" {
				response := privateRequest(t, a, "/api/v1/twitch/connect", `{"client_id":"differentclient"}`)
				if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "credential_storage_unavailable") {
					t.Fatal("unavailable storage was not refused by authenticated setup")
				}
			}
			data, _ := json.Marshal(state)
			config, _ := os.ReadFile(filepath.Join(dataDir, configFileName))
			if strings.Contains(string(data)+string(config), "APP-FIXTURE-") {
				t.Fatal("credential escaped startup secret boundary")
			}
		})
	}
}
