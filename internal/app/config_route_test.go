package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAuthenticatedSetupReplacementFailurePreservesAppAndFile(t *testing.T) {
	for _, provider := range []string{"twitch", "obs"} {
		t.Run(provider, func(t *testing.T) {
			dataDir := t.TempDir()
			original := DefaultConfig()
			original.TwitchClientID = "previousclient"
			if err := SaveConfig(dataDir, original); err != nil {
				t.Fatal(err)
			}
			a, err := New(context.Background(), Options{DataDir: dataDir, ListenAddress: fmt.Sprintf("127.0.0.1:%d", freePort(t)), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			before, _ := a.PrivateSnapshot(context.Background())
			beforeConfig, beforeFileConfig := a.cfg, a.fileConfig
			path := filepath.Join(dataDir, configFileName)
			beforeBytes, _ := os.ReadFile(path)
			sub := a.SubscribePrivate()
			defer sub.Cancel()
			<-sub.C
			replace := replaceConfigFile
			replaceConfigFile = func(string, string) error { return errors.New("fixture replacement error") }
			defer func() { replaceConfigFile = replace }()
			body := `{"client_id":"replacementclient"}`
			if provider == "obs" {
				body = `{"address":"ws://127.0.0.1:4456","password":"SYNTHETIC-OBS-SECRET"}`
			}
			response := privateRequest(t, a, "/api/v1/"+provider+"/connect", body)
			if response.Code < 400 || !strings.Contains(response.Body.String(), `"error"`) {
				t.Fatalf("failed replacement route returned status %d", response.Code)
			}
			afterBytes, err := os.ReadFile(path)
			if err != nil || string(afterBytes) != string(beforeBytes) {
				t.Fatal("failed setup changed prior config bytes")
			}
			if a.cfg != beforeConfig || a.fileConfig != beforeFileConfig {
				t.Fatal("failed setup changed effective or persisted config cache")
			}
			after, _ := a.PrivateSnapshot(context.Background())
			if !reflect.DeepEqual(before, after) {
				t.Fatal("failed setup changed its private projection or started a provider lifecycle")
			}
			select {
			case <-sub.C:
				t.Fatal("failed setup published a success/state update")
			default:
			}
			entries, _ := os.ReadDir(dataDir)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".config-") {
					t.Fatal("failed replacement left a temporary config file")
				}
			}
		})
	}
}
