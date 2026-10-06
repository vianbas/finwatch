package realtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeOutbox is an in-memory Outbox. markErrs are returned by successive
// MarkPublished calls before it starts succeeding.
type fakeOutbox struct {
	mu        sync.Mutex
	events    []Event
	published map[int64]bool
	listErr   error
	markErrs  []error
}

func newFakeOutbox(n int) *fakeOutbox {
	o := &fakeOutbox{published: make(map[int64]bool)}
	for i := 1; i <= n; i++ {
		o.events = append(o.events, testEvent(int64(i)))
	}
	return o
}

func (o *fakeOutbox) ListUnpublished(_ context.Context, limit int) ([]Event, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.listErr != nil {
		return nil, o.listErr
	}
	var out []Event
	for _, e := range o.events {
		if !o.published[e.ID] && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (o *fakeOutbox) MarkPublished(_ context.Context, ids []int64) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.markErrs) > 0 {
		err := o.markErrs[0]
		o.markErrs = o.markErrs[1:]
		return err
	}
	for _, id := range ids {
		o.published[id] = true
	}
	return nil
}

func (o *fakeOutbox) unpublished() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.events) - len(o.published)
}

type recordingPublisher struct {
	mu  sync.Mutex
	ids []int64
}

func (p *recordingPublisher) Publish(e Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ids = append(p.ids, e.ID)
}

func (p *recordingPublisher) published() []int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]int64(nil), p.ids...)
}

func TestRelay_DrainPublishesInOrderAcrossBatches(t *testing.T) {
	n := relayBatchSize*2 + 5
	outbox := newFakeOutbox(n)
	pub := &recordingPublisher{}
	r := NewRelay(outbox, pub, time.Millisecond, discardLogger())

	got, err := r.drain(context.Background())
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got != n {
		t.Errorf("drained %d, want %d", got, n)
	}
	ids := pub.published()
	if len(ids) != n {
		t.Fatalf("published %d events, want %d", len(ids), n)
	}
	for i, id := range ids {
		if id != int64(i+1) {
			t.Fatalf("published[%d] = %d, want %d", i, id, i+1)
		}
	}
	if outbox.unpublished() != 0 {
		t.Errorf("%d events left unpublished", outbox.unpublished())
	}
}

func TestRelay_FailedMarkRedeliversSameEvents(t *testing.T) {
	outbox := newFakeOutbox(2)
	outbox.markErrs = []error{errors.New("connection reset")}
	pub := &recordingPublisher{}
	r := NewRelay(outbox, pub, time.Millisecond, discardLogger())

	if _, err := r.drain(context.Background()); err == nil {
		t.Fatal("first drain: want mark error, got nil")
	}
	if outbox.unpublished() != 2 {
		t.Fatalf("unpublished = %d after failed mark, want 2", outbox.unpublished())
	}
	if _, err := r.drain(context.Background()); err != nil {
		t.Fatalf("second drain: %v", err)
	}

	// At-least-once: both events reach the hub twice with stable ids, and
	// nothing is lost.
	want := []int64{1, 2, 1, 2}
	got := pub.published()
	if len(got) != len(want) {
		t.Fatalf("published %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("published %v, want %v", got, want)
		}
	}
	if outbox.unpublished() != 0 {
		t.Errorf("%d events left unpublished", outbox.unpublished())
	}
}

func TestRelay_ListErrorPublishesNothing(t *testing.T) {
	outbox := newFakeOutbox(3)
	outbox.listErr = errors.New("database unavailable")
	pub := &recordingPublisher{}
	r := NewRelay(outbox, pub, time.Millisecond, discardLogger())

	if _, err := r.drain(context.Background()); err == nil {
		t.Fatal("want list error, got nil")
	}
	if len(pub.published()) != 0 {
		t.Errorf("published %v, want none", pub.published())
	}
}

func TestRelay_RunRecoversAndStopsOnCancel(t *testing.T) {
	outbox := newFakeOutbox(3)
	outbox.markErrs = []error{errors.New("transient")}
	pub := &recordingPublisher{}
	r := NewRelay(outbox, pub, time.Millisecond, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Run(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for outbox.unpublished() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("relay did not publish all events after a transient failure")
		}
		time.Sleep(time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
