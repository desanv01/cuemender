package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/store"
	"github.com/gorilla/websocket"
)

// Uses App.New's production GoobsDialer and real local HTTP/WebSocket sockets.
func TestRunBlockedOBSProviderShutsDownWithActiveSSE(t *testing.T) {
	for _, phase := range []string{"hello", "identify", "version"} {
		t.Run(phase, func(t *testing.T) {
			entered, peerDone := make(chan struct{}), make(chan struct{})
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer close(peerDone)
				defer conn.Close()
				if phase != "hello" {
					_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"rpcVersion": 1}})
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
				if phase == "version" {
					_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
				close(entered)
				_, _, _ = conn.ReadMessage()
			}))
			defer peer.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dataDir := t.TempDir()
			a, err := New(ctx, Options{DataDir: dataDir, ListenAddress: fmt.Sprintf("127.0.0.1:%d", freePort(t)), NoBrowser: true,
				SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			runDone := make(chan error, 1)
			go func() { runDone <- a.Run(ctx) }()
			waitHealthy(t, a.BaseURL())
			resp, err := http.Get(a.BaseURL() + "/public/v1/events")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("SSE status %d", resp.StatusCode)
			}
			initial, sseDone := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(sseDone)
				scanner := bufio.NewScanner(resp.Body)
				sent := false
				for scanner.Scan() {
					if !sent && strings.HasPrefix(scanner.Text(), "data: ") {
						sent = true
						close(initial)
					}
				}
			}()
			select {
			case <-initial:
			case <-time.After(time.Second):
				t.Fatal("no initial SSE snapshot")
			}
			if err := a.OBSConnect(ctx, "ws"+strings.TrimPrefix(peer.URL, "http"), nil); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("OBS did not reach blocked phase")
			}
			cancel()
			select {
			case err := <-runDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("production App did not join shutdown")
			}
			select {
			case <-peerDone:
			case <-time.After(time.Second):
				t.Fatal("OBS socket/peer remains")
			}
			select {
			case <-sseDone:
			case <-time.After(time.Second):
				t.Fatal("SSE remains")
			}
			addr := strings.TrimPrefix(a.BaseURL(), "http://")
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				t.Fatalf("listener not released: %v", err)
			}
			_ = ln.Close()
			db, err := store.Open(context.Background(), filepath.Join(dataDir, databaseFileName))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.LoadSettings(context.Background()); err != nil {
				_ = db.Close()
				t.Fatal(fmt.Errorf("reopen settings: %w", err))
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
