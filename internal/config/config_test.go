package config

import (
	"os"
	"testing"
)

// unsetEnv removes the given variables for the duration of the test,
// restoring their prior values via t.Cleanup.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		orig, had := os.LookupEnv(k)
		_ = os.Unsetenv(k)
		key := k
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(key, orig)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	unsetEnv(t, "PORT", "LOG_LEVEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoad_RejectsInvalidLogLevel(t *testing.T) {
	unsetEnv(t, "PORT")
	t.Setenv("LOG_LEVEL", "verbose")

	if _, err := Load(); err == nil {
		t.Fatal("expected validation error for LOG_LEVEL=verbose, got nil")
	}
}

func TestLoad_RejectsOutOfRangePort(t *testing.T) {
	unsetEnv(t, "LOG_LEVEL")
	t.Setenv("PORT", "70000")

	if _, err := Load(); err == nil {
		t.Fatal("expected validation error for PORT=70000, got nil")
	}
}
