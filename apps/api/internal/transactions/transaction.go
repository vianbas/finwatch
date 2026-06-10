// Package transactions is the synthetic-transaction feature module. It holds the
// domain model, validation, a synthetic data generator, and the service that
// orchestrates ingestion and listing. It is independent of HTTP and of any
// specific database: persistence is reached through the Repository port.
//
// All data produced and stored here is synthetic.
package transactions

import (
	"errors"
	"regexp"
	"time"
)

// Direction is the movement of money relative to the account.
type Direction string

const (
	DirectionDebit  Direction = "debit"
	DirectionCredit Direction = "credit"
)

// Valid reports whether d is a known direction.
func (d Direction) Valid() bool {
	return d == DirectionDebit || d == DirectionCredit
}

// Status is the lifecycle state of a transaction. The bootstrap models only a
// single terminal state; richer lifecycles arrive with later features.
type Status string

const StatusPosted Status = "posted"

// Transaction is a single synthetic monetary movement. AmountMinor is in integer
// minor units (e.g. cents); OccurredAt is UTC.
type Transaction struct {
	ID          string
	AccountID   string
	AmountMinor int64
	Currency    string
	Direction   Direction
	Status      Status
	OccurredAt  time.Time
	CreatedAt   time.Time
}

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Validation errors returned by New.
var (
	ErrAccountRequired    = errors.New("transactions: account id is required")
	ErrAmountNotPositive  = errors.New("transactions: amount minor units must be positive")
	ErrCurrencyInvalid    = errors.New("transactions: currency must be a 3-letter uppercase ISO code")
	ErrDirectionInvalid   = errors.New("transactions: direction must be debit or credit")
	ErrOccurredAtRequired = errors.New("transactions: occurred at is required")
)

// New builds a validated Transaction in the posted status. ID and CreatedAt are
// assigned by the store on persistence; OccurredAt is normalised to UTC.
func New(accountID string, amountMinor int64, currency string, direction Direction, occurredAt time.Time) (Transaction, error) {
	switch {
	case accountID == "":
		return Transaction{}, ErrAccountRequired
	case amountMinor <= 0:
		return Transaction{}, ErrAmountNotPositive
	case !currencyPattern.MatchString(currency):
		return Transaction{}, ErrCurrencyInvalid
	case !direction.Valid():
		return Transaction{}, ErrDirectionInvalid
	case occurredAt.IsZero():
		return Transaction{}, ErrOccurredAtRequired
	}
	return Transaction{
		AccountID:   accountID,
		AmountMinor: amountMinor,
		Currency:    currency,
		Direction:   direction,
		Status:      StatusPosted,
		OccurredAt:  occurredAt.UTC(),
	}, nil
}
