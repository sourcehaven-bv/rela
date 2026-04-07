package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProvider_Missing(t *testing.T) {
	dir := t.TempDir()
	if p := LoadProvider(dir); p != nil {
		t.Errorf("expected nil provider when config missing, got %T", p)
	}
}

func TestLoadProvider_Valid(t *testing.T) {
	dir := t.TempDir()
	writeAIYaml(t, dir, `
base_url: https://example.com/v1
model: test-model
`)
	p := LoadProvider(dir)
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestLoadProvider_Malformed(t *testing.T) {
	dir := t.TempDir()
	writeAIYaml(t, dir, `not: valid: yaml: at: all`)
	if p := LoadProvider(dir); p != nil {
		t.Errorf("expected nil provider on malformed config, got %T", p)
	}
}

func TestLoadProvider_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	writeAIYaml(t, dir, `model: only-model-no-url`)
	if p := LoadProvider(dir); p != nil {
		t.Errorf("expected nil provider on invalid config, got %T", p)
	}
}

func writeAIYaml(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
