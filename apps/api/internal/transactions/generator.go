package transactions

import (
	"math/rand"
	"time"
)

// Synthetic value sets. These are deliberately generic placeholders — they do
// not represent any real institution, account, or customer.
var (
	genAccounts   = []string{"ACC-1001", "ACC-1002", "ACC-1003", "ACC-1004", "ACC-1005"}
	genCurrencies = []string{"USD", "EUR", "GBP", "SGD"}
	genDirections = []Direction{DirectionDebit, DirectionCredit}
)

// Generator produces synthetic Transaction values. It is seeded for
// reproducibility and accepts an injectable clock so tests are deterministic.
type Generator struct {
	rng *rand.Rand
	now func() time.Time
}

// NewGenerator returns a Generator seeded with the given value.
func NewGenerator(seed int64) *Generator {
	return &Generator{
		rng: rand.New(rand.NewSource(seed)),
		now: time.Now,
	}
}

// Next returns one valid synthetic Transaction with a recent occurrence time.
// Amount is in integer minor units between 1.00 and ~10,000.00.
func (g *Generator) Next() Transaction {
	amountMinor := int64(g.rng.Intn(1_000_000) + 100)
	occurredAt := g.now().UTC().Add(-time.Duration(g.rng.Intn(72)) * time.Hour)

	// Inputs are drawn from valid sets, so New never returns an error here.
	t, _ := New(
		genAccounts[g.rng.Intn(len(genAccounts))],
		amountMinor,
		genCurrencies[g.rng.Intn(len(genCurrencies))],
		genDirections[g.rng.Intn(len(genDirections))],
		occurredAt,
	)
	return t
}
