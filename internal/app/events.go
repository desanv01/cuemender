package app

import (
	"sync"

	"livecheck/internal/httpui"
)

type hubSubscriber struct {
	ch   chan []byte
	done chan struct{}
}

type hub struct {
	mu   sync.Mutex
	subs map[uint64]*hubSubscriber
	next uint64
}

func newHub() *hub {
	return &hub{subs: make(map[uint64]*hubSubscriber)}
}

func (h *hub) subscribe(initial []byte) httpui.Subscription {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	id := h.next
	sub := &hubSubscriber{
		ch:   make(chan []byte, 1),
		done: make(chan struct{}),
	}
	if initial != nil {
		sub.ch <- initial
	}
	h.subs[id] = sub
	return httpui.Subscription{
		C:    sub.ch,
		Done: sub.done,
		Cancel: func() {
			h.remove(id)
		},
	}
}

func (h *hub) remove(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subs[id]
	if !ok {
		return
	}
	delete(h.subs, id)
	close(sub.done)
}

// publish never blocks on a slow SSE client. The subscriber buffer holds only
// the latest snapshot; an older pending snapshot is replaced.
func (h *hub) publish(data []byte) {
	h.mu.Lock()
	subs := make([]*hubSubscriber, 0, len(h.subs))
	for _, sub := range h.subs {
		subs = append(subs, sub)
	}
	h.mu.Unlock()
	for _, sub := range subs {
		select {
		case sub.ch <- data:
		default:
			select {
			case <-sub.ch:
			default:
			}
			select {
			case sub.ch <- data:
			default:
			}
		}
	}
}
