package httpui_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestP03MonitoringAndIncidentRoutes(t *testing.T) {
	ts, ui, backend := newTestServer(t, nil)
	defer ts.Close()
	client := newClient(t, ts)
	csrf := bootstrap(t, ts, client, ui.BootstrapToken())
	headers := map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf}

	resp, body := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/monitoring/start", `{"expected_revision":0}`, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start = %d body=%v", resp.StatusCode, body)
	}
	data, _ := body["data"].(map[string]any)
	if data["monitoring_active"] != true || data["session_id"] == "" {
		t.Fatalf("start data = %v", data)
	}
	resp, body = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/monitoring/start", `{"expected_revision":0}`, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("repeat start = %d", resp.StatusCode)
	}
	again, _ := body["data"].(map[string]any)
	if again["session_id"] != data["session_id"] {
		t.Fatal("repeat start created a second session")
	}
	resp, body = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/incidents/check", `{"category":"audio","expected_revision":0}`, headers)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("check = %d body=%v", resp.StatusCode, body)
	}
	checkData, _ := body["data"].(map[string]any)
	incidentID, _ := checkData["incident_id"].(string)
	if incidentID == "" || checkData["public_id"] == "" {
		t.Fatalf("check data = %v", checkData)
	}
	backend.mu.Lock()
	backend.incidents = append(backend.incidents, domain.Incident{ID: incidentID, PublicID: "CM-TEST", Category: "audio", Stage: domain.StageInvestigating})
	backend.mu.Unlock()
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/incidents/"+incidentID+"/dismiss", `{}`, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dismiss = %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/incidents/missing/dismiss", `{}`, headers)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing dismiss = %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/monitoring/stop", `{}`, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop = %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/monitoring/start", `{"expected_revision":0}`, map[string]string{"Origin": ts.URL})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("start without CSRF = %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/incidents/check", `{"category":"bogus","expected_revision":0}`, headers)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad category = %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/monitoring/start", `{"expected_revision":0,"unknown":true}`, headers)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown field = %d", resp.StatusCode)
	}
}

func TestP03RouteErrorBoundaries(t *testing.T) {
	ts, ui, backend := newTestServer(t, nil)
	defer ts.Close()
	client := newClient(t, ts)
	csrf := bootstrap(t, ts, client, ui.BootstrapToken())
	headers := map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf}
	backend.mu.Lock()
	backend.routeErr = domain.ErrUnavailable
	backend.mu.Unlock()
	resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/monitoring/start", `{"expected_revision":0}`, headers)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unavailable start = %d", resp.StatusCode)
	}
	backend.mu.Lock()
	backend.routeErr = domain.ErrJobLimit
	backend.mu.Unlock()
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/incidents/check", `{"category":"audio","expected_revision":0}`, headers)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("busy check = %d", resp.StatusCode)
	}
	backend.mu.Lock()
	backend.routeErr = domain.ErrTerminalIncident
	backend.mu.Unlock()
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/incidents/x/dismiss", `{}`, headers)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("terminal dismiss = %d", resp.StatusCode)
	}
}

func TestP03PublicStateRedaction(t *testing.T) {
	ts, _, backend := newTestServer(t, nil)
	defer ts.Close()
	backend.mu.Lock()
	backend.monitoring = true
	backend.incidents = []domain.Incident{{ID: "private-id", PublicID: "CM-PUB", Category: "audio", Stage: domain.StageFindingAvailable}}
	backend.mu.Unlock()
	resp, body := doJSON(t, http.DefaultClient, http.MethodGet, ts.URL+"/public/v1/state", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public = %d", resp.StatusCode)
	}
	raw, _ := json.Marshal(body)
	for _, forbidden := range []string{"private-id", "settings", "microphone", "findings"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("public state leaked %q: %s", forbidden, raw)
		}
	}
}
