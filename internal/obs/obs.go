package obs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andreykaipov/goobs"
	"github.com/andreykaipov/goobs/api/requests/config"
	"github.com/andreykaipov/goobs/api/requests/general"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/andreykaipov/goobs/api/requests/scenes"
	"github.com/andreykaipov/goobs/api/requests/stream"
	"github.com/andreykaipov/goobs/api/typedefs"
	gorilla "github.com/gorilla/websocket"

	"github.com/desanv01/cuemender/internal/domain"
)

type VersionInfo struct {
	OBSVersion          string
	ObsWebSocketVersion string
	RPCVersion          float64
}

// Backend is a read-only OBS seam. It deliberately has no mutation methods.
type Backend interface {
	Version(ctx context.Context) (VersionInfo, error)
	Inputs(ctx context.Context) ([]domain.OBSInput, error)
	InputMute(ctx context.Context, ref domain.InputRef) (*bool, error)
	InputAudioTracks(ctx context.Context, ref domain.InputRef) (map[string]bool, error)
	ProgramScene(ctx context.Context) (name, uuid string, err error)
	Scenes(ctx context.Context) ([]domain.OBSScene, error)
	SceneItems(ctx context.Context, sceneUUID string) ([]domain.OBSSceneItem, error)
	SceneItemEnabled(ctx context.Context, sceneUUID string, itemID int) (*bool, error)
	StreamStatus(ctx context.Context) (*bool, error)
	Close() error
}

type Dialer interface {
	Connect(ctx context.Context, endpoint, password string) (Backend, error)
}

type GoobsDialer struct{}

func (GoobsDialer) Connect(ctx context.Context, endpoint, password string) (Backend, error) {
	dialer := *gorilla.DefaultDialer
	dialer.HandshakeTimeout = 2 * time.Second
	client, err := goobs.New(endpoint,
		goobs.WithContext(ctx),
		goobs.WithDialer(&dialer),
		goobs.WithPassword(password),
		goobs.WithResponseTimeoutDuration(5*time.Second),
		goobs.WithWriteTimeoutDuration(2*time.Second),
		goobs.WithEventSubscriptions(0),
		goobs.WithLogger(silentLogger{}),
	)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = client.Disconnect()
		return nil, err
	}
	return &goobsBackend{client: client, gate: make(chan struct{}, 1)}, nil
}

// silentLogger keeps provider payloads out of process logs. goobs defaults to
// stderr logging, which would bypass the application's redaction boundary.
type silentLogger struct{}

func (silentLogger) Printf(string, ...any) {}

// callRead admits only one SDK call and joins it after cancellation. The
// patched pinned SDK makes Disconnect interrupt authentication and requests.
func (b *goobsBackend) callRead(ctx context.Context, fn func() (any, error)) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case b.gate <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	defer func() { <-b.gate }()
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, ErrUnavailable
	}
	b.active.Add(1)
	b.mu.Unlock()
	defer b.active.Done()
	type result struct {
		value any
		err   error
	}
	done := make(chan result, 1)
	go func() { value, err := fn(); done <- result{value, err} }()
	select {
	case got := <-done:
		return got.value, got.err
	case <-ctx.Done():
		b.stop()
		<-done
		return nil, ctx.Err()
	}
}

type goobsBackend struct {
	client   *goobs.Client
	gate     chan struct{}
	mu       sync.Mutex
	closed   bool
	stopOnce sync.Once
	active   sync.WaitGroup
}

func (b *goobsBackend) stop() {
	b.stopOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()
		_ = b.client.Disconnect()
	})
}

func (b *goobsBackend) Version(ctx context.Context) (VersionInfo, error) {
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.General.GetVersion()
	})
	if err != nil {
		return VersionInfo{}, err
	}
	response := value.(*general.GetVersionResponse)
	return VersionInfo{
		OBSVersion:          response.ObsVersion,
		ObsWebSocketVersion: response.ObsWebSocketVersion,
		RPCVersion:          response.RpcVersion,
	}, nil
}

func (b *goobsBackend) Inputs(ctx context.Context) ([]domain.OBSInput, error) {
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.Inputs.GetInputList()
	})
	if err != nil {
		return nil, err
	}
	response := value.(*inputs.GetInputListResponse)
	out := make([]domain.OBSInput, 0, len(response.Inputs))
	for _, input := range response.Inputs {
		if input == nil {
			continue
		}
		out = append(out, domain.OBSInput{Name: input.InputName, UUID: input.InputUuid, Kind: input.InputKind})
	}
	return out, nil
}

func inputParams(ref domain.InputRef) (*inputs.GetInputMuteParams, *inputs.GetInputAudioTracksParams) {
	mute := inputs.NewGetInputMuteParams()
	tracks := inputs.NewGetInputAudioTracksParams()
	if ref.UUID != "" {
		mute.WithInputUuid(ref.UUID)
		tracks.WithInputUuid(ref.UUID)
	} else {
		mute.WithInputName(ref.Name)
		tracks.WithInputName(ref.Name)
	}
	return mute, tracks
}

func (b *goobsBackend) InputMute(ctx context.Context, ref domain.InputRef) (*bool, error) {
	params, _ := inputParams(ref)
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.Inputs.GetInputMute(params)
	})
	if err != nil {
		return nil, err
	}
	response := value.(*inputs.GetInputMuteResponse)
	result := response.InputMuted
	return &result, nil
}

func (b *goobsBackend) InputAudioTracks(ctx context.Context, ref domain.InputRef) (map[string]bool, error) {
	_, params := inputParams(ref)
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.Inputs.GetInputAudioTracks(params)
	})
	if err != nil {
		return nil, err
	}
	response := value.(*inputs.GetInputAudioTracksResponse)
	if response.InputAudioTracks == nil {
		return nil, nil
	}
	// Typed map decoding turns JSON null into false. Inspect the raw shape
	// so incomplete/malformed wire evidence cannot authorize a repair.
	var raw struct {
		Tracks map[string]*bool `json:"inputAudioTracks"`
	}
	if err := json.Unmarshal(response.GetRaw(), &raw); err != nil {
		return nil, nil
	}
	out := make(map[string]bool, len(raw.Tracks))
	for key, value := range raw.Tracks {
		domainKey := normalizeWireTrackKey(key)
		if domainKey == "" || value == nil {
			return nil, nil
		}
		if _, duplicate := out[domainKey]; duplicate {
			return nil, nil
		}
		out[domainKey] = *value
	}
	return out, nil
}

func (b *goobsBackend) setInputAudioTracks(ctx context.Context, ref domain.InputRef, tracks map[string]bool) error {
	if !exactTracks(tracks) || ref.Name == "" || ref.Context == "" {
		return ErrRepairPrecondition
	}
	wire := make(typedefs.InputAudioTracks, 6)
	for key, value := range tracks {
		wire[WireTrackKey(key)] = value
	}
	params := inputs.NewSetInputAudioTracksParams().WithInputAudioTracks(&wire)
	if ref.UUID != "" {
		params.WithInputUuid(ref.UUID)
	} else {
		params.WithInputName(ref.Name)
	}
	_, err := b.callRead(ctx, func() (any, error) { return b.client.Inputs.SetInputAudioTracks(params) })
	return err
}

// normalizeWireTrackKey converts OBS's numeric "1".."6" keys into the stable
// domain convention track1..track6. P04 must convert back with WireTrackKey.
func normalizeWireTrackKey(key string) string {
	if key == "" {
		return ""
	}
	if key[0] >= '1' && key[0] <= '6' && len(key) == 1 {
		return "track" + key
	}
	if len(key) == 6 && key[0:5] == "track" && key[5] >= '1' && key[5] <= '6' {
		return key
	}
	return ""
}

// WireTrackKey maps a domain track key back to the numeric OBS wire key.
func WireTrackKey(domainKey string) string {
	if len(domainKey) == 6 && domainKey[0:5] == "track" && domainKey[5] >= '1' && domainKey[5] <= '6' {
		return domainKey[5:]
	}
	return ""
}

func (b *goobsBackend) SceneCollection(ctx context.Context) (string, error) {
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.Config.GetSceneCollectionList()
	})
	if err != nil {
		return "", err
	}
	return value.(*config.GetSceneCollectionListResponse).CurrentSceneCollectionName, nil
}

func (b *goobsBackend) ProgramScene(ctx context.Context) (string, string, error) {
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.Scenes.GetCurrentProgramScene()
	})
	if err != nil {
		return "", "", err
	}
	response := value.(*scenes.GetCurrentProgramSceneResponse)
	name := response.SceneName
	if name == "" {
		name = response.CurrentProgramSceneName
	}
	uuid := response.SceneUuid
	if uuid == "" {
		uuid = response.CurrentProgramSceneUuid
	}
	return name, uuid, nil
}

func (b *goobsBackend) Scenes(ctx context.Context) ([]domain.OBSScene, error) {
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.Scenes.GetSceneList()
	})
	if err != nil {
		return nil, err
	}
	response := value.(*scenes.GetSceneListResponse)
	out := make([]domain.OBSScene, 0, len(response.Scenes))
	for _, scene := range response.Scenes {
		if scene == nil {
			continue
		}
		out = append(out, domain.OBSScene{Name: scene.SceneName, UUID: scene.SceneUuid})
	}
	return out, nil
}

func (b *goobsBackend) SceneItems(ctx context.Context, sceneUUID string) ([]domain.OBSSceneItem, error) {
	params := sceneitems.NewGetSceneItemListParams()
	if sceneUUID != "" {
		params.WithSceneUuid(sceneUUID)
	}
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.SceneItems.GetSceneItemList(params)
	})
	if err != nil {
		return nil, err
	}
	response := value.(*sceneitems.GetSceneItemListResponse)
	out := make([]domain.OBSSceneItem, 0, len(response.SceneItems))
	for _, item := range response.SceneItems {
		if item == nil {
			continue
		}
		out = append(out, domain.OBSSceneItem{
			ID:         item.SceneItemID,
			SourceName: item.SourceName,
			SourceUUID: item.SourceUuid,
			Enabled:    item.SceneItemEnabled,
			IsGroup:    item.IsGroup,
		})
	}
	return out, nil
}

func (b *goobsBackend) SceneItemEnabled(ctx context.Context, sceneUUID string, itemID int) (*bool, error) {
	params := sceneitems.NewGetSceneItemEnabledParams().WithSceneItemId(itemID)
	if sceneUUID != "" {
		params.WithSceneUuid(sceneUUID)
	}
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.SceneItems.GetSceneItemEnabled(params)
	})
	if err != nil {
		return nil, err
	}
	response := value.(*sceneitems.GetSceneItemEnabledResponse)
	result := response.SceneItemEnabled
	return &result, nil
}

func (b *goobsBackend) StreamStatus(ctx context.Context) (*bool, error) {
	value, err := b.callRead(ctx, func() (any, error) {
		return b.client.Stream.GetStreamStatus()
	})
	if err != nil {
		return nil, err
	}
	response := value.(*stream.GetStreamStatusResponse)
	result := response.OutputActive
	return &result, nil
}

func (b *goobsBackend) Close() error {
	b.stop()
	b.active.Wait()
	return nil
}

// ValidateEndpoint applies the same loopback ws/wss constraints as startup
// config before any OBS dial is attempted.
func ValidateEndpoint(address string) error {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		return errors.New("OBS endpoint must use ws:// or wss://")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("OBS endpoint must be a plain loopback websocket address")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return errors.New("OBS endpoint must target a loopback IP")
	}
	if u.Port() == "" {
		return errors.New("OBS endpoint requires a port")
	}
	if strings.Contains(address, " ") {
		return errors.New("OBS endpoint must not contain spaces")
	}
	return nil
}

// The compatibility test references these SDK symbols directly.
var (
	_ = typedefs.Input{}
	_ = typedefs.SceneItem{}
	_ = inputs.NewGetInputMuteParams
	_ = scenes.NewGetSceneListParams
	_ = sceneitems.NewGetSceneItemListParams
	_ = &stream.GetStreamStatusParams{}
)
