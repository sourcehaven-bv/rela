package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "test.lua")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLoad_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
api_key: sk-abc123
base_url: https://example.com
`)

	sec, err := Load(dir, "any-script.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["api_key"] != "sk-abc123" {
		t.Errorf("expected sk-abc123, got %q", sec["api_key"])
	}
	if sec["base_url"] != "https://example.com" {
		t.Errorf("expected https://example.com, got %q", sec["base_url"])
	}
}

func TestLoad_OverrideMerge(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
api_key: global-key
shared: shared-value
overrides:
  special.lua:
    api_key: special-key
    extra: extra-value
`)

	// Script with override
	sec, err := Load(dir, "special.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["api_key"] != "special-key" {
		t.Errorf("expected override key, got %q", sec["api_key"])
	}
	if sec["shared"] != "shared-value" {
		t.Errorf("expected global shared value, got %q", sec["shared"])
	}
	if sec["extra"] != "extra-value" {
		t.Errorf("expected override extra, got %q", sec["extra"])
	}

	// Script without override gets globals only
	sec2, err := Load(dir, "other.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec2["api_key"] != "global-key" {
		t.Errorf("expected global key, got %q", sec2["api_key"])
	}
	if _, ok := sec2["extra"]; ok {
		t.Error("other.lua should not have extra key")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "")

	sec, err := Load(dir, "test.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sec) != 0 {
		t.Errorf("expected empty map, got %v", sec)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "{{invalid")

	_, err := Load(dir, "test.lua")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_OverridesOnly(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `
overrides:
  my-script.lua:
    token: secret
`)

	sec, err := Load(dir, "my-script.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec["token"] != "secret" {
		t.Errorf("expected secret, got %q", sec["token"])
	}

	// Script not in overrides gets empty map
	sec2, err := Load(dir, "other.lua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sec2) != 0 {
		t.Errorf("expected empty map, got %v", sec2)
	}
}

func writeYAML(t *testing.T, dir, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(content), 0600)
	if err != nil {
		t.Fatalf("write secrets.yaml: %v", err)
	}
}
