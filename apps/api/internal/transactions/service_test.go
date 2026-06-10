package transactions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeRepo struct {
	saved     []Transaction
	lastQuery PageQuery
	page      Page
	saveErr   error
}

func (f *fakeRepo) Save(_ context.Context, t Transaction) (Transaction, error) {
	if f.saveErr != nil {
		return Transaction{}, f.saveErr
	}
	t.ID = "generated-id"
	t.CreatedAt = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	f.saved = append(f.saved, t)
	return t, nil
}

func (f *fakeRepo) List(_ context.Context, q PageQuery) (Page, error) {
	f.lastQuery = q
	return f.page, nil
}

func quietLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func TestService_Ingest(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, NewGenerator(1), quietLogger())

	n, err := svc.Ingest(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 || len(repo.saved) != 5 {
		t.Fatalf("persisted = %d / saved = %d, want 5 / 5", n, len(repo.saved))
	}
}

func TestService_Ingest_StopsOnError(t *testing.T) {
	repo := &fakeRepo{saveErr: errors.New("db down")}
	svc := NewService(repo, NewGenerator(1), quietLogger())

	n, err := svc.Ingest(context.Background(), 3)
	if err == nil {
		t.Fatal("expected error")
	}
	if n != 0 {
		t.Fatalf("persisted = %d, want 0", n)
	}
}

func TestService_List_ClampsLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero defaults", 0, DefaultPageLimit},
		{"negative defaults", -10, DefaultPageLimit},
		{"over max clamps", 9999, MaxPageLimit},
		{"within range kept", 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := NewService(repo, NewGenerator(1), quietLogger())
			if _, err := svc.List(context.Background(), PageQuery{Limit: tt.in}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.lastQuery.Limit != tt.want {
				t.Errorf("limit passed to repo = %d, want %d", repo.lastQuery.Limit, tt.want)
			}
		})
	}
}
