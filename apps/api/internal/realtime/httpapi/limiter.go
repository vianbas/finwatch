package httpapi

import "sync"

// connLimiter caps concurrent streams per user. It is in-process, matching the
// single-instance deployment in ADR 0003.
type connLimiter struct {
	max int

	mu     sync.Mutex
	active map[string]int
}

func newConnLimiter(max int) *connLimiter {
	return &connLimiter{max: max, active: make(map[string]int)}
}

// acquire reserves a slot for user, reporting false when the user is already
// at the limit. Every successful acquire must be paired with one release.
func (l *connLimiter) acquire(user string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active[user] >= l.max {
		return false
	}
	l.active[user]++
	return true
}

// release frees a slot taken by acquire.
func (l *connLimiter) release(user string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active[user] <= 1 {
		delete(l.active, user)
		return
	}
	l.active[user]--
}

// count reports the user's active slots.
func (l *connLimiter) count(user string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active[user]
}
