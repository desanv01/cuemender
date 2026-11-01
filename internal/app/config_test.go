package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLoadConfigMissingUsesDefaultsWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultConfig() {
		t.Fatalf("cfg = %+v, want defaults", cfg)
	}
	if _, err := os.Stat(filepath.Join(dir, configFileName)); !os.IsNotExist(err) {
		t.Fatalf("missing config was unexpectedly created: %v", err)
	}
}

func TestLoadConfigMalformedIsPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	original := []byte(`{"schema_version":1,"listen_address":`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("expected malformed config error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("malformed config was modified: %q", after)
	}
}

func TestLoadConfigRejectsUnknownSecretField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"listen_address":"127.0.0.1:8787","obs_address":"ws://127.0.0.1:4455","twitch_client_secret":"nope"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown field rejection", err)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := DefaultConfig()
	if err := ValidateConfig(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := []struct {
		name string
		cfg  Config
	}{
		{"bad schema", Config{SchemaVersion: 2, ListenAddress: "127.0.0.1:8787", OBSAddress: "ws://127.0.0.1:4455"}},
		{"wildcard listen", Config{SchemaVersion: 1, ListenAddress: "0.0.0.0:8787", OBSAddress: "ws://127.0.0.1:4455"}},
		{"remote obs", Config{SchemaVersion: 1, ListenAddress: "127.0.0.1:8787", OBSAddress: "ws://192.168.1.20:4455"}},
		{"bad client id", Config{SchemaVersion: 1, ListenAddress: "127.0.0.1:8787", OBSAddress: "ws://127.0.0.1:4455", TwitchClientID: "has space"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateConfig(tc.cfg); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestValidateListenAddress(t *testing.T) {
	valid := []string{"127.0.0.1:8787", "[::1]:8787"}
	for _, address := range valid {
		if err := ValidateListenAddress(address); err != nil {
			t.Errorf("%s rejected: %v", address, err)
		}
	}
	invalid := []string{"0.0.0.0:8787", "localhost:8787", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:70000"}
	for _, address := range invalid {
		if err := ValidateListenAddress(address); err == nil {
			t.Errorf("%s accepted", address)
		}
	}
}

func TestValidateOBSAddress(t *testing.T) {
	valid := []string{"ws://127.0.0.1:4455", "wss://[::1]:4455"}
	for _, address := range valid {
		if err := ValidateOBSAddress(address); err != nil {
			t.Errorf("%s rejected: %v", address, err)
		}
	}
	invalid := []string{
		"http://127.0.0.1:4455",
		"ws://localhost:4455",
		"ws://192.168.1.2:4455",
		"ws://user:pass@127.0.0.1:4455",
		"ws://127.0.0.1:4455/path",
		"ws://127.0.0.1:4455?x=1",
		"ws://127.0.0.1",
	}
	for _, address := range invalid {
		if err := ValidateOBSAddress(address); err == nil {
			t.Errorf("%s accepted", address)
		}
	}
}

func TestResolveDataDirSupportsSpaceAndUnicode(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "space ü", "nested")
	got, err := ResolveDataDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
	info, err := os.Stat(got)
	if err != nil || !info.IsDir() {
		t.Fatalf("data directory not created: %v", err)
	}
}

func TestSaveConfigAtomicRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.TwitchClientID = "publicclient123"
	cfg.OBSAddress = "ws://127.0.0.1:4456"
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != cfg {
		t.Fatalf("loaded = %+v, want %+v", loaded, cfg)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".config-") {
			t.Fatalf("temporary config file left behind: %s", entry.Name())
		}
	}
}

func TestSaveConfigInvalidPreservesPrior(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	bad := cfg
	bad.SchemaVersion = 99
	if err := SaveConfig(dir, bad); err == nil {
		t.Fatal("invalid config was saved")
	}
	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != cfg {
		t.Fatalf("prior config changed: %+v", loaded)
	}
}

func TestSaveConfigReplacementFailurePreservesBytes(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, configFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original := replaceConfigFile
	replaceConfigFile = func(string, string) error { return errors.New("injected replacement failure") }
	t.Cleanup(func() { replaceConfigFile = original })
	changed := cfg
	changed.TwitchClientID = "changedclient"
	if err := SaveConfig(dir, changed); err == nil {
		t.Fatal("injected replacement failure was reported as success")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("prior config bytes changed after replacement failure")
	}
	if loaded, err := LoadConfig(dir); err != nil || loaded.TwitchClientID != "" {
		t.Fatalf("failed save was observable in config: %+v, %v", loaded, err)
	}
}

func TestSaveConfigConcurrentUpdatesStayValid(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := DefaultConfig()
			cfg.TwitchClientID = fmt.Sprintf("client%d", i)
			if err := SaveConfig(dir, cfg); err != nil {
				t.Errorf("save %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("final config invalid: %v", err)
	}
	if !strings.HasPrefix(loaded.TwitchClientID, "client") {
		t.Fatalf("unexpected final client id %q", loaded.TwitchClientID)
	}
}

func TestAppPersistConfigKeepsCLIListenOutOfFile(t *testing.T) {
	port := 0
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port = ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	dataDir := t.TempDir()
	a, err := New(context.Background(), Options{
		DataDir:            dataDir,
		ListenAddress:      fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser:          true,
		SessionOnlySecrets: true,
		Stdout:             io.Discard,
		Stderr:             io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.persistConfig(func(cfg *Config) { cfg.TwitchClientID = "client123" }); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TwitchClientID != "client123" {
		t.Fatalf("client id = %q", loaded.TwitchClientID)
	}
	if loaded.ListenAddress != DefaultListenAddress {
		t.Fatalf("CLI listen override leaked into config: %q", loaded.ListenAddress)
	}
	if err := a.persistConfig(func(cfg *Config) { cfg.ListenAddress = "0.0.0.0:9999" }); err == nil {
		t.Fatal("invalid config update succeeded")
	}
	still, err := LoadConfig(dataDir)
	if err != nil || still.TwitchClientID != "client123" || still.ListenAddress != DefaultListenAddress {
		t.Fatalf("prior config not preserved: %+v, %v", still, err)
	}
}

func TestAppTwitchConnectConfigFailureDoesNotStartConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	dataDir := t.TempDir()
	a, err := New(context.Background(), Options{
		DataDir:            dataDir,
		ListenAddress:      fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser:          true,
		SessionOnlySecrets: true,
		Stdout:             io.Discard,
		Stderr:             io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	beforeRevision := a.stateRevision
	original := replaceConfigFile
	replaceConfigFile = func(string, string) error { return errors.New("injected route replacement failure") }
	defer func() { replaceConfigFile = original }()
	if err := a.TwitchConnect(context.Background(), "abc123"); err == nil {
		t.Fatal("connect reported success after config persistence failure")
	}
	if a.stateRevision != beforeRevision {
		t.Fatal("failed setup published a state revision")
	}
	if _, err := os.Stat(filepath.Join(dataDir, configFileName)); !os.IsNotExist(err) {
		t.Fatalf("failed setup created a config file: %v", err)
	}
	if a.twitchMgr.Snapshot().Status != "not_connected" {
		t.Fatal("failed setup started a Twitch connection")
	}
}

func TestAppPersistConfigConcurrent(t *testing.T) {
	port := 0
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port = ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	a, err := New(context.Background(), Options{
		DataDir:            t.TempDir(),
		ListenAddress:      fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser:          true,
		SessionOnlySecrets: true,
		Stdout:             io.Discard,
		Stderr:             io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := a.persistConfig(func(cfg *Config) { cfg.TwitchClientID = fmt.Sprintf("client%d", i) }); err != nil {
				t.Errorf("persist %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	loaded, err := LoadConfig(a.DataDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(loaded.TwitchClientID, "client") {
		t.Fatalf("final config = %+v", loaded)
	}
}
