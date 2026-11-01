package goobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andreykaipov/goobs/api"
	"github.com/andreykaipov/goobs/api/opcodes"
	"github.com/andreykaipov/goobs/api/requests/general"
	"github.com/gorilla/websocket"
)

func TestCueMenderConstructorCancellationJoins(t *testing.T) {
	for _, phase := range []string{"hello", "identify", "auth_failure", "eof", "timeout"} {
		t.Run(phase, func(t *testing.T) {
			entered, peerDone := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer close(peerDone)
				defer conn.Close()
				if phase != "hello" && phase != "timeout" {
					_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"rpcVersion": 1}})
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
				close(entered)
				if phase == "auth_failure" {
					_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4009, "denied"))
					return
				}
				if phase == "eof" {
					return
				}
				_, _, _ = conn.ReadMessage()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var captured *Client
			result := make(chan error, 1)
			go func() {
				timeout := 5 * time.Second
				if phase == "timeout" {
					timeout = 20 * time.Millisecond
				}
				_, err := New("ws"+strings.TrimPrefix(server.URL, "http"), WithContext(ctx), WithResponseTimeoutDuration(timeout),
					WithLogger(discardLogger{}), func(c *Client) { captured = c })
				result <- err
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("peer did not reach phase")
			}
			if phase == "hello" || phase == "identify" {
				cancel()
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("constructor succeeded")
				}
			case <-time.After(300 * time.Millisecond):
				t.Fatal("constructor did not cancel and join")
			}
			joined := make(chan struct{})
			go func() { captured.Wait(); close(joined) }()
			select {
			case <-joined:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("SDK workers remain")
			}
			select {
			case <-peerDone:
			case <-time.After(time.Second):
				t.Fatal("peer remains open")
			}
		})
	}
}

type discardLogger struct{}

func (discardLogger) Printf(string, ...any) {}

func TestCueMenderPendingRequestDisconnectJoins(t *testing.T) {
	request, peerDone := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer close(peerDone)
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"rpcVersion": 1}})
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		close(request)
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	client, err := New("ws"+strings.TrimPrefix(server.URL, "http"), WithLogger(discardLogger{}))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect()
	result := make(chan error, 1)
	go func() { _, err := client.General.GetVersion(); result <- err }()
	<-request
	closed := make(chan struct{})
	go func() { _ = client.Disconnect(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Disconnect did not join")
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("stalled request succeeded")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("request remains")
	}
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("peer remains")
	}
}

func TestCueMenderRequestAdmissionTimeout(t *testing.T) {
	client := &api.Client{Disconnected: make(chan struct{}), Opcodes: make(chan opcodes.Opcode),
		IncomingResponses: make(chan *opcodes.RequestResponse), ResponseTimeout: 30 * time.Millisecond, Log: discardLogger{}}
	defer client.Close()
	result := make(chan error, 1)
	go func() { result <- client.SendRequest(&general.GetVersionParams{}, &general.GetVersionResponse{}) }()
	select {
	case err := <-result:
		if err == nil {
			t.Error("unconsumed request admission succeeded")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("request timeout does not cover opcode admission")
		client.Close()
		<-result
	}
}

func TestCueMenderLateResponseCannotBlockNextRequest(t *testing.T) {
	releaseLate, lateSent, peerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	responseSeen := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer close(peerDone)
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"rpcVersion": 1}})
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
		var request struct {
			D struct {
				Type string `json:"requestType"`
				ID   string `json:"requestId"`
			} `json:"d"`
		}
		if err := conn.ReadJSON(&request); err != nil {
			return
		}
		<-releaseLate
		_ = conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{"requestType": request.D.Type,
			"requestId": request.D.ID, "requestStatus": map[string]any{"result": true, "code": 100}, "responseData": map[string]any{}}})
		close(lateSent)
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	client, err := New("ws"+strings.TrimPrefix(server.URL, "http"), WithResponseTimeoutDuration(80*time.Millisecond), WithLogger(&responseBarrierLogger{seen: responseSeen}))
	if err != nil {
		close(releaseLate)
		t.Fatal(err)
	}
	defer client.Disconnect()
	if _, err := client.General.GetVersion(); err == nil {
		close(releaseLate)
		t.Fatal("withheld response did not time out")
	}
	close(releaseLate)
	<-lateSent
	// The old worker has entered response delivery, or the timeout has already
	// invalidated the pipeline. Do not race B against raw-frame processing.
	select {
	case <-responseSeen:
	case <-client.client.Disconnected:
	case <-time.After(time.Second):
		t.Fatal("late response never reached delivery")
	}
	result := make(chan error, 1)
	go func() { _, err := client.General.GetVersion(); result <- err }()
	select {
	case err := <-result:
		if err == nil {
			t.Error("poisoned pipeline accepted another request")
		}
	case <-time.After(250 * time.Millisecond):
		t.Error("late response blocks next request admission beyond its timeout")
		_ = client.Disconnect()
		<-result
	}
	_ = client.Disconnect()
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("timed-out pipeline left peer open")
	}
}

type responseBarrierLogger struct {
	seen chan struct{}
	once sync.Once
}

func (l *responseBarrierLogger) Printf(format string, _ ...any) {
	if strings.Contains(format, "Response for ID") {
		l.once.Do(func() { close(l.seen) })
	}
}
