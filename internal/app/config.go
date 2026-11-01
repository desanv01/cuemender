package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ConfigSchemaVersion  = 1
	DefaultListenAddress = "127.0.0.1:8787"
	DefaultOBSAddress    = "ws://127.0.0.1:4455"
	configFileName       = "config.json"
	databaseFileName     = "livecheck.db"
)

var clientIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`)

// Config is the non-secret startup configuration stored in dataDir/config.json.
type Config struct {
	SchemaVersion  int    `json:"schema_version"`
	ListenAddress  string `json:"listen_address"`
	TwitchClientID string `json:"twitch_client_id,omitempty"`
	OBSAddress     string `json:"obs_address"`
}

func DefaultConfig() Config {
	return Config{
		SchemaVersion: ConfigSchemaVersion,
		ListenAddress: DefaultListenAddress,
		OBSAddress:    DefaultOBSAddress,
	}
}

// LoadConfig reads dataDir/config.json. A missing file returns defaults and
// does not create or modify anything. Malformed or invalid configuration is
// returned as an error and the file is left untouched.
func LoadConfig(dataDir string) (Config, error) {
	path := filepath.Join(dataDir, configFileName)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	dec := json.NewDecoder(io.LimitReader(f, 64<<10))
	dec.DisallowUnknownFields()
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := ValidateConfig(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return cfg, nil
}

func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}

var saveConfigMu sync.Mutex

// replaceConfigFile is an injectable boundary for deterministic filesystem
// failure tests; production uses same-directory rename.
var replaceConfigFile = os.Rename

// SaveConfig validates and atomically replaces dataDir/config.json. The
// temporary file is written, synced and closed in the same directory before
// replacement, so a failure leaves the previous valid file intact.
func SaveConfig(dataDir string, cfg Config) error {
	saveConfigMu.Lock()
	defer saveConfigMu.Unlock()
	if err := ValidateConfig(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	tmp, err := os.CreateTemp(dataDir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	path := filepath.Join(dataDir, configFileName)
	var renameErr error
	for attempt := 0; attempt < 20; attempt++ {
		renameErr = replaceConfigFile(tmpPath, path)
		if renameErr == nil {
			return nil
		}
		// Windows can transiently deny replacement while a reader/AV scanner
		// still has the prior file open. Retry briefly without weakening the
		// atomic same-directory replacement.
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return fmt.Errorf("replace config: %w", renameErr)
}

func ValidateConfig(cfg Config) error {
	if cfg.SchemaVersion != ConfigSchemaVersion {
		return fmt.Errorf("unsupported config schema_version %d (this build supports %d)", cfg.SchemaVersion, ConfigSchemaVersion)
	}
	if err := ValidateListenAddress(cfg.ListenAddress); err != nil {
		return err
	}
	if err := ValidateOBSAddress(cfg.OBSAddress); err != nil {
		return err
	}
	if cfg.TwitchClientID != "" && !clientIDPattern.MatchString(cfg.TwitchClientID) {
		return errors.New("twitch_client_id must be 1-64 alphanumeric characters")
	}
	return nil
}

// ValidateListenAddress enforces an explicit loopback IP and a real port.
func ValidateListenAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("listen address must be host:port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("listen address must be an explicit loopback IP (127.0.0.1 or ::1)")
	}
	if err := validatePort(port); err != nil {
		return fmt.Errorf("listen address: %w", err)
	}
	return nil
}

// ValidateOBSAddress accepts a local obs-websocket endpoint without userinfo,
// query, fragment or path. P01 stores it but never connects.
func ValidateOBSAddress(address string) error {
	u, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("obs_address is not a valid URL: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return errors.New("obs_address must use ws:// or wss://")
	}
	if u.User != nil {
		return errors.New("obs_address must not contain URL credentials")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("obs_address must not contain a query or fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return errors.New("obs_address must not contain a path")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("obs_address must target an explicit loopback IP")
	}
	if err := validatePort(u.Port()); err != nil {
		return fmt.Errorf("obs_address: %w", err)
	}
	return nil
}

func validatePort(port string) error {
	if port == "" {
		return errors.New("port is required")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

// ResolveDataDir returns an absolute data directory, creating it if needed.
// Spaces and Unicode are supported.
func ResolveDataDir(explicit string) (string, error) {
	if explicit == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		explicit = filepath.Join(base, "LiveCheck")
	}
	abs, err := filepath.Abs(explicit)
	if err != nil {
		return "", fmt.Errorf("resolve data directory: %w", err)
	}
	if strings.TrimSpace(abs) == "" {
		return "", errors.New("data directory is empty")
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}
	return abs, nil
}
