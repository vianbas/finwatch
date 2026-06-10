// Package alerts manages the alert lifecycle: alerts are raised when rules match
// synthetic transactions, then transition open -> acknowledged -> resolved.
// Raising and each transition write through the transactional outbox / use
// conditional updates so concurrent callers cannot lose updates.
package alerts

import (
	"errors"
	"time"
)

// Status is an alert's lifecycle state.
type Status string

const (
	StatusOpen         Status = "open"
	StatusAcknowledged Status = "acknowledged"
	StatusResolved     Status = "resolved"
)

// Alert is a single raised alert.
type Alert struct {
	ID            string
	RuleID        string
	TransactionID string
	Status        Status
	Severity      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Sentinel errors surfaced to the transport layer.
var (
	ErrNotFound          = errors.New("alerts: alert not found")
	ErrInvalidTransition = errors.New("alerts: invalid status transition")
)
