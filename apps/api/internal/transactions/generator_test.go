package transactions

import (
	"testing"
	"time"
)

// fixedGenerator returns a generator with a deterministic seed and clock.
func fixedGenerator(seed int64) *Generator {
	g := NewGenerator(seed)
	g.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	return g
}

func TestGenerator_Deterministic(t *testing.T) {
	a := fixedGenerator(42)
	b := fixedGenerator(42)
	for i := 0; i < 25; i++ {
		ta, tb := a.Next(), b.Next()
		if ta.AccountID != tb.AccountID || ta.AmountMinor != tb.AmountMinor ||
			ta.Currency != tb.Currency || ta.Direction != tb.Direction ||
			!ta.OccurredAt.Equal(tb.OccurredAt) {
			t.Fatalf("non-deterministic at %d: %+v vs %+v", i, ta, tb)
		}
	}
}

func TestGenerator_ProducesValid(t *testing.T) {
	g := fixedGenerator(7)
	for i := 0; i < 50; i++ {
		tx := g.Next()
		if _, err := New(tx.AccountID, tx.AmountMinor, tx.Currency, tx.Direction, tx.OccurredAt); err != nil {
			t.Fatalf("generated invalid transaction: %+v (%v)", tx, err)
		}
		if tx.AmountMinor <= 0 {
			t.Fatalf("non-positive amount: %d", tx.AmountMinor)
		}
		if tx.OccurredAt.Location() != time.UTC {
			t.Fatalf("occurredAt not UTC: %v", tx.OccurredAt.Location())
		}
	}
}
