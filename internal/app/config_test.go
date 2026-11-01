package app

import (
	"os"
	"path/filepath"
	"strings"
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
