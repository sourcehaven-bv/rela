package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_Missing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("expected ErrConfigNotFound, got err=%v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for missing file, got %+v", cfg)
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
provider: openai-compatible
base_url: https://api.openai.com/v1
model: gpt-4o-mini
api_key_env: OPENAI_API_KEY
timeout_seconds: 60
`)
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("APIKeyEnv = %q", cfg.APIKeyEnv)
	}
	if cfg.Timeout() != 60 {
		t.Errorf("Timeout() = %d", cfg.Timeout())
	}
}

func TestLoadConfig_NoAPIKeyEnv(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
base_url: http://localhost:11434/v1
model: gemma3:12b
`)
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.APIKeyEnv != "" {
		t.Errorf("expected empty APIKeyEnv, got %q", cfg.APIKeyEnv)
	}
	if cfg.Timeout() != DefaultTimeoutSeconds {
		t.Errorf("expected default timeout, got %d", cfg.Timeout())
	}
}

func TestLoadConfig_Malformed(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `not: valid: yaml: at: all`)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestLoadConfig_MissingBaseURL(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `model: gpt-4`)
	_, err := LoadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "base_url") {
		t.Fatalf("expected base_url error, got %v", err)
	}
}

func TestLoadConfig_MissingModel(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `base_url: https://example.com/v1`)
	_, err := LoadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("expected model error, got %v", err)
	}
}

func TestLoadConfig_BaseURLNoScheme(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
base_url: api.openai.com/v1
model: gpt-4
`)
	_, err := LoadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "http") {
		t.Fatalf("expected scheme error, got %v", err)
	}
}

func TestLoadConfig_BaseURLWithCredentials(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
base_url: https://user:secret@api.example.com/v1
model: gpt-4
`)
	_, err := LoadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("expected credentials error, got %v", err)
	}
}

func TestLoadConfig_UnsupportedProvider(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
provider: anthropic-native
base_url: https://api.anthropic.com/v1
model: claude
`)
	_, err := LoadConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "openai-compatible") {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestLoadConfig_NegativeTimeout(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
base_url: https://example.com/v1
model: gpt-4
timeout_seconds: -1
`)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
}

func TestLoadConfig_DirectoryDoesNotExist(t *testing.T) {
	cfg, err := LoadConfig("/definitely/does/not/exist/anywhere")
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("expected ErrConfigNotFound for missing dir, got %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil cfg, got %+v", cfg)
	}
}

func TestConfig_Timeout_Default(t *testing.T) {
	c := &Config{}
	if c.Timeout() != DefaultTimeoutSeconds {
		t.Errorf("Timeout() = %d, want default %d", c.Timeout(), DefaultTimeoutSeconds)
	}
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ConfigFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Sanity check that LoadConfig returns nil when the directory exists
// but the file is permission-denied. Skipped on Windows / when running
// as root since chmod 000 doesn't reliably block root.
func TestLoadConfig_ReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read 000 files")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFile)
	if err := os.WriteFile(path, []byte("base_url: https://x/y\nmodel: m\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	// Should be a wrapped read error, not os.ErrNotExist.
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("unexpectedly got ErrNotExist: %v", err)
	}
}
