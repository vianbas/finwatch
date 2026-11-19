package httpapi

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestConnLimiter_EnforcesPerUserMax(t *testing.T) {
	l := newConnLimiter(2)
	if !l.acquire("a") || !l.acquire("a") {
		t.Fatal("first two acquires for a should succeed")
	}
	if l.acquire("a") {
		t.Fatal("third acquire for a should fail")
	}
	if !l.acquire("b") {
		t.Fatal("b has its own budget")
	}

	l.release("a")
	if !l.acquire("a") {
		t.Fatal("acquire after release should succeed")
	}

	l.release("a")
	l.release("a")
	l.release("b")
	if len(l.active) != 0 {
		t.Errorf("active = %v, want empty after releasing everything", l.active)
	}
}

func TestConnLimiter_ConcurrentAcquire(t *testing.T) {
	const max, callers = 5, 100
	l := newConnLimiter(max)

	var granted atomic.Int32
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.acquire("u") {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := granted.Load(); got != max {
		t.Fatalf("granted %d slots, want %d", got, max)
	}
	for range max {
		l.release("u")
	}
	if l.count("u") != 0 || len(l.active) != 0 {
		t.Errorf("count = %d, active = %v, want all released", l.count("u"), l.active)
	}
}
