package realtime

import (
	"log/slog"
	"sync"
)

// subscriberBuffer is how many encoded events a subscriber may lag behind
// before the hub drops it rather than block the relay.
const subscriberBuffer = 64

// DropReason says why the hub ended a subscription.
type DropReason int

const (
	// DropNone means the subscription is still active or was closed by its
	// owner.
	DropNone DropReason = iota
	// DropSlow means the subscriber's buffer was full when an event arrived.
	DropSlow
	// DropShutdown means the hub was closed.
	DropShutdown
)

// Hub fans encoded events out to subscribers. Publish never blocks: a
// subscriber that cannot keep up is dropped and expected to reconnect and
// refetch.
type Hub struct {
	log *slog.Logger

	mu     sync.Mutex
	subs   map[*Subscription]struct{}
	closed bool
}

// NewHub constructs an empty Hub.
func NewHub(log *slog.Logger) *Hub {
	return &Hub{log: log, subs: make(map[*Subscription]struct{})}
}

// Subscription is one subscriber's view of the hub.
type Subscription struct {
	hub    *Hub
	events chan []byte
	done   chan struct{}
	reason DropReason // written under hub.mu before done is closed
}

// Events delivers encoded envelopes in publish order.
func (s *Subscription) Events() <-chan []byte { return s.events }

// Done is closed when the subscription ends, by Close or by the hub.
func (s *Subscription) Done() <-chan struct{} { return s.done }

// Reason reports why the hub ended the subscription. It is only meaningful
// once Done is closed.
func (s *Subscription) Reason() DropReason {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	return s.reason
}

// Close unsubscribes. It is safe to call more than once and after the hub has
// dropped the subscription.
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	s.hub.removeLocked(s, DropNone)
}

// Subscribe registers a new subscriber. Subscribing to a closed hub returns a
// subscription that is already done.
func (h *Hub) Subscribe() *Subscription {
	s := &Subscription{
		hub:    h,
		events: make(chan []byte, subscriberBuffer),
		done:   make(chan struct{}),
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		s.reason = DropShutdown
		close(s.done)
		return s
	}
	h.subs[s] = struct{}{}
	return s
}

// Publish encodes e once and offers it to every subscriber.
func (h *Hub) Publish(e Event) {
	msg, err := MarshalEnvelope(e)
	if err != nil {
		// The payload came from a JSONB column, so this indicates a bug rather
		// than bad input; skip the event instead of wedging the relay.
		h.log.Error("realtime: encode event failed",
			slog.Int64("event_id", e.ID), slog.String("error", err.Error()))
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.events <- msg:
		default:
			h.removeLocked(s, DropSlow)
			h.log.Warn("realtime: dropped slow subscriber", slog.Int64("event_id", e.ID))
		}
	}
}

// Len reports the number of active subscribers.
func (h *Hub) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// Close ends every subscription with DropShutdown and rejects new ones.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for s := range h.subs {
		h.removeLocked(s, DropShutdown)
	}
}

// removeLocked ends s if it is still registered. The events channel is never
// closed, so a concurrent Publish can never send on a closed channel.
func (h *Hub) removeLocked(s *Subscription, reason DropReason) {
	if _, ok := h.subs[s]; !ok {
		return
	}
	delete(h.subs, s)
	s.reason = reason
	close(s.done)
}
