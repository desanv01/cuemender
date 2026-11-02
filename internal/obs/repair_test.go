package obs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	gorilla "github.com/gorilla/websocket"
)

// This fixture exercises the pinned SDK's real websocket serialization. It is
// local protocol evidence, not a live OBS test or an application approval test.
type repairProtocol struct {
	mu            sync.Mutex
	tracks        map[string]bool
	name          string
	uuid          string
	collection    string
	writes        []map[string]json.RawMessage
	malformed     any
	ignoreWrite   bool
	withholdReply bool
	withholdRead  bool
	onSet         func()
	setSeen       chan struct{}
	setOnce       sync.Once
}

func newRepairProtocol(t *testing.T) (*repairProtocol, *Manager, domain.TrackRepairRequest) {
	t.Helper()
	p := &repairProtocol{name: "Test mic", uuid: "test-mic-uuid", collection: "Test collection",
		tracks: tracksForMask(0b100010), setSeen: make(chan struct{})}
	server := httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(server.Close)
	backend, err := (GoobsDialer{}).Connect(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), "")
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(Options{Context: context.Background(), Now: time.Now, Dialer: &fakeDialer{}})
	ref := domain.InputRef{Name: p.name, UUID: p.uuid, Context: p.collection}
	now, track := time.Now().UTC(), 1
	m.SetSettings(7, domain.Expectations{Microphone: &ref, StreamTrack: &track, TrackConfirmedAt: &now})
	// Install the authenticated real transport without starting unrelated polls.
	m.mu.Lock()
	m.backend, m.gen, m.workerCtx = backend, 3, m.ctx
	m.conn = domain.OBSConnection{Status: domain.ConnectionConnected, Generation: 3, ConfigRevision: 7}
	m.mu.Unlock()
	t.Cleanup(m.Close)
	return p, m, domain.TrackRepairRequest{ActionID: "test-action", Input: ref,
		Generation: 3, ConfigRevision: 7, ExpectedTracks: cloneTracks(p.tracks), SelectedTrack: 1}
}

func (p *repairProtocol) serve(w http.ResponseWriter, r *http.Request) {
	conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"obsWebSocketVersion": "5.7.3", "rpcVersion": 1}})
	var identify any
	if conn.ReadJSON(&identify) != nil {
		return
	}
	_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
	for {
		var request struct {
			Op int `json:"op"`
			D  struct {
				Type string                     `json:"requestType"`
				ID   string                     `json:"requestId"`
				Data map[string]json.RawMessage `json:"requestData"`
			} `json:"d"`
		}
		if conn.ReadJSON(&request) != nil {
			return
		}
		p.mu.Lock()
		var data any = map[string]any{}
		withhold, onSet := false, (func())(nil)
		switch request.D.Type {
		case "GetSceneCollectionList":
			data = map[string]any{"currentSceneCollectionName": p.collection}
		case "GetInputList":
			data = map[string]any{"inputs": []any{map[string]any{"inputName": p.name, "inputUuid": p.uuid}}}
		case "GetInputAudioTracks":
			withhold = p.withholdRead
			wire := map[string]bool{}
			for key, value := range p.tracks {
				wire[WireTrackKey(key)] = value
			}
			data = map[string]any{"inputAudioTracks": wire}
			if p.malformed != nil {
				data = map[string]any{"inputAudioTracks": p.malformed}
			}
		case "SetInputAudioTracks":
			p.writes = append(p.writes, request.D.Data)
			var wire map[string]bool
			_ = json.Unmarshal(request.D.Data["inputAudioTracks"], &wire)
			if !p.ignoreWrite {
				// Emulate OBS's numeric-key partial update semantics, so domain
				// keys would silently fail to make the intended change.
				for i := 1; i <= 6; i++ {
					if value, exists := wire[fmt.Sprint(i)]; exists {
						p.tracks[fmt.Sprintf("track%d", i)] = value
					}
				}
			}
			withhold, onSet = p.withholdReply, p.onSet
			p.setOnce.Do(func() { close(p.setSeen) })
		}
		p.mu.Unlock()
		if onSet != nil {
			onSet()
		}
		if !withhold {
			_ = conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{
				"requestType": request.D.Type, "requestId": request.D.ID,
				"requestStatus": map[string]any{"result": true, "code": 100}, "responseData": data}})
		}
	}
}

func tracksForMask(mask int) map[string]bool {
	tracks := make(map[string]bool, 6)
	for i := 1; i <= 6; i++ {
		tracks[fmt.Sprintf("track%d", i)] = mask&(1<<(i-1)) != 0
	}
	return tracks
}

func (p *repairProtocol) writeCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.writes)
}

func TestTrackRepairProtocolPreservesEveryAssignment(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	count := 0
	for mask := 0; mask < 64; mask++ {
		for selected := 1; selected <= 6; selected++ {
			if mask&(1<<(selected-1)) != 0 {
				continue
			}
			before := tracksForMask(mask)
			p.mu.Lock()
			p.tracks = cloneTracks(before)
			p.mu.Unlock()
			m.mu.Lock()
			m.settings.StreamTrack = &selected
			m.mu.Unlock()
			req.SelectedTrack, req.ExpectedTracks = selected, before
			desired := cloneTracks(before)
			desired[fmt.Sprintf("track%d", selected)] = true
			result := m.RepairTrack(context.Background(), req, func(pre domain.TrackRepairPreflight) error {
				if !reflect.DeepEqual(pre.State.Tracks, before) || !reflect.DeepEqual(pre.DesiredTracks, desired) {
					t.Fatalf("incorrect intent evidence: %+v", pre)
				}
				// Authority gets an immutable copy, not maps later sent to OBS.
				pre.DesiredTracks["track1"] = !pre.DesiredTracks["track1"]
				return nil
			})
			if result.Status != domain.TrackRepairVerified || !result.DispatchAdmitted || !result.PossiblyTransmitted ||
				result.Observed == nil || !reflect.DeepEqual(result.Observed.Tracks, desired) {
				t.Fatalf("mask=%d selected=%d: %+v", mask, selected, result)
			}
			p.mu.Lock()
			wireRequest := p.writes[len(p.writes)-1]
			p.mu.Unlock()
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(wireRequest["inputAudioTracks"], &wire); err != nil || len(wire) != 6 {
				t.Fatalf("not exactly six wire assignments: %s, %v", wireRequest["inputAudioTracks"], err)
			}
			for i := 1; i <= 6; i++ {
				if string(wire[fmt.Sprint(i)]) != fmt.Sprint(desired[fmt.Sprintf("track%d", i)]) {
					t.Fatalf("numeric key or explicit false lost: %s", wireRequest["inputAudioTracks"])
				}
			}
			if string(wireRequest["inputUuid"]) != `"test-mic-uuid"` || wireRequest["inputName"] != nil {
				t.Fatalf("wrong wire identity: %v", wireRequest)
			}
			count++
		}
	}
	if count != 192 || p.writeCount() != count {
		t.Fatalf("expected 192 individually approved additions, got %d/%d", count, p.writeCount())
	}
}

func TestTrackRepairProtocolRejectsChangedPreconditions(t *testing.T) {
	for _, scenario := range []string{"generation", "revision", "rename", "replacement", "collection", "map", "extra_map_key", "unconfirmed", "changed_track", "nil_callback", "callback_error", "callback_reconfigure", "callback_cancel"} {
		t.Run(scenario, func(t *testing.T) {
			p, m, req := newRepairProtocol(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			callback := func(domain.TrackRepairPreflight) error { return nil }
			p.mu.Lock()
			switch scenario {
			case "generation":
				req.Generation++
			case "revision":
				req.ConfigRevision++
			case "rename":
				p.name = "Renamed mic"
			case "replacement":
				p.uuid = "replacement-uuid"
			case "collection":
				p.collection = "Other collection"
			case "map":
				p.tracks["track5"] = true
			case "extra_map_key":
				req.ExpectedTracks["track7"] = true
			case "unconfirmed":
				m.settings.TrackConfirmedAt = nil
			case "changed_track":
				other := 6
				m.settings.StreamTrack = &other
			case "nil_callback":
				callback = nil
			case "callback_error":
				callback = func(domain.TrackRepairPreflight) error { return errors.New("intent persistence failed") }
			case "callback_reconfigure":
				callback = func(domain.TrackRepairPreflight) error { m.SetSettings(8, testSettings()); return nil }
			case "callback_cancel":
				callback = func(domain.TrackRepairPreflight) error { cancel(); return nil }
			}
			p.mu.Unlock()
			result := m.RepairTrack(ctx, req, callback)
			if result.Status != domain.TrackRepairInvalidated || result.PossiblyTransmitted || p.writeCount() != 0 {
				t.Fatalf("rejected precondition reached mutation: %+v, writes=%d", result, p.writeCount())
			}
		})
	}
}

func TestTrackRepairProtocolAlreadyDesiredIsNoOp(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	p.mu.Lock()
	p.tracks["track1"] = true
	p.mu.Unlock()
	called := false
	result := m.RepairTrack(context.Background(), req, func(domain.TrackRepairPreflight) error { called = true; return nil })
	if result.Status != domain.TrackRepairNoOp || result.Observed == nil || !result.Observed.Tracks["track1"] ||
		called || result.PossiblyTransmitted || p.writeCount() != 0 {
		t.Fatalf("already desired state was not a read-only no-op: %+v", result)
	}
}

func TestTrackRepairProtocolMismatchIsUnresolvedWithoutRollback(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	p.mu.Lock()
	p.ignoreWrite = true
	p.mu.Unlock()
	result := m.RepairTrack(context.Background(), req, func(domain.TrackRepairPreflight) error { return nil })
	if result.Status != domain.TrackRepairFailed || result.DetailCode != "readback_mismatch" ||
		result.Observed == nil || p.writeCount() != 1 {
		t.Fatalf("mismatch did not preserve one-write evidence: %+v", result)
	}
}

func TestTrackRepairProtocolLostResponseRemainsUnknownWithoutReplay(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	p.mu.Lock()
	p.withholdReply = true
	p.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan domain.TrackRepairResult, 1)
	go func() { done <- m.RepairTrack(ctx, req, func(domain.TrackRepairPreflight) error { return nil }) }()
	select {
	case <-p.setSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("write never reached protocol server")
	}
	cancel()
	select {
	case result := <-done:
		if result.Status != domain.TrackRepairUnknown || !result.PossiblyTransmitted || p.writeCount() != 1 {
			t.Fatalf("lost response falsely settled/replayed: %+v", result)
		}
		if conn := m.Snapshot(); conn.Status == domain.ConnectionConnected || conn.Generation == req.Generation || !conn.Stale {
			t.Fatalf("canceled transport retained connected evidence: %+v", conn)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled write did not join the transport")
	}
	if p.writeCount() != 1 {
		t.Fatal("unknown action was replayed")
	}
}

func TestTrackRepairProtocolOldGenerationCannotVerify(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	p.mu.Lock()
	p.onSet = func() { m.mu.Lock(); m.gen++; m.mu.Unlock() }
	p.mu.Unlock()
	result := m.RepairTrack(context.Background(), req, func(domain.TrackRepairPreflight) error { return nil })
	if result.Status != domain.TrackRepairUnknown || !result.PossiblyTransmitted || result.Generation != req.Generation ||
		result.Observed != nil || p.writeCount() != 1 {
		t.Fatalf("old-generation result verified or lost audit: %+v", result)
	}
}

func TestTrackStateProtocolRejectsMalformedMap(t *testing.T) {
	for name, wire := range map[string]any{
		"null":      map[string]any{"1": nil, "2": false, "3": false, "4": false, "5": false, "6": false},
		"missing":   map[string]bool{"1": false},
		"collision": map[string]bool{"1": false, "track1": true, "2": false, "3": false, "4": false, "5": false, "6": false},
		"extra":     map[string]bool{"1": false, "2": false, "3": false, "4": false, "5": false, "6": false, "7": true},
	} {
		t.Run(name, func(t *testing.T) {
			p, m, req := newRepairProtocol(t)
			p.mu.Lock()
			p.malformed = wire
			p.mu.Unlock()
			_, err := m.ReadTrackState(context.Background(), domain.TrackStateRequest{Input: req.Input, Generation: req.Generation, ConfigRevision: req.ConfigRevision})
			if err == nil || p.writeCount() != 0 {
				t.Fatal("malformed wire map became complete trusted evidence")
			}
		})
	}
}

func TestTrackRepairProtocolNaturalTimeoutInvalidatesTransport(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	p.mu.Lock()
	p.withholdReply = true
	p.mu.Unlock()
	started := time.Now()
	result := m.RepairTrack(context.Background(), req, func(domain.TrackRepairPreflight) error { return nil })
	if result.Status != domain.TrackRepairUnknown || !result.PossiblyTransmitted || p.writeCount() != 1 {
		t.Fatalf("SDK response timeout lost or replayed mutation: %+v", result)
	}
	if elapsed := time.Since(started); elapsed < 4*time.Second || elapsed > 8*time.Second {
		t.Fatalf("SDK timeout was not bounded as configured: %v", elapsed)
	}
	if conn := m.Snapshot(); conn.Status == domain.ConnectionConnected || conn.Generation == req.Generation || !conn.Stale {
		t.Fatalf("timed-out transport retained connected evidence: %+v", conn)
	}
}

func TestTrackStateCanceledReadImmediatelyInvalidatesTransport(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	p.mu.Lock()
	p.withholdRead = true
	p.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := m.ReadTrackState(ctx, domain.TrackStateRequest{Input: req.Input, Generation: req.Generation, ConfigRevision: req.ConfigRevision})
	if err == nil {
		t.Fatal("canceled read reported trusted state")
	}
	if conn := m.Snapshot(); conn.Status == domain.ConnectionConnected || conn.Generation == req.Generation || !conn.Stale {
		t.Fatalf("canceled read retained connected evidence: %+v", conn)
	}
	if p.writeCount() != 0 {
		t.Fatal("reconciliation sent a mutation")
	}
}

func TestTrackStateReconciliationTargetsOriginalInput(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	m.SetSettings(8, testSettings()) // current configured microphone is different
	state, err := m.ReadTrackState(context.Background(), domain.TrackStateRequest{Input: req.Input, Generation: req.Generation, ConfigRevision: 8})
	if err != nil || state.Input != req.Input || state.ConfigRevision != 8 || !reflect.DeepEqual(state.Tracks, p.tracks) || p.writeCount() != 0 {
		t.Fatalf("read-only reconciliation retargeted current expectations: %+v, %v", state, err)
	}
}

func TestTrackRepairAdmissionIsHeldThroughAuthorityCallback(t *testing.T) {
	p, m, req := newRepairProtocol(t)
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan domain.TrackRepairResult, 1)
	go func() {
		done <- m.RepairTrack(context.Background(), req, func(domain.TrackRepairPreflight) error { close(entered); <-release; return nil })
	}()
	<-entered
	second := m.RepairTrack(context.Background(), req, func(domain.TrackRepairPreflight) error { t.Error("second dispatch admitted"); return nil })
	if second.DetailCode != "obs_busy" {
		t.Errorf("concurrent repair was not refused: %+v", second)
	}
	_, err := m.ReadTrackState(context.Background(), domain.TrackStateRequest{Input: req.Input, Generation: req.Generation, ConfigRevision: req.ConfigRevision})
	if !errors.Is(err, ErrBusy) {
		t.Errorf("repair released its operation during callback: %v", err)
	}
	close(release)
	if result := <-done; result.Status != domain.TrackRepairVerified || p.writeCount() != 1 {
		t.Fatalf("reserved operation did not settle exactly once: %+v", result)
	}
}
