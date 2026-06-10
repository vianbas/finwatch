package rules_test

import (
	"testing"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/rules"
	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

func makeTxn(amount int64, currency string, dir transactions.Direction) transactions.Transaction {
	return transactions.Transaction{
		AmountMinor: amount,
		Currency:    currency,
		Direction:   dir,
		OccurredAt:  time.Now().UTC(),
	}
}

func enabledRule(id string, field rules.Field, op rules.Operator, value, sev string) rules.Rule {
	return rules.Rule{ID: id, Name: id, Field: field, Operator: op, Value: value, Severity: sev, Enabled: true}
}

func TestEvaluate_AmountMinor(t *testing.T) {
	tests := []struct {
		name    string
		op      rules.Operator
		value   string
		amount  int64
		wantHit bool
	}{
		{"gte_exact", rules.OpGTE, "1000", 1000, true},
		{"gte_above", rules.OpGTE, "1000", 1001, true},
		{"gte_below", rules.OpGTE, "1000", 999, false},
		{"lte_exact", rules.OpLTE, "1000", 1000, true},
		{"lte_below", rules.OpLTE, "1000", 999, true},
		{"lte_above", rules.OpLTE, "1000", 1001, false},
		{"eq_match", rules.OpEq, "1000", 1000, true},
		{"eq_no_match", rules.OpEq, "1000", 999, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := enabledRule("r", rules.FieldAmountMinor, tc.op, tc.value, "high")
			ms := rules.Evaluate(makeTxn(tc.amount, "USD", transactions.DirectionDebit), []rules.Rule{r})
			if hit := len(ms) > 0; hit != tc.wantHit {
				t.Errorf("hit=%v, want %v", hit, tc.wantHit)
			}
		})
	}
}

func TestEvaluate_Currency(t *testing.T) {
	r := enabledRule("r", rules.FieldCurrency, rules.OpEq, "GBP", "low")
	if ms := rules.Evaluate(makeTxn(100, "GBP", transactions.DirectionDebit), []rules.Rule{r}); len(ms) != 1 {
		t.Fatalf("want 1 match for GBP, got %d", len(ms))
	}
	if ms := rules.Evaluate(makeTxn(100, "USD", transactions.DirectionDebit), []rules.Rule{r}); len(ms) != 0 {
		t.Fatalf("want 0 matches for USD, got %d", len(ms))
	}
}

func TestEvaluate_Direction(t *testing.T) {
	r := enabledRule("r", rules.FieldDirection, rules.OpEq, "debit", "low")
	if ms := rules.Evaluate(makeTxn(100, "USD", transactions.DirectionDebit), []rules.Rule{r}); len(ms) != 1 {
		t.Fatalf("want 1 match for debit, got %d", len(ms))
	}
	if ms := rules.Evaluate(makeTxn(100, "USD", transactions.DirectionCredit), []rules.Rule{r}); len(ms) != 0 {
		t.Fatalf("want 0 matches for credit, got %d", len(ms))
	}
}

func TestEvaluate_DisabledRuleSkipped(t *testing.T) {
	r := rules.Rule{ID: "r", Field: rules.FieldAmountMinor, Operator: rules.OpGTE, Value: "0", Severity: "low", Enabled: false}
	if ms := rules.Evaluate(makeTxn(9999, "USD", transactions.DirectionDebit), []rules.Rule{r}); len(ms) != 0 {
		t.Fatalf("disabled rule must not fire, got %d matches", len(ms))
	}
}

func TestEvaluate_InvalidAmountValue_Skipped(t *testing.T) {
	r := enabledRule("r", rules.FieldAmountMinor, rules.OpGTE, "not-a-number", "low")
	if ms := rules.Evaluate(makeTxn(9999, "USD", transactions.DirectionDebit), []rules.Rule{r}); len(ms) != 0 {
		t.Fatalf("rule with invalid value must not fire, got %d matches", len(ms))
	}
}

func TestEvaluate_UnknownField_Skipped(t *testing.T) {
	r := enabledRule("r", rules.Field("unknown"), rules.OpEq, "x", "low")
	if ms := rules.Evaluate(makeTxn(100, "USD", transactions.DirectionDebit), []rules.Rule{r}); len(ms) != 0 {
		t.Fatalf("unknown field must not fire, got %d matches", len(ms))
	}
}

func TestEvaluate_CurrencyWithNonEqOperator_Skipped(t *testing.T) {
	r := enabledRule("r", rules.FieldCurrency, rules.OpGTE, "GBP", "low")
	if ms := rules.Evaluate(makeTxn(100, "GBP", transactions.DirectionDebit), []rules.Rule{r}); len(ms) != 0 {
		t.Fatalf("non-eq operator on currency must not fire, got %d matches", len(ms))
	}
}

func TestEvaluate_MultipleRules_AllMatchingFire(t *testing.T) {
	rs := []rules.Rule{
		enabledRule("r1", rules.FieldAmountMinor, rules.OpGTE, "500000", "high"),
		enabledRule("r2", rules.FieldAmountMinor, rules.OpGTE, "900000", "critical"),
		enabledRule("r3", rules.FieldCurrency, rules.OpEq, "GBP", "low"),
	}
	// $9000 USD: both amount rules fire, currency does not.
	ms := rules.Evaluate(makeTxn(900000, "USD", transactions.DirectionDebit), rs)
	if len(ms) != 2 {
		t.Fatalf("want 2 matches for USD, got %d", len(ms))
	}
	// $9000 GBP: all three fire.
	ms = rules.Evaluate(makeTxn(900000, "GBP", transactions.DirectionDebit), rs)
	if len(ms) != 3 {
		t.Fatalf("want 3 matches for GBP, got %d", len(ms))
	}
}

func TestEvaluate_Match_CarriesRuleIDAndSeverity(t *testing.T) {
	r := enabledRule("abc-123", rules.FieldAmountMinor, rules.OpGTE, "1", "critical")
	ms := rules.Evaluate(makeTxn(1, "USD", transactions.DirectionDebit), []rules.Rule{r})
	if len(ms) != 1 {
		t.Fatalf("want 1 match, got %d", len(ms))
	}
	if ms[0].RuleID != "abc-123" || ms[0].Severity != "critical" {
		t.Errorf("unexpected match %+v", ms[0])
	}
}
