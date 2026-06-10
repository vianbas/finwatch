package config

import (
	"testing"
	"time"
)

// envFunc builds a getenv function backed by a map for hermetic tests.
func envFunc(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":      "development",
		"LOG_LEVEL":    "info",
		"DATABASE_URL": "postgres://user:pass@localhost:5432/finwatch?sslmode=disable",
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	cfg, err := Load(envFunc(validEnv()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.Addr() != "0.0.0.0:8080" {
		t.Errorf("Addr() = %q, want 0.0.0.0:8080", cfg.Addr())
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSAllowedOrigins = %v, want [http://localhost:5173]", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_OverridesParsed(t *testing.T) {
	env := validEnv()
	env["HTTP_PORT"] = "9090"
	env["HTTP_SHUTDOWN_TIMEOUT"] = "5s"
	env["CORS_ALLOWED_ORIGINS"] = "http://a.example, http://b.example"
	cfg, err := Load(envFunc(env))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, want 9090", cfg.HTTPPort)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 5s", cfg.ShutdownTimeout)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Errorf("CORSAllowedOrigins = %v, want 2 entries", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"missing database url", func(m map[string]string) { delete(m, "DATABASE_URL") }},
		{"non-postgres database url", func(m map[string]string) { m["DATABASE_URL"] = "mysql://localhost/db" }},
		{"invalid app env", func(m map[string]string) { m["APP_ENV"] = "prod" }},
		{"invalid log level", func(m map[string]string) { m["LOG_LEVEL"] = "trace" }},
		{"port out of range", func(m map[string]string) { m["HTTP_PORT"] = "70000" }},
		{"non-integer port", func(m map[string]string) { m["HTTP_PORT"] = "abc" }},
		{"non-duration timeout", func(m map[string]string) { m["HTTP_READ_TIMEOUT"] = "soon" }},
		{"zero timeout", func(m map[string]string) { m["HTTP_IDLE_TIMEOUT"] = "0s" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			tt.mutate(env)
			if _, err := Load(envFunc(env)); err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
		})
	}
}
