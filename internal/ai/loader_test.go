package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProvider_Missing(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadProvider(dir)
	if !errors.Is(err, ErrConfigNotFound) {
		t.Errorf("expected ErrConfigNotFound, got %v", err)
	}
	if p != nil {
		t.Errorf("expected nil provider when config missing, got %T", p)
	}
}

func TestLoadProvider_Valid(t *testing.T) {
	dir := t.TempDir()
	writeAIYaml(t, dir, `
base_url: https://example.com/v1
model: test-model
`)
	p, err := LoadProvider(dir)
	if err != nil {
		t.Fatalf("LoadProvider: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestLoadProvider_Malformed(t *testing.T) {
	dir := t.TempDir()
	writeAIYaml(t, dir, `not: valid: yaml: at: all`)
	p, err := LoadProvider(dir)
	if err == nil {
		t.Fatal("expected error on malformed config")
	}
	if p != nil {
		t.Errorf("expected nil provider on malformed config, got %T", p)
	}
}

func TestLoadProvider_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	writeAIYaml(t, dir, `model: only-model-no-url`)
	p, err := LoadProvider(dir)
	if err == nil {
		t.Fatal("expected error on invalid config")
	}
	if !strings.Contains(err.Error(), "base_url") {
		t.Errorf("expected base_url error, got %v", err)
	}
	if p != nil {
		t.Errorf("expected nil provider on invalid config, got %T", p)
	}
}

func writeAIYaml(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
