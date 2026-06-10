// Package rules holds generic, declarative operational rules and a deterministic
// engine that evaluates them against synthetic transactions. Rules are
// intentionally simple (field/operator/value) and not derived from any real
// institution.
package rules

import (
	"context"
	"strconv"

	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

// Field is the transaction attribute a rule inspects.
type Field string

const (
	FieldAmountMinor Field = "amount_minor"
	FieldCurrency    Field = "currency"
	FieldDirection   Field = "direction"
)

// Operator is the comparison a rule applies.
type Operator string

const (
	OpGTE Operator = "gte"
	OpLTE Operator = "lte"
	OpEq  Operator = "eq"
)

// Rule is a single declarative rule. Value is stored as text and interpreted
// per field (an integer for amount_minor, a literal otherwise).
type Rule struct {
	ID       string
	Name     string
	Field    Field
	Operator Operator
	Value    string
	Severity string
	Enabled  bool
}

// Match is a rule that fired for a transaction.
type Match struct {
	RuleID   string
	Severity string
}

// Evaluate returns the matches for t across the enabled rules. It is pure and
// deterministic: the same inputs always yield the same ordered output.
func Evaluate(t transactions.Transaction, rs []Rule) []Match {
	matches := make([]Match, 0)
	for _, r := range rs {
		if r.Enabled && r.Matches(t) {
			matches = append(matches, Match{RuleID: r.ID, Severity: r.Severity})
		}
	}
	return matches
}

// Matches reports whether the rule fires for the given transaction. Unknown
// field/operator combinations never match.
func (r Rule) Matches(t transactions.Transaction) bool {
	switch r.Field {
	case FieldAmountMinor:
		v, err := strconv.ParseInt(r.Value, 10, 64)
		if err != nil {
			return false
		}
		switch r.Operator {
		case OpGTE:
			return t.AmountMinor >= v
		case OpLTE:
			return t.AmountMinor <= v
		case OpEq:
			return t.AmountMinor == v
		}
	case FieldCurrency:
		if r.Operator == OpEq {
			return t.Currency == r.Value
		}
	case FieldDirection:
		if r.Operator == OpEq {
			return string(t.Direction) == r.Value
		}
	}
	return false
}

// Repository lists enabled rules and seeds the generic defaults.
type Repository interface {
	ListEnabled(ctx context.Context) ([]Rule, error)
	EnsureDefaults(ctx context.Context) error
}

// DefaultRules are generic, synthetic example rules used to demonstrate
// evaluation. They are not derived from any real institution or threshold.
var DefaultRules = []Rule{
	{Name: "high-value-debit", Field: FieldAmountMinor, Operator: OpGTE, Value: "500000", Severity: "high", Enabled: true},
	{Name: "very-high-value", Field: FieldAmountMinor, Operator: OpGTE, Value: "900000", Severity: "critical", Enabled: true},
	{Name: "gbp-currency", Field: FieldCurrency, Operator: OpEq, Value: "GBP", Severity: "low", Enabled: true},
}
