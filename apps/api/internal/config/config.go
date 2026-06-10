// Package config loads and validates runtime configuration from environment
// variables. Configuration is read once at startup; an invalid configuration
// is a fatal, fail-fast condition rather than something handled at request time.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the API service. All values are
// derived from environment variables and validated by Validate.
type Config struct {
	// AppEnv is the deployment environment: development, staging or production.
	AppEnv string
	// LogLevel controls the minimum slog level: debug, info, warn or error.
	LogLevel string

	// HTTPHost is the interface the HTTP server binds to.
	HTTPHost string
	// HTTPPort is the TCP port the HTTP server listens on.
	HTTPPort int

	// ReadHeaderTimeout bounds the time spent reading request headers.
	ReadHeaderTimeout time.Duration
	// ReadTimeout bounds the time spent reading the full request.
	ReadTimeout time.Duration
	// WriteTimeout bounds the time spent writing the response.
	WriteTimeout time.Duration
	// IdleTimeout bounds how long an idle keep-alive connection is kept open.
	IdleTimeout time.Duration
	// ShutdownTimeout bounds graceful shutdown before connections are forced closed.
	ShutdownTimeout time.Duration

	// DatabaseURL is the PostgreSQL connection string (pgx/libpq format).
	DatabaseURL string

	// CORSAllowedOrigins is the list of browser origins permitted to call the API.
	CORSAllowedOrigins []string
}

// Addr returns the host:port the HTTP server should bind to.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.HTTPHost, c.HTTPPort)
}

// validEnvs / validLevels are the closed sets accepted by Validate.
var (
	validEnvs   = map[string]bool{"development": true, "staging": true, "production": true}
	validLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
)

// Load reads configuration using the provided getenv function (typically
// os.Getenv) and validates it. Passing getenv explicitly keeps Load pure and
// testable without mutating process environment.
func Load(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		AppEnv:   firstNonEmpty(getenv("APP_ENV"), "development"),
		LogLevel: firstNonEmpty(getenv("LOG_LEVEL"), "info"),
		HTTPHost: firstNonEmpty(getenv("HTTP_HOST"), "0.0.0.0"),

		DatabaseURL: getenv("DATABASE_URL"),
	}

	var err error
	if cfg.HTTPPort, err = intEnv(getenv, "HTTP_PORT", 8080); err != nil {
		return nil, err
	}
	if cfg.ReadHeaderTimeout, err = durationEnv(getenv, "HTTP_READ_HEADER_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ReadTimeout, err = durationEnv(getenv, "HTTP_READ_TIMEOUT", 15*time.Second); err != nil {
		return nil, err
	}
	if cfg.WriteTimeout, err = durationEnv(getenv, "HTTP_WRITE_TIMEOUT", 15*time.Second); err != nil {
		return nil, err
	}
	if cfg.IdleTimeout, err = durationEnv(getenv, "HTTP_IDLE_TIMEOUT", 60*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = durationEnv(getenv, "HTTP_SHUTDOWN_TIMEOUT", 15*time.Second); err != nil {
		return nil, err
	}

	cfg.CORSAllowedOrigins = splitAndTrim(firstNonEmpty(getenv("CORS_ALLOWED_ORIGINS"), "http://localhost:5173"))

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate enforces invariants that must hold for the service to start.
func (c Config) Validate() error {
	if !validEnvs[c.AppEnv] {
		return fmt.Errorf("config: APP_ENV %q must be one of development, staging, production", c.AppEnv)
	}
	if !validLevels[c.LogLevel] {
		return fmt.Errorf("config: LOG_LEVEL %q must be one of debug, info, warn, error", c.LogLevel)
	}
	if c.HTTPHost == "" {
		return fmt.Errorf("config: HTTP_HOST must not be empty")
	}
	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		return fmt.Errorf("config: HTTP_PORT %d must be between 1 and 65535", c.HTTPPort)
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL is required")
	}
	if !strings.HasPrefix(c.DatabaseURL, "postgres://") && !strings.HasPrefix(c.DatabaseURL, "postgresql://") {
		return fmt.Errorf("config: DATABASE_URL must be a postgres:// or postgresql:// connection string")
	}
	for name, d := range map[string]time.Duration{
		"HTTP_READ_HEADER_TIMEOUT": c.ReadHeaderTimeout,
		"HTTP_READ_TIMEOUT":        c.ReadTimeout,
		"HTTP_WRITE_TIMEOUT":       c.WriteTimeout,
		"HTTP_IDLE_TIMEOUT":        c.IdleTimeout,
		"HTTP_SHUTDOWN_TIMEOUT":    c.ShutdownTimeout,
	} {
		if d <= 0 {
			return fmt.Errorf("config: %s must be a positive duration", name)
		}
	}
	return nil
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func intEnv(getenv func(string) string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s %q must be an integer: %w", key, raw, err)
	}
	return n, nil
}

func durationEnv(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s %q must be a Go duration (e.g. 15s): %w", key, raw, err)
	}
	return d, nil
}

func splitAndTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
