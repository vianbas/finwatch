package realtime

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testEvent(id int64) Event {
	return Event{
		ID:         id,
		Type:       "alert.raised",
		Payload:    json.RawMessage(`{"alertId":"00000000-0000-0000-0000-000000000001"}`),
		OccurredAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func receive(t *testing.T, s *Subscription) envelope {
	t.Helper()
	select {
	case msg := <-s.Events():
		var env envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		return env
	default:
		t.Fatal("no event buffered")
		return envelope{}
	}
}

func isDone(s *Subscription) bool {
	select {
	case <-s.Done():
		return true
	default:
		return false
	}
}

func TestMarshalEnvelope(t *testing.T) {
	e := testEvent(42)
	e.OccurredAt = time.Date(2026, 1, 2, 10, 4, 5, 0, time.FixedZone("UTC+7", 7*3600))
	got, err := MarshalEnvelope(e)
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}
	want := `{"id":"42","type":"alert.raised","version":1,"occurredAt":"2026-01-02T03:04:05Z",` +
		`"data":{"alertId":"00000000-0000-0000-0000-000000000001"}}`
	if string(got) != want {
		t.Errorf("envelope =\n%s\nwant\n%s", got, want)
	}
}

func TestHub_FansOutToEverySubscriber(t *testing.T) {
	h := NewHub(discardLogger())
	a, b := h.Subscribe(), h.Subscribe()

	h.Publish(testEvent(1))

	for _, s := range []*Subscription{a, b} {
		if env := receive(t, s); env.ID != "1" || env.Type != "alert.raised" {
			t.Errorf("got %+v", env)
		}
	}
}

func TestHub_DropsSlowSubscriberWithoutBlocking(t *testing.T) {
	h := NewHub(discardLogger())
	slow, fast := h.Subscribe(), h.Subscribe()

	for i := range subscriberBuffer {
		h.Publish(testEvent(int64(i)))
		receive(t, fast)
	}
	if isDone(slow) {
		t.Fatal("slow subscriber dropped before its buffer was full")
	}

	h.Publish(testEvent(999)) // must not block

	if !isDone(slow) || slow.Reason() != DropSlow {
		t.Fatalf("slow subscriber: done=%v reason=%v, want done with DropSlow", isDone(slow), slow.Reason())
	}
	if env := receive(t, fast); env.ID != "999" {
		t.Errorf("fast subscriber got id %q, want 999", env.ID)
	}
	if h.Len() != 1 {
		t.Errorf("Len = %d, want 1", h.Len())
	}
}

func TestSubscription_CloseUnsubscribes(t *testing.T) {
	h := NewHub(discardLogger())
	s := h.Subscribe()

	s.Close()
	s.Close() // idempotent

	if !isDone(s) || s.Reason() != DropNone {
		t.Fatalf("done=%v reason=%v, want done with DropNone", isDone(s), s.Reason())
	}
	if h.Len() != 0 {
		t.Errorf("Len = %d, want 0", h.Len())
	}
	h.Publish(testEvent(1))
	if len(s.Events()) != 0 {
		t.Error("closed subscription still received an event")
	}
}

func TestHub_CloseEndsSubscriptionsAndRejectsNewOnes(t *testing.T) {
	h := NewHub(discardLogger())
	s := h.Subscribe()

	h.Close()

	if !isDone(s) || s.Reason() != DropShutdown {
		t.Fatalf("done=%v reason=%v, want done with DropShutdown", isDone(s), s.Reason())
	}
	late := h.Subscribe()
	if !isDone(late) || late.Reason() != DropShutdown {
		t.Errorf("subscription after Close: done=%v reason=%v", isDone(late), late.Reason())
	}
	if h.Len() != 0 {
		t.Errorf("Len = %d, want 0", h.Len())
	}
}
