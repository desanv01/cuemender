package httpui

import (
	"context"
	"net/http"
)

func (s *Server) handleReceiveOnlyConnect(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		Channel   string `json:"channel"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	backend, ok := s.backend.(interface {
		ReceiveOnlyConnect(context.Context, string, bool) error
	})
	if !ok {
		writeError(w, r, 409, "mode_conflict", "Receive-only chat is not enabled for this run.")
		return
	}
	if err := backend.ReceiveOnlyConnect(r.Context(), body.Channel, body.Confirmed); err != nil {
		writeIntegrationError(w, r, err)
		return
	}
	writeDataStatus(w, r, http.StatusAccepted, map[string]string{"status": "connecting", "provider": "anonymous_irc"})
}
