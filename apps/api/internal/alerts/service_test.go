package alerts_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/alerts"
	"github.com/vianbas/finwatch/apps/api/internal/rules"
	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

// --- fakes ---

type fakeAlertRepo struct {
	store   map[string]alerts.Alert
	raised  []alerts.Raise
	listErr error

	// Concurrency simulation: when failNextTransition is true, the next
	// Transition call returns ErrNotFound once; subsequent Get calls then
	// return postRaceAlert instead of the store value.
	failNextTransition bool
	postRaceAlert      *alerts.Alert
	transitionFailed   bool
}

func newFakeAlertRepo() *fakeAlertRepo {
	return &fakeAlertRepo{store: make(map[string]alerts.Alert)}
}

func (f *fakeAlertRepo) put(a alerts.Alert) { f.store[a.ID] = a }

func (f *fakeAlertRepo) Raise(_ context.Context, r alerts.Raise) (alerts.Alert, bool, error) {
	f.raised = append(f.raised, r)
	a := alerts.Alert{
		ID:            "alert-" + r.RuleID,
		RuleID:        r.RuleID,
		TransactionID: r.TransactionID,
		Status:        alerts.StatusOpen,
		Severity:      r.Severity,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	f.store[a.ID] = a
	return a, true, nil
}

func (f *fakeAlertRepo) Get(_ context.Context, id string) (alerts.Alert, error) {
	if f.transitionFailed && f.postRaceAlert != nil {
		return *f.postRaceAlert, nil
	}
	a, ok := f.store[id]
	if !ok {
		return alerts.Alert{}, alerts.ErrNotFound
	}
	return a, nil
}

func (f *fakeAlertRepo) List(_ context.Context, _ alerts.PageQuery) (alerts.Page, error) {
	return alerts.Page{}, f.listErr
}

func (f *fakeAlertRepo) Transition(_ context.Context, id string, _, to alerts.Status) (alerts.Alert, error) {
	if f.failNextTransition && !f.transitionFailed {
		f.transitionFailed = true
		return alerts.Alert{}, alerts.ErrNotFound
	}
	a, ok := f.store[id]
	if !ok {
		return alerts.Alert{}, alerts.ErrNotFound
	}
	a.Status = to
	a.UpdatedAt = time.Now()
	f.store[id] = a
	return a, nil
}

type fakeRulesRepo struct{ rules []rules.Rule }

func (f *fakeRulesRepo) ListEnabled(_ context.Context) ([]rules.Rule, error) { return f.rules, nil }
func (f *fakeRulesRepo) EnsureDefaults(_ context.Context) error              { return nil }

func openAlert(id string) alerts.Alert {
	return alerts.Alert{ID: id, RuleID: "r1", TransactionID: "t1", Status: alerts.StatusOpen, Severity: "high", CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func alertWith(id string, status alerts.Status) alerts.Alert {
	a := openAlert(id)
	a.Status = status
	return a
}

func newSvc(repo alerts.Repository) *alerts.Service {
	return alerts.NewService(repo, &fakeRulesRepo{}, slog.Default())
}

// --- state machine tests ---

func TestService_Acknowledge_OpenToAcknowledged(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.put(openAlert("a1"))
	got, err := newSvc(repo).Acknowledge(context.Background(), "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != alerts.StatusAcknowledged {
		t.Errorf("status=%q, want acknowledged", got.Status)
	}
}

func TestService_Acknowledge_AlreadyAcknowledged_IsIdempotent(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.put(alertWith("a1", alerts.StatusAcknowledged))
	got, err := newSvc(repo).Acknowledge(context.Background(), "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != alerts.StatusAcknowledged {
		t.Errorf("status=%q, want acknowledged", got.Status)
	}
}

func TestService_Acknowledge_Resolved_ReturnsInvalidTransition(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.put(alertWith("a1", alerts.StatusResolved))
	_, err := newSvc(repo).Acknowledge(context.Background(), "a1")
	if !errors.Is(err, alerts.ErrInvalidTransition) {
		t.Fatalf("got %v, want ErrInvalidTransition", err)
	}
}

func TestService_Resolve_AcknowledgedToResolved(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.put(alertWith("a1", alerts.StatusAcknowledged))
	got, err := newSvc(repo).Resolve(context.Background(), "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != alerts.StatusResolved {
		t.Errorf("status=%q, want resolved", got.Status)
	}
}

func TestService_Resolve_AlreadyResolved_IsIdempotent(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.put(alertWith("a1", alerts.StatusResolved))
	got, err := newSvc(repo).Resolve(context.Background(), "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != alerts.StatusResolved {
		t.Errorf("status=%q, want resolved", got.Status)
	}
}

func TestService_Resolve_Open_ReturnsInvalidTransition(t *testing.T) {
	repo := newFakeAlertRepo()
	repo.put(openAlert("a1"))
	_, err := newSvc(repo).Resolve(context.Background(), "a1")
	if !errors.Is(err, alerts.ErrInvalidTransition) {
		t.Fatalf("got %v, want ErrInvalidTransition", err)
	}
}

func TestService_Acknowledge_NotFound(t *testing.T) {
	_, err := newSvc(newFakeAlertRepo()).Acknowledge(context.Background(), "no-such-id")
	if !errors.Is(err, alerts.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestService_Acknowledge_ConcurrentRace_AlreadyAtTarget(t *testing.T) {
	// Simulate: Get says open, Transition returns ErrNotFound (concurrent
	// change), re-Get returns acknowledged → idempotent success.
	repo := newFakeAlertRepo()
	repo.put(openAlert("a1"))
	raced := alertWith("a1", alerts.StatusAcknowledged)
	repo.failNextTransition = true
	repo.postRaceAlert = &raced

	got, err := newSvc(repo).Acknowledge(context.Background(), "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != alerts.StatusAcknowledged {
		t.Errorf("status=%q, want acknowledged", got.Status)
	}
}

func TestService_Acknowledge_ConcurrentRace_ConflictingStatus(t *testing.T) {
	// Simulate: Get says open, Transition returns ErrNotFound, re-Get returns
	// resolved → ErrInvalidTransition.
	repo := newFakeAlertRepo()
	repo.put(openAlert("a1"))
	raced := alertWith("a1", alerts.StatusResolved)
	repo.failNextTransition = true
	repo.postRaceAlert = &raced

	_, err := newSvc(repo).Acknowledge(context.Background(), "a1")
	if !errors.Is(err, alerts.ErrInvalidTransition) {
		t.Fatalf("got %v, want ErrInvalidTransition", err)
	}
}

func TestService_TransactionIngested_RaisesAlertPerMatch(t *testing.T) {
	rulesRepo := &fakeRulesRepo{rules: []rules.Rule{
		{ID: "r1", Name: "high-value", Field: rules.FieldAmountMinor, Operator: rules.OpGTE, Value: "500000", Severity: "high", Enabled: true},
		{ID: "r2", Name: "gbp", Field: rules.FieldCurrency, Operator: rules.OpEq, Value: "GBP", Severity: "low", Enabled: true},
	}}
	repo := newFakeAlertRepo()
	svc := alerts.NewService(repo, rulesRepo, slog.Default())

	// Amount matches r1 and r2; currency matches r2 (GBP). Two unique matches.
	txn := transactions.Transaction{
		ID:          "txn-1",
		AmountMinor: 600000,
		Currency:    "GBP",
		Direction:   transactions.DirectionDebit,
	}
	if err := svc.TransactionIngested(context.Background(), txn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.raised) != 2 {
		t.Errorf("want 2 alerts raised, got %d", len(repo.raised))
	}
}

func TestService_TransactionIngested_NoMatchesNoAlerts(t *testing.T) {
	rulesRepo := &fakeRulesRepo{rules: []rules.Rule{
		{ID: "r1", Name: "gbp", Field: rules.FieldCurrency, Operator: rules.OpEq, Value: "GBP", Severity: "low", Enabled: true},
	}}
	repo := newFakeAlertRepo()
	svc := alerts.NewService(repo, rulesRepo, slog.Default())

	txn := transactions.Transaction{ID: "txn-2", AmountMinor: 100, Currency: "USD", Direction: transactions.DirectionDebit}
	if err := svc.TransactionIngested(context.Background(), txn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.raised) != 0 {
		t.Errorf("want 0 alerts raised, got %d", len(repo.raised))
	}
}
