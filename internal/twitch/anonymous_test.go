package twitch

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func testIRCLine(now time.Time, id, actor, body string) string {
	return fmt.Sprintf("@room-id=123;user-id=%s;id=%s;tmi-sent-ts=%d :viewer!viewer@viewer.tmi.twitch.tv PRIVMSG #testchannel :%s", actor, id, now.UnixMilli(), body)
}
func TestAnonymousMetadataRejection(t *testing.T) {
	now := time.Now()
	a := NewAnonymous(AnonymousOptions{Channel: "testchannel"})
	defer a.Close()
	valid := testIRCLine(now, "message-123456", "456", "!issueaudio")
	cases := map[string]string{"missing_sender": strings.Replace(valid, "user-id=456;", "", 1), "missing_message": strings.Replace(valid, "id=message-123456;", "", 1), "wrong_room": strings.Replace(valid, "room-id=123", "room-id=789", 1), "broadcaster": strings.Replace(valid, "user-id=456", "user-id=123", 1), "other_channel": strings.Replace(valid, "#testchannel", "#other", 1), "shared": strings.Replace(valid, "@", "@source-room-id=789;", 1), "old": testIRCLine(now.Add(-time.Minute), "message-123456", "456", "!issueaudio"), "future": testIRCLine(now.Add(time.Minute), "message-123456", "456", "!issueaudio"), "bad_command": testIRCLine(now, "message-123456", "456", "!issueaudio extra"), "duplicate_tag": strings.Replace(valid, "@", "@room-id=123;", 1), "malformed_id": strings.Replace(valid, "user-id=456", "user-id=x", 1)}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			m, ok := parseIRC(line)
			if ok {
				_, ok = a.event(m, "123", now)
			}
			if ok {
				t.Fatal("accepted invalid provenance")
			}
		})
	}
	for _, body := range []string{"!issueaudio", "!issuevideo", "!fixed CM-ABCDEF", "!stillbroken CM-ABCDEF"} {
		m, ok := parseIRC(testIRCLine(now, "message-123456", "456", body))
		if !ok {
			t.Fatal("parse")
		}
		event, ok := a.event(m, "123", now)
		if !ok || event.ActorID != "456" || event.BroadcasterID != "123" || event.ReceivedAt.IsZero() {
			t.Fatal("valid event rejected")
		}
	}
	for _, channel := range []string{"../test", "test\r\nJOIN #other", "A", "two words", ""} {
		if ValidChannel(channel) {
			t.Fatal("unsafe channel accepted")
		}
	}
}

func TestAnonymousLifecycleDedupAndReconnect(t *testing.T) {
	peers := make(chan net.Conn, 4)
	events := make(chan CommandEvent, 8)
	a := NewAnonymous(AnonymousOptions{Channel: "testchannel", RetryDelay: time.Millisecond, Dial: func(ctx context.Context) (net.Conn, error) { c, s := net.Pipe(); peers <- s; return c, nil }, OnCommand: func(e CommandEvent) { events <- e }})
	defer a.Close()
	connect := func() net.Conn {
		var s net.Conn
		select {
		case s = <-peers:
		case <-time.After(time.Second):
			t.Fatal("no transport")
		}
		t.Cleanup(func() { s.Close() })
		reader := bufio.NewReader(s)
		for i := 0; i < 4; i++ {
			s.SetReadDeadline(time.Now().Add(time.Second))
			line, e := reader.ReadString('\n')
			if e != nil || strings.HasPrefix(line, "PRIVMSG") {
				t.Fatalf("handshake %q %v", line, e)
			}
		}
		fmt.Fprint(s, ":tmi.twitch.tv CAP * ACK :twitch.tv/tags twitch.tv/commands\r\n@room-id=123 :tmi.twitch.tv ROOMSTATE #testchannel\r\n:tmi.twitch.tv 366 justinfan123123 #testchannel :End\r\n")
		deadline := time.Now().Add(time.Second)
		for a.Snapshot().Status != "connected" && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if a.Snapshot().Status != "connected" {
			t.Fatal("not ready")
		}
		return s
	}
	if e := a.Connect(); e != nil {
		t.Fatal(e)
	}
	s := connect()
	gen := a.Snapshot().ConnectionGeneration
	now := time.Now()
	line := testIRCLine(now, "message-123456", "456", "!issueaudio")
	fmt.Fprintln(s, line)
	var first CommandEvent
	select {
	case first = <-events:
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
	if first.ConnectionGeneration != gen {
		t.Fatal("wrong generation")
	}
	fmt.Fprintln(s, line)
	fmt.Fprintln(s, testIRCLine(now, "message-123457", "456", "!issueaudio"))
	// A second actor is a read barrier proving both preceding lines were processed.
	fmt.Fprintln(s, testIRCLine(now, "message-123458", "457", "!issuevideo"))
	select {
	case e := <-events:
		if e.ActorID != "457" {
			t.Fatal("dedup/cooldown failed")
		}
	case <-time.After(time.Second):
		t.Fatal("barrier missing")
	}
	s.Close()
	s = connect()
	if a.Snapshot().ConnectionGeneration <= gen {
		t.Fatal("generation not replaced")
	}
	fmt.Fprintln(s, testIRCLine(time.Now(), "message-123459", "458", "!issueaudio"))
	select {
	case e := <-events:
		if e.ConnectionGeneration <= gen {
			t.Fatal("old generation")
		}
	case <-time.After(time.Second):
		t.Fatal("reconnect missing")
	}
	a.Disconnect()
	if a.Snapshot().Status != "not_connected" {
		t.Fatal("disconnect not published")
	}
	b := make([]byte, 1)
	s.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := s.Read(b); err == nil {
		t.Fatal("socket still open")
	}
	a.Close()
	if a.Connect() == nil {
		t.Fatal("closed receiver reconnected")
	}
}

func TestAnonymousCancellationDuringDial(t *testing.T) {
	started := make(chan struct{})
	a := NewAnonymous(AnonymousOptions{Channel: "testchannel", Dial: func(ctx context.Context) (net.Conn, error) { close(started); <-ctx.Done(); return nil, ctx.Err() }})
	if e := a.Connect(); e != nil {
		t.Fatal(e)
	}
	<-started
	done := make(chan struct{})
	go func() { a.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close did not cancel/join dial")
	}
}

func TestAnonymousCloseDuringSilentHandshake(t *testing.T) {
	peers := make(chan net.Conn, 1)
	a := NewAnonymous(AnonymousOptions{Channel: "testchannel", Dial: func(context.Context) (net.Conn, error) { c, s := net.Pipe(); peers <- s; return c, nil }})
	defer a.Close()
	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	s := <-peers
	defer s.Close()
	reader := bufio.NewReader(s)
	for i := 0; i < 4; i++ {
		s.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan struct{})
	go func() { a.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("silent handshake did not cancel/join")
	}
	if a.Snapshot().Status != "not_connected" {
		t.Fatal("close left stale readiness")
	}
}
