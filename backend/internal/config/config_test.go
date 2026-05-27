package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", cfg.Server.ReadTimeout)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Level = %q, want info", cfg.Logging.Level)
	}
	if cfg.Data.Tier != "midnight-s1" {
		t.Errorf("Tier = %q, want midnight-s1", cfg.Data.Tier)
	}
	if cfg.Optimizer.Weights.Empirical != 8.0 {
		t.Errorf("Weights.Empirical = %v, want 8", cfg.Optimizer.Weights.Empirical)
	}
}

func TestLoadOverride(t *testing.T) {
	p := writeTemp(t, "server:\n  addr: \":9999\"\nlogging:\n  level: debug\noptimizer:\n  weights:\n    empirical: 2.5\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr != ":9999" {
		t.Errorf("Addr = %q, want :9999", cfg.Server.Addr)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Level = %q, want debug", cfg.Logging.Level)
	}
	if cfg.Optimizer.Weights.Empirical != 2.5 {
		t.Errorf("Empirical = %v, want 2.5", cfg.Optimizer.Weights.Empirical)
	}
	// Untouched fields keep their defaults.
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want default 30s", cfg.Server.WriteTimeout)
	}
}

func TestLoadInvalidEnum(t *testing.T) {
	if _, err := Load(writeTemp(t, "logging:\n  level: verbose\n")); err == nil {
		t.Fatal("expected error for invalid logging.level")
	}
}

func TestLoadUnknownField(t *testing.T) {
	if _, err := Load(writeTemp(t, "bogus: true\n")); err == nil {
		t.Fatal("expected error for unknown field (closed schema)")
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	if _, err := Load(writeTemp(t, "server:\n  read_timeout: \"30x\"\n")); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}
