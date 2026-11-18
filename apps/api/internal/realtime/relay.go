package realtime

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	// relayBatchSize bounds how many outbox rows one fetch reads.
	relayBatchSize = 100
	// maxRelayBackoff caps the wait between polls while the outbox is failing.
	maxRelayBackoff = 30 * time.Second
)

// Outbox is the relay's view of the outbox_events table.
type Outbox interface {
	// ListUnpublished returns up to limit unpublished events, oldest first.
	ListUnpublished(ctx context.Context, limit int) ([]Event, error)
	// MarkPublished stamps published_at on the given event ids.
	MarkPublished(ctx context.Context, ids []int64) error
}

// Publisher receives relayed events. Publish must not block.
type Publisher interface {
	Publish(e Event)
}

// Relay polls the outbox and hands unpublished events to a Publisher, marking
// them published afterwards. An event is published before it is marked, so a
// failed mark leads to redelivery (at-least-once), never to loss.
//
// The relay assumes a single running instance, matching the single-process
// deployment in ADR 0003.
type Relay struct {
	outbox   Outbox
	pub      Publisher
	interval time.Duration
	log      *slog.Logger
}

// NewRelay constructs a Relay that polls every interval.
func NewRelay(outbox Outbox, pub Publisher, interval time.Duration, log *slog.Logger) *Relay {
	return &Relay{outbox: outbox, pub: pub, interval: interval, log: log}
}

// Run polls until ctx is cancelled. Consecutive failures back off
// exponentially up to maxRelayBackoff.
func (r *Relay) Run(ctx context.Context) {
	wait := r.interval
	timer := time.NewTimer(wait)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		if _, err := r.drain(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			wait = min(wait*2, maxRelayBackoff)
			r.log.Error("realtime: relay outbox failed",
				slog.String("error", err.Error()), slog.Duration("retry_in", wait))
		} else {
			wait = r.interval
		}
		timer.Reset(wait)
	}
}

// drain relays batches until the outbox has no more unpublished events,
// returning how many events were published.
func (r *Relay) drain(ctx context.Context) (int, error) {
	total := 0
	for {
		events, err := r.outbox.ListUnpublished(ctx, relayBatchSize)
		if err != nil {
			return total, fmt.Errorf("list unpublished: %w", err)
		}
		if len(events) == 0 {
			return total, nil
		}

		ids := make([]int64, len(events))
		for i, e := range events {
			r.pub.Publish(e)
			ids[i] = e.ID
		}
		if err := r.outbox.MarkPublished(ctx, ids); err != nil {
			return total, fmt.Errorf("mark published: %w", err)
		}
		total += len(events)

		if len(events) < relayBatchSize {
			return total, nil
		}
	}
}
