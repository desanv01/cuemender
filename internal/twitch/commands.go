package twitch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type CommandKind string

const (
	CommandIssueAudio  CommandKind = "issue_audio"
	CommandIssueVideo  CommandKind = "issue_video"
	CommandFixed       CommandKind = "fixed"
	CommandStillBroken CommandKind = "still_broken"
	CommandStreamHelp  CommandKind = "streamhelp"
)

type Command struct {
	Kind         CommandKind
	Category     string
	IncidentCode string
}

// CommandEvent is the typed C04 seam for later P03 processing. It retains only
// normalized command metadata, never raw chat text.
type CommandEvent struct {
	DeliveryID           string
	MessageID            string
	BroadcasterID        string
	ActorID              string
	ReceivedAt           time.Time
	Command              Command
	ConnectionGeneration uint64
	MonitoringSessionID  string
}

func ParseCommand(text string) (Command, bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return Command{}, false
	}
	switch strings.ToLower(fields[0]) {
	case "!issueaudio":
		if len(fields) == 1 {
			return Command{Kind: CommandIssueAudio, Category: "audio"}, true
		}
	case "!issuevideo":
		if len(fields) == 1 {
			return Command{Kind: CommandIssueVideo, Category: "video"}, true
		}
	case "!streamhelp":
		if len(fields) == 1 {
			return Command{Kind: CommandStreamHelp}, true
		}
	case "!fixed":
		if len(fields) == 2 && validIncidentCode(fields[1]) {
			return Command{Kind: CommandFixed, IncidentCode: strings.ToUpper(fields[1])}, true
		}
	case "!stillbroken":
		if len(fields) == 2 && validIncidentCode(fields[1]) {
			return Command{Kind: CommandStillBroken, IncidentCode: strings.ToUpper(fields[1])}, true
		}
	}
	return Command{}, false
}

func validIncidentCode(code string) bool {
	if len(code) < 4 || len(code) > 12 {
		return false
	}
	for _, r := range code {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func (c Command) IsIssue() bool {
	return c.Kind == CommandIssueAudio || c.Kind == CommandIssueVideo
}

// Dedup retains delivery/message IDs for 24 hours and at most 10,000 IDs.
type Dedup struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	entries map[string]time.Time
	order   []string
}

func NewDedup() *Dedup {
	return &Dedup{
		ttl:     24 * time.Hour,
		max:     10000,
		entries: make(map[string]time.Time),
	}
}

// Seen records both IDs and returns true when either was already retained.
func (d *Dedup) Seen(deliveryID, messageID string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.evict(now)
	if deliveryID != "" {
		if _, ok := d.entries[deliveryID]; ok {
			return true
		}
	}
	if messageID != "" {
		if _, ok := d.entries[messageID]; ok {
			return true
		}
	}
	d.record(deliveryID, now)
	d.record(messageID, now)
	d.evict(now)
	return false
}

func (d *Dedup) record(id string, now time.Time) {
	if id == "" {
		return
	}
	if _, exists := d.entries[id]; exists {
		return
	}
	d.entries[id] = now
	d.order = append(d.order, id)
}

func (d *Dedup) evict(now time.Time) {
	cutoff := now.Add(-d.ttl)
	for len(d.order) > 0 {
		id := d.order[0]
		when, ok := d.entries[id]
		if ok && when.After(cutoff) && len(d.entries) <= d.max {
			break
		}
		delete(d.entries, id)
		d.order = d.order[1:]
	}
}

// Cooldown applies the per-actor issue-command window.
type Cooldown struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	last   map[string]time.Time
}

func NewCooldown(window time.Duration) *Cooldown {
	return &Cooldown{window: window, max: 10000, last: make(map[string]time.Time)}
}

func (c *Cooldown) Allow(actor string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, prior := range c.last {
		if now.Sub(prior) >= c.window {
			delete(c.last, key)
		}
	}
	if prior, ok := c.last[actor]; ok && now.Sub(prior) < c.window {
		return false
	}
	if len(c.last) >= c.max {
		var oldestKey string
		var oldest time.Time
		for key, prior := range c.last {
			if oldestKey == "" || prior.Before(oldest) {
				oldestKey, oldest = key, prior
			}
		}
		delete(c.last, oldestKey)
	}
	c.last[actor] = now
	return true
}

// ReplyQueue sends at most one fixed reply per interval with a bounded queue.
type ReplyQueue struct {
	mu           sync.Mutex
	items        []replyItem
	max          int
	ttl          time.Duration
	interval     time.Duration
	now          func() time.Time
	sender       func(context.Context, string) error
	providerWait func() time.Duration
	wake         chan struct{}
	closed       chan struct{}
	done         chan struct{}
	closeOnce    sync.Once
	ctx          context.Context
	cancel       context.CancelFunc
}

type replyItem struct {
	text    string
	expires time.Time
}

func NewReplyQueue(parent context.Context, sender func(context.Context, string) error, now func() time.Time, interval time.Duration, providerWait ...func() time.Duration) *ReplyQueue {
	if now == nil {
		now = time.Now
	}
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ctx, cancel := context.WithCancel(parent)
	q := &ReplyQueue{
		max:      20,
		ttl:      30 * time.Second,
		interval: interval,
		now:      now,
		sender:   sender,
		wake:     make(chan struct{}, 1),
		closed:   make(chan struct{}),
		done:     make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}
	if len(providerWait) != 0 {
		q.providerWait = providerWait[0]
	}
	go q.run()
	return q
}

func (q *ReplyQueue) Enqueue(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-q.closed:
		return false
	default:
	}
	if len(q.items) >= q.max {
		return false
	}
	q.items = append(q.items, replyItem{text: text, expires: q.now().Add(q.ttl)})
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return true
}

func (q *ReplyQueue) Close() {
	q.stop()
	<-q.done
}

// stop may be called by the sender itself; joining is the lifecycle owner's job.
func (q *ReplyQueue) stop() {
	q.closeOnce.Do(func() {
		q.cancel()
		close(q.closed)
	})
}

func (q *ReplyQueue) run() {
	defer close(q.done)
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.wake:
		}
		q.mu.Lock()
		if len(q.items) == 0 {
			q.mu.Unlock()
			continue
		}
		item := q.items[0]
		q.items = q.items[1:]
		more := len(q.items) > 0
		q.mu.Unlock()
		if q.now().After(item.expires) {
			if more {
				select {
				case q.wake <- struct{}{}:
				default:
				}
			}
			continue
		}
		wait := q.interval
		if err := q.sender(q.ctx, item.text); err != nil {
			// A failed or ambiguous send is not retried blindly. Honour a
			// provider Retry-After before considering the next aggregate reply.
			if retry, ok := err.(interface{ RetryAfterDuration() time.Duration }); ok {
				if d := retry.RetryAfterDuration(); d > wait {
					wait = d
				}
			}
		}
		if q.providerWait != nil {
			if delay := q.providerWait(); delay > wait {
				wait = delay
			}
		}
		timer := time.NewTimer(wait)
		select {
		case <-q.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		q.mu.Lock()
		more = len(q.items) > 0
		q.mu.Unlock()
		if more {
			select {
			case q.wake <- struct{}{}:
			default:
			}
		}
	}
}

var ErrQueueFull = errors.New("twitch reply queue is full")

func (q *ReplyQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *ReplyQueue) enqueueOrError(text string) error {
	if !q.Enqueue(text) {
		return fmt.Errorf("%w", ErrQueueFull)
	}
	return nil
}
