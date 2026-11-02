package twitch

// Anonymous is deliberately receive-only. Its sole outbound IRC verbs are
// CAP/PASS/NICK/JOIN/PING/PONG; no OAuth or chat-send surface exists.
import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func ValidChannel(s string) bool {
	if len(s) < 1 || len(s) > 25 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

type AnonymousOptions struct {
	Context   context.Context
	Channel   string
	OnUpdate  func()
	OnCommand func(CommandEvent)
	Now       func() time.Time
	// Private composition seam for protocol tests; production uses verified TLS.
	Dial       func(context.Context) (net.Conn, error)
	RetryDelay time.Duration
}
type Anonymous struct {
	opts     AnonymousOptions
	life     sync.Mutex
	mu       sync.Mutex
	state    domain.TwitchConnection
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	closed   bool
	dedup    *Dedup
	cooldown *Cooldown
}

func NewAnonymous(opts AnonymousOptions) *Anonymous {
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = 3 * time.Second
	}
	if opts.Dial == nil {
		opts.Dial = func(ctx context.Context) (net.Conn, error) {
			d := tls.Dialer{NetDialer: &net.Dialer{Timeout: 15 * time.Second}, Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "irc.chat.twitch.tv"}}
			return d.DialContext(ctx, "tcp", "irc.chat.twitch.tv:6697")
		}
	}
	return &Anonymous{opts: opts, state: domain.TwitchConnection{Status: domain.ConnectionNotConnected, Provider: "anonymous_irc", ReceiveOnly: true, Login: opts.Channel, Detail: "Receive-only chat. Ownership unverified; replies and repairs unavailable."}, dedup: NewDedup(), cooldown: NewCooldown(10 * time.Second)}
}
func (a *Anonymous) Snapshot() domain.TwitchConnection {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.state
	if s.Receipt.LastAt != nil {
		t := *s.Receipt.LastAt
		s.Receipt.LastAt = &t
	}
	return s
}
func (a *Anonymous) publish() {
	if a.opts.OnUpdate != nil {
		a.opts.OnUpdate()
	}
}
func (a *Anonymous) setStatus(status string, advance bool) {
	a.mu.Lock()
	a.state.Status = status
	if advance {
		a.state.ConnectionGeneration++
	}
	a.mu.Unlock()
	a.publish()
}
func (a *Anonymous) Connect() error {
	a.life.Lock()
	defer a.life.Unlock()
	if a.closed || a.cancel != nil || !ValidChannel(a.opts.Channel) {
		return ErrBusy
	}
	a.ctx, a.cancel = context.WithCancel(a.opts.Context)
	a.done = make(chan struct{})
	a.setStatus(domain.ConnectionConnecting, true)
	go a.run(a.ctx, a.done)
	return nil
}
func (a *Anonymous) stop() {
	if a.cancel != nil {
		a.cancel()
		<-a.done
		a.cancel = nil
	}
	a.setStatus(domain.ConnectionNotConnected, true)
}
func (a *Anonymous) Disconnect() { a.life.Lock(); defer a.life.Unlock(); a.stop() }
func (a *Anonymous) Close() {
	a.life.Lock()
	defer a.life.Unlock()
	if a.closed {
		return
	}
	a.closed = true
	a.stop()
}
func (a *Anonymous) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	for ctx.Err() == nil {
		_ = a.session(ctx)
		if ctx.Err() != nil {
			break
		}
		a.setStatus(domain.ConnectionReconnecting, true)
		timer := time.NewTimer(a.opts.RetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

type ircMessage struct {
	tags            map[string]string
	prefix, command string
	params          []string
	trailing        string
}

func parseIRC(line string) (ircMessage, bool) {
	m := ircMessage{tags: map[string]string{}}
	if len(line) > 16384 || strings.ContainsAny(line, "\r\n\x00") {
		return m, false
	}
	if strings.HasPrefix(line, "@") {
		p := strings.SplitN(line, " ", 2)
		if len(p) != 2 {
			return m, false
		}
		line = p[1]
		for _, t := range strings.Split(p[0][1:], ";") {
			kv := strings.SplitN(t, "=", 2)
			if len(kv) != 2 {
				continue
			}
			if _, ok := m.tags[kv[0]]; ok {
				return m, false
			}
			m.tags[kv[0]] = kv[1]
		}
	}
	if strings.HasPrefix(line, ":") {
		p := strings.SplitN(line, " ", 2)
		if len(p) != 2 {
			return m, false
		}
		m.prefix = p[0][1:]
		line = p[1]
	}
	if i := strings.Index(line, " :"); i >= 0 {
		m.trailing = line[i+2:]
		line = line[:i]
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return m, false
	}
	m.command = parts[0]
	m.params = parts[1:]
	return m, true
}
func numericID(s string) bool {
	if len(s) == 0 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != "0"
}
func messageID(s string) bool {
	if len(s) < 8 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func (a *Anonymous) session(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	c, err := a.opts.Dial(dialCtx)
	cancel()
	if err != nil {
		return err
	}
	defer c.Close()
	sessionCtx, stop := context.WithCancel(ctx)
	defer stop()
	joined := make(chan struct{})
	go func() { defer close(joined); <-sessionCtx.Done(); _ = c.Close() }()
	defer func() { stop(); <-joined }()
	var writeMu sync.Mutex
	write := func(s string) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err := fmt.Fprint(c, s+"\r\n")
		return err
	}
	if err := write("CAP REQ :twitch.tv/tags twitch.tv/commands\r\nPASS oauth:59301\r\nNICK justinfan123123\r\nJOIN #" + a.opts.Channel); err != nil {
		return err
	}
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				if write("PING :cuemender") != nil {
					_ = c.Close()
					return
				}
			}
		}
	}()
	defer func() { stop(); <-heartbeatDone }()
	scanner := bufio.NewScanner(c)
	scanner.Buffer(make([]byte, 4096), 16384)
	_ = c.SetReadDeadline(time.Now().Add(15 * time.Second))
	caps, channelJoined := false, false
	room := ""
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		m, ok := parseIRC(scanner.Text())
		if !ok {
			continue
		}
		if m.command == "PING" {
			if err := write("PONG :" + m.trailing); err != nil {
				return err
			}
			continue
		}
		if m.command == "RECONNECT" {
			return errors.New("server reconnect")
		}
		if m.command == "CAP" && len(m.params) >= 2 && m.params[1] == "ACK" {
			tags, commands := false, false
			for _, cap := range strings.Fields(m.trailing) {
				tags = tags || cap == "twitch.tv/tags"
				commands = commands || cap == "twitch.tv/commands"
			}
			caps = tags && commands
		}
		if m.command == "366" && len(m.params) >= 2 && m.params[1] == "#"+a.opts.Channel {
			channelJoined = true
		}
		if m.command == "ROOMSTATE" && len(m.params) == 1 && m.params[0] == "#"+a.opts.Channel && numericID(m.tags["room-id"]) {
			if room != "" && room != m.tags["room-id"] {
				return errors.New("room changed")
			}
			room = m.tags["room-id"]
		}
		if caps && channelJoined && room != "" {
			if a.Snapshot().Status != domain.ConnectionConnected {
				a.setStatus(domain.ConnectionConnected, false)
			}
			_ = c.SetReadDeadline(time.Now().Add(90 * time.Second))
		}
		if m.command != "PRIVMSG" || !caps || !channelJoined || room == "" {
			continue
		}
		event, ok := a.event(m, room, a.opts.Now())
		if !ok {
			continue
		}
		if a.dedup.Seen(event.DeliveryID, event.MessageID, a.opts.Now()) {
			continue
		}
		if event.Command.IsIssue() && !a.cooldown.Allow(event.ActorID, a.opts.Now()) {
			continue
		}
		a.mu.Lock()
		event.ConnectionGeneration = a.state.ConnectionGeneration
		a.state.Receipt.Accepted++
		a.state.Receipt.LastCommand = string(event.Command.Kind)
		a.state.Receipt.LastCategory = event.Command.Category
		t := event.ReceivedAt
		a.state.Receipt.LastAt = &t
		a.mu.Unlock()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if a.opts.OnCommand != nil {
			a.opts.OnCommand(event)
		}
		a.publish()
	}
	return scanner.Err()
}
func (a *Anonymous) event(m ircMessage, room string, now time.Time) (CommandEvent, bool) {
	bad := CommandEvent{}
	if len(m.params) != 1 || m.params[0] != "#"+a.opts.Channel || m.tags["room-id"] != room || !numericID(m.tags["user-id"]) || !messageID(m.tags["id"]) {
		return bad, false
	}
	// Shared-chat origin cannot be silently attributed to the local channel.
	if m.tags["source-room-id"] != "" || m.tags["source-id"] != "" || m.tags["user-id"] == room {
		return bad, false
	}
	sender, _, ok := strings.Cut(m.prefix, "!")
	if !ok || !ValidChannel(sender) || sender == a.opts.Channel {
		return bad, false
	}
	stamp, err := strconv.ParseInt(m.tags["tmi-sent-ts"], 10, 64)
	if err != nil {
		return bad, false
	}
	sent := time.UnixMilli(stamp).UTC()
	if sent.Before(now.Add(-30*time.Second)) || sent.After(now.Add(5*time.Second)) {
		return bad, false
	}
	command, ok := ParseCommand(m.trailing)
	if !ok || command.Kind == CommandStreamHelp {
		return bad, false
	}
	return CommandEvent{DeliveryID: "irc:" + m.tags["id"], MessageID: m.tags["id"], BroadcasterID: room, ActorID: m.tags["user-id"], ReceivedAt: sent, Command: command}, true
}
