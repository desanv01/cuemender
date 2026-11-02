//go:build reviewrepro

package app

import (
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
)

func TestReviewR05ProtocolFixtureJoinsCanceledUpgrade(t *testing.T) {
	chat := &protocolChat{welcome: make(chan string, 1), help: make(chan string, 1), chat: make(chan string, 1), replies: make(chan string, 1)}
	server := localTwitchServer(t, chat)
	conn, _, err := gorilla.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	<-chat.welcome
	conn.Close()
	done := make(chan struct{})
	go func() { chat.handlerWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("upgraded fixture handler did not join after client cancellation before help/report")
		// Finish cleanup even on failure; the existing five-second fixture timer exits.
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("fixture handler still alive after cleanup deadline")
		}
	}
}
