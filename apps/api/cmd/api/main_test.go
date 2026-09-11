package main

import (
	"strings"
	"testing"
)

func TestDemoPassword(t *testing.T) {
	tests := []struct {
		name     string
		getenv   func(string) string
		appEnv   string
		key      string
		fallback string
		want     string
		wantErr  bool
	}{
		{
			name:     "env set returns env value",
			getenv:   func(string) string { return "from-env-value" },
			appEnv:   "production",
			key:      "DEMO_OPERATOR_PASSWORD",
			fallback: "operator_dev_password",
			want:     "from-env-value",
		},
		{
			name:     "unset in development returns fallback",
			getenv:   func(string) string { return "" },
			appEnv:   "development",
			key:      "DEMO_OPERATOR_PASSWORD",
			fallback: "operator_dev_password",
			want:     "operator_dev_password",
		},
		{
			name:     "unset in staging returns error",
			getenv:   func(string) string { return "" },
			appEnv:   "staging",
			key:      "DEMO_OPERATOR_PASSWORD",
			fallback: "operator_dev_password",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := demoPassword(tt.getenv, tt.appEnv, tt.key, tt.fallback)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if strings.Contains(err.Error(), tt.fallback) {
					t.Errorf("error message %q must not contain the fallback password", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("demoPassword() = %q, want %q", got, tt.want)
			}
		})
	}
}
