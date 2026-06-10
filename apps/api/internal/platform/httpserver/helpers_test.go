package httpserver

import (
	"io"
	"log/slog"
)

// testLogger returns a logger that discards output, keeping test logs quiet.
func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
