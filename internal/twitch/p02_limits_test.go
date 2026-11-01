package twitch

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCooldownExpiresAndIsBounded(t *testing.T) {
	cooldown := NewCooldown(time.Second)
	cooldown.max = 2
	now := time.Now()
	if !cooldown.Allow("a", now) || !cooldown.Allow("b", now) {
		t.Fatal("initial actors were rejected")
	}
	if !cooldown.Allow("c", now) {
		t.Fatal("bounded cooldown did not evict")
	}
	if cooldown.Allow("c", now) {
		t.Fatal("cooldown did not reject within the window")
	}
	if !cooldown.Allow("c", now.Add(2*time.Second)) {
		t.Fatal("cooldown did not expire")
	}
}

func TestCommandEventCarriesNormalizedIdentityOnly(t *testing.T) {
	manager := NewManager(Options{Context: context.Background(), OnUpdate: func() {}})
	var got CommandEvent
	manager.onCommand = func(event CommandEvent) { got = event }
	c := reviewCredentials()
	manager.mu.Lock()
	manager.creds = &c
	manager.conn.ConnectionGeneration = 7
	manager.mu.Unlock()
	event := ChatEvent{
		BroadcasterUserID: "user-1",
		ChatterUserID:     "viewer-2",
		MessageID:         "msg-1",
	}
	event.Message.Text = "!issueaudio"
	manager.handleChat(0, event, "delivery-1")
	if got.DeliveryID != "delivery-1" || got.MessageID != "msg-1" || got.ActorID != "viewer-2" ||
		got.BroadcasterID != "user-1" || got.Command.Kind != CommandIssueAudio || got.ConnectionGeneration != 7 {
		t.Fatalf("normalized command event = %+v", got)
	}
	if got.ReceivedAt.IsZero() {
		t.Fatal("normalized command event has no timestamp")
	}
}

func TestReplyQueueHonorsProviderRetryAfter(t *testing.T) {
	var mu sync.Mutex
	var sent []time.Time
	queue := NewReplyQueue(context.Background(), func(context.Context, string) error {
		mu.Lock()
		sent = append(sent, time.Now())
		n := len(sent)
		mu.Unlock()
		if n == 1 {
			return &ProviderError{Code: "rate_limited", Status: 429, RetryAfter: 60 * time.Millisecond}
		}
		return nil
	}, time.Now, time.Millisecond)
	defer queue.Close()
	queue.Enqueue("one")
	queue.Enqueue("two")
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := len(sent)
		mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reply queue did not send second item")
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	gap := sent[1].Sub(sent[0])
	mu.Unlock()
	if gap < 45*time.Millisecond {
		t.Fatalf("provider Retry-After was ignored: gap=%v", gap)
	}
}

func TestReplyQueueDrainsExpiredHeadAndJoinsSender(t *testing.T) {
	clock := &closureClock{value: time.Unix(1000, 0)}
	var sent []string
	var mu sync.Mutex
	sender := func(ctx context.Context, text string) error {
		mu.Lock()
		sent = append(sent, text)
		mu.Unlock()
		return nil
	}
	queue := NewReplyQueue(context.Background(), sender, clock.now, time.Millisecond)
	queue.Enqueue("expired")
	clock.advance(time.Minute)
	queue.Enqueue("fresh")
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		got := len(sent)
		mu.Unlock()
		if got > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired head stranded the fresh item")
		}
		time.Sleep(time.Millisecond)
	}
	queue.Close()

	blocked := make(chan struct{})
	joined := NewReplyQueue(context.Background(), func(ctx context.Context, _ string) error {
		close(blocked)
		<-ctx.Done()
		return ctx.Err()
	}, time.Now, time.Millisecond)
	joined.Enqueue("cancel me")
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("sender did not start")
	}
	done := make(chan struct{})
	go func() { joined.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ReplyQueue.Close did not join an active sender")
	}
}
