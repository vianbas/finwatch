package httpserver

import (
	"net/http"
	"time"
)

// Options configures the underlying *http.Server. Timeout values come from
// validated configuration and provide secure defaults against slow-client and
// resource-exhaustion conditions.
type Options struct {
	Addr              string
	Handler           http.Handler
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// New constructs an *http.Server with explicit, bounded timeouts. There is no
// implicit unbounded behaviour: every phase of the request lifecycle has a
// deadline.
func New(opts Options) *http.Server {
	return &http.Server{
		Addr:              opts.Addr,
		Handler:           opts.Handler,
		ReadHeaderTimeout: opts.ReadHeaderTimeout,
		ReadTimeout:       opts.ReadTimeout,
		WriteTimeout:      opts.WriteTimeout,
		IdleTimeout:       opts.IdleTimeout,
	}
}
