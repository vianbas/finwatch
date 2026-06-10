package transactions

import (
	"errors"
	"testing"
	"time"
)

func TestNew_Valid(t *testing.T) {
	loc := time.FixedZone("UTC+1", 3600)
	occurred := time.Date(2026, 1, 2, 15, 4, 5, 0, loc)

	tx, err := New("ACC-1001", 1500, "USD", DirectionDebit, occurred)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Status != StatusPosted {
		t.Errorf("status = %q, want posted", tx.Status)
	}
	if tx.AmountMinor != 1500 {
		t.Errorf("amount = %d, want 1500", tx.AmountMinor)
	}
	if tx.OccurredAt.Location() != time.UTC {
		t.Errorf("occurredAt location = %v, want UTC", tx.OccurredAt.Location())
	}
	if !tx.OccurredAt.Equal(occurred) {
		t.Errorf("occurredAt instant changed: got %v want %v", tx.OccurredAt, occurred)
	}
}

func TestNew_Errors(t *testing.T) {
	valid := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	tests := []struct {
		name      string
		account   string
		amount    int64
		currency  string
		direction Direction
		occurred  time.Time
		want      error
	}{
		{"empty account", "", 100, "USD", DirectionDebit, valid, ErrAccountRequired},
		{"zero amount", "ACC-1", 0, "USD", DirectionDebit, valid, ErrAmountNotPositive},
		{"negative amount", "ACC-1", -5, "USD", DirectionDebit, valid, ErrAmountNotPositive},
		{"lowercase currency", "ACC-1", 100, "usd", DirectionDebit, valid, ErrCurrencyInvalid},
		{"short currency", "ACC-1", 100, "US", DirectionDebit, valid, ErrCurrencyInvalid},
		{"long currency", "ACC-1", 100, "USDD", DirectionDebit, valid, ErrCurrencyInvalid},
		{"bad direction", "ACC-1", 100, "USD", Direction("sideways"), valid, ErrDirectionInvalid},
		{"zero time", "ACC-1", 100, "USD", DirectionDebit, time.Time{}, ErrOccurredAtRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.account, tt.amount, tt.currency, tt.direction, tt.occurred)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
