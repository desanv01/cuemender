package httpui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/desanv01/cuemender/internal/domain"
)

func (b *testBackend) CloseIncident(_ context.Context, id, outcome string) (domain.Incident, error) {
	if id != "inc-close" {
		return domain.Incident{}, domain.ErrIncidentNotFound
	}
	return domain.Incident{ID: id, PublicID: "CM-CLOSE", Stage: domain.StageClosed, ClosureOutcome: outcome}, nil
}

func TestP05ClosureAndRehearsalRouteBoundaries(t *testing.T) {
	ts, ui, _ := newTestServer(t, nil)
	defer ts.Close()
	client := newClient(t, ts)
	csrf := bootstrap(t, ts, client, ui.BootstrapToken())
	post := func(path string, body any, token string) *http.Response {
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Host = req.URL.Host
		req.Header.Set("Origin", ts.URL)
		if token != "" {
			req.Header.Set("X-CSRF-Token", token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	resp := post("/api/v1/incidents/inc-close/close", map[string]any{"outcome": "viewer_confirmed"}, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("close status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = post("/api/v1/incidents/inc-close/close", map[string]any{"outcome": "bogus"}, csrf)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown outcome status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = post("/api/v1/incidents/inc-close/close", map[string]any{"outcome": "unresolved"}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = post("/api/v1/rehearsal/scenario", map[string]any{"scenario": "track_missing"}, csrf)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("live rehearsal scenario status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	resp = post("/api/v1/rehearsal/report", map[string]any{"category": "audio"}, csrf)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("live rehearsal report status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}
