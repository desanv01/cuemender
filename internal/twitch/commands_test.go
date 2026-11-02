package twitch

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseCommandExact(t *testing.T) {
	tests := []struct {
		text string
		kind CommandKind
		code string
		ok   bool
	}{
		{"!issueaudio", CommandIssueAudio, "", true},
		{"  !ISSUEAUDIO  ", CommandIssueAudio, "", true},
		{"!issuevideo", CommandIssueVideo, "", true},
		{"!streamhelp", CommandStreamHelp, "", true},
		{"!fixed ab12", CommandFixed, "AB12", true},
		{"!fixed CM-ABC123", CommandFixed, "CM-ABC123", true},
		{"!stillbroken XY9Z", CommandStillBroken, "XY9Z", true},
		{"!issue audio", "", "", false},
		{"!issueaudio extra", "", "", false},
		{"!fixed", "", "", false},
		{"!fixed AB", "", "", false},
		{"!fixed AB12 extra", "", "", false},
		{"hello", "", "", false},
	}
	for _, tc := range tests {
		got, ok := ParseCommand(tc.text)
		if ok != tc.ok {
			t.Errorf("%q ok=%v, want %v", tc.text, ok, tc.ok)
			continue
		}
		if ok && (got.Kind != tc.kind || got.IncidentCode != tc.code) {
			t.Errorf("%q = %+v", tc.text, got)
		}
	}
}

func TestDedupAndExpiry(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	d := NewDedup()
	if d.Seen("delivery-1", "message-1", now) {
		t.Fatal("first delivery marked duplicate")
	}
	if !d.Seen("delivery-1", "", now.Add(time.Second)) {
		t.Fatal("delivery id was not deduplicated")
	}
	if !d.Seen("", "message-1", now.Add(time.Second)) {
		t.Fatal("message id was not deduplicated")
	}
	if d.Seen("delivery-1", "message-1", now.Add(25*time.Hour)) {
		t.Fatal("24-hour expiry did not evict")
	}
}

func TestDedupBounded(t *testing.T) {
	d := NewDedup()
	now := time.Now()
	for i := 0; i < 10001; i++ {
		d.Seen("", "id-"+string(rune(i)), now)
	}
	d.mu.Lock()
	count := len(d.entries)
	d.mu.Unlock()
	if count > d.max {
		t.Fatalf("dedup retained %d entries, max %d", count, d.max)
	}
}

func TestCooldown(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	c := NewCooldown(10 * time.Second)
	if !c.Allow("actor", now) {
		t.Fatal("first issue rejected")
	}
	if c.Allow("actor", now.Add(5*time.Second)) {
		t.Fatal("cooldown did not reject")
	}
	if !c.Allow("actor", now.Add(11*time.Second)) {
		t.Fatal("cooldown did not expire")
	}
	if !c.Allow("other", now.Add(5*time.Second)) {
		t.Fatal("cooldown leaked across actors")
	}
}

func TestReplyQueueRateAndCapacity(t *testing.T) {
	var mu sync.Mutex
	var sent []string
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := NewReplyQueue(ctx, func(_ context.Context, text string) error {
		if text == "primer" {
			close(entered)
			<-release
			return nil
		}
		mu.Lock()
		sent = append(sent, text)
		mu.Unlock()
		return nil
	}, time.Now, 10*time.Millisecond)
	defer q.Close()
	defer unblock()
	q.Enqueue("primer")
	awaitClosed(t, entered, "capacity fixture sender")
	for i := 0; i < 20; i++ {
		if !q.Enqueue("reply") {
			t.Fatalf("queue rejected item %d before capacity", i)
		}
	}
	if q.Enqueue("overflow") {
		t.Fatal("queue accepted item over capacity")
	}
	unblock()
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(sent)
		mu.Unlock()
		if n >= 20 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d replies sent", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestValidateReconnectURL(t *testing.T) {
	valid := "wss://eventsub.wss.twitch.tv/ws?session_id=abc"
	if err := ValidateReconnectURL(valid); err != nil {
		t.Fatalf("valid reconnect rejected: %v", err)
	}
	for _, bad := range []string{
		"ws://eventsub.wss.twitch.tv/ws",
		"wss://evil.example/ws",
		"wss://eventsub.wss.twitch.tv.evil/ws",
		"wss://eventsub.wss.twitch.tv/ws with space",
	} {
		if err := ValidateReconnectURL(bad); err == nil {
			t.Errorf("accepted unsafe reconnect %q", bad)
		}
	}
}

func TestParseEventSubMessages(t *testing.T) {
	welcome := []byte(`{"metadata":{"message_id":"m1","message_type":"session_welcome","message_timestamp":"2026-11-01T12:00:00Z"},"payload":{"session":{"id":"session-1","status":"connected","keepalive_timeout_seconds":10}}}`)
	env, err := ParseEnvelope(welcome)
	if err != nil {
		t.Fatal(err)
	}
	session, err := ParseWelcome(env)
	if err != nil || session.ID != "session-1" || keepaliveDuration(session) != 10*time.Second {
		t.Fatalf("welcome = %+v, %v", session, err)
	}
	chat := []byte(`{"metadata":{"message_id":"delivery-1","message_type":"notification","message_timestamp":"2026-11-01T12:00:00Z"},"payload":{"subscription":{"type":"channel.chat.message","version":"1"},"event":{"message_id":"message-1","broadcaster_user_id":"user-1","chatter_user_id":"user-2","chatter_user_login":"viewer","message_type":"text","message":{"text":"!issueaudio"}}}}`)
	env, err = ParseEnvelope(chat)
	if err != nil {
		t.Fatal(err)
	}
	notification, err := ParseChatNotification(env)
	if err != nil || notification.Event.Message.Text != "!issueaudio" {
		t.Fatalf("chat = %+v, %v", notification, err)
	}
	reconnect := []byte(`{"metadata":{"message_id":"m2","message_type":"session_reconnect","message_timestamp":"2026-11-01T12:00:00Z"},"payload":{"session":{"id":"session-2","status":"reconnecting","reconnect_url":"wss://eventsub.wss.twitch.tv/ws?session_id=2"}}}`)
	env, err = ParseEnvelope(reconnect)
	if err != nil {
		t.Fatal(err)
	}
	session, err = ParseReconnect(env)
	if err != nil || session.ReconnectURL == "" {
		t.Fatalf("reconnect = %+v, %v", session, err)
	}
	if _, err := ParseEnvelope([]byte(`{"metadata":{"message_id":"x","message_type":"bogus"},"payload":{}}`)); err == nil {
		t.Fatal("unknown message type accepted")
	}
}

func TestChatPayloadSizeBound(t *testing.T) {
	huge := strings.Repeat("x", 4097)
	payload := []byte(`{"metadata":{"message_id":"m","message_type":"notification","message_timestamp":"2026-11-01T12:00:00Z"},"payload":{"subscription":{"type":"channel.chat.message","version":"1"},"event":{"message_id":"m","broadcaster_user_id":"u1","chatter_user_id":"u2","message":{"text":"` + huge + `"}}}}`)
	env, err := ParseEnvelope(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseChatNotification(env); err == nil {
		t.Fatal("oversized chat text accepted")
	}
}
