package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if !cfg.App.AllowRegistration {
		t.Fatal("expected registration to be enabled by default")
	}
	if cfg.App.DataDir != "./data" {
		t.Fatalf("expected default data dir ./data, got %q", cfg.App.DataDir)
	}
}

func TestLoadReadsRegistrationSwitchFromConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("app:\n  allowRegistration: false\n  jwtSecret: test-secret\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.App.AllowRegistration {
		t.Fatal("expected registration to be disabled from config")
	}
	if cfg.App.JWTSecret != "test-secret" {
		t.Fatalf("expected jwt secret from config, got %q", cfg.App.JWTSecret)
	}
}
