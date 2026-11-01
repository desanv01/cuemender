package app

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/store"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func waitHealthy(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("healthz did not become ready")
}

func TestPortConflictFailsBeforeDatabaseOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	dataDir := t.TempDir()
	_, err = New(context.Background(), Options{
		DataDir:            dataDir,
		ListenAddress:      fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser:          true,
		SessionOnlySecrets: true,
		Stdout:             io.Discard,
		Stderr:             io.Discard,
	})
	if err == nil {
		t.Fatal("expected a port-conflict error")
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, databaseFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("database was opened despite early listen failure: %v", statErr)
	}
}

func TestRunServesAndShutsDownWithActiveSSE(t *testing.T) {
	port := freePort(t)
	var stdout, stderr bytes.Buffer
	opts := Options{
		DataDir:            t.TempDir(),
		ListenAddress:      fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser:          true,
		SessionOnlySecrets: true,
		Stdout:             &stdout,
		Stderr:             &stderr,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if a.httpServer != nil {
			_ = a.httpServer.Close()
		}
		a.Close()
	})
	if a.ControlURL() == "" || !strings.Contains(a.ControlURL(), "/control/#bootstrap=") {
		t.Fatal("control URL was not generated")
	}
	runErr := make(chan error, 1)
	go func() { runErr <- a.Run(ctx) }()
	waitHealthy(t, a.BaseURL())

	state, err := a.PrivateSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Secrets.Mode != "memory_only" || !state.Secrets.Available {
		t.Fatalf("unexpected secrets status: %+v", state.Secrets)
	}
	if state.Twitch.Status != "not_connected" || state.OBS.Status != "not_connected" {
		t.Fatalf("P01 reported a connected integration: %+v %+v", state.Twitch, state.OBS)
	}
	public, err := a.PublicSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if public.ServiceStatus != "disconnected" || public.Incidents == nil {
		t.Fatalf("unexpected public state: %+v", public)
	}

	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, a.BaseURL()+"/public/v1/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE status = %d", resp.StatusCode)
	}
	gotData := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		sawData := false
		sent := false
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if sawData && !sent {
					sent = true
					gotData <- true
				}
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				sawData = true
			}
		}
		if !sent {
			gotData <- false
		}
	}()
	select {
	case ok := <-gotData:
		if !ok {
			t.Fatal("SSE closed before sending an initial snapshot")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for SSE initial snapshot")
	}

	// Deliberately keep the SSE request and response open while canceling the
	// application. This is the active-client case; closing reqCtx first would
	// make the test inconclusive.
	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(8 * time.Second):
		// The application's own bounded Shutdown is 5 seconds. Leave a
		// scheduling margin without hiding a stuck handler indefinitely.
		t.Fatal("Run did not shut down with an active SSE client")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("unexpected context state: %v", ctx.Err())
	}
	if strings.Contains(stderr.String(), "http shutdown incomplete") {
		t.Fatalf("shutdown timed out instead of ending the active SSE connection: %s", stderr.String())
	}

	// A released listener and a reopenable database are additional proof that
	// shutdown closed the resources it owned.
	releaseLn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("port was not released after shutdown: %v", err)
	}
	_ = releaseLn.Close()
	reopened, err := store.Open(context.Background(), filepath.Join(opts.DataDir, databaseFileName))
	if err != nil {
		t.Fatalf("database could not be reopened after shutdown: %v", err)
	}
	if _, err := reopened.LoadSettings(context.Background()); err != nil {
		reopened.Close()
		t.Fatalf("reopened database could not load settings: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("reopened database close: %v", err)
	}
}
