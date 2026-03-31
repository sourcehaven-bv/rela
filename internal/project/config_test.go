package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Formatting.LineWidth != 80 {
		t.Errorf("DefaultConfig().Formatting.LineWidth = %d, want 80", cfg.Formatting.LineWidth)
	}
}

func TestLoadConfig_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() error = %v", err)
	}

	if cfg.Formatting.LineWidth != 80 {
		t.Errorf("LineWidth = %d, want 80 (default)", cfg.Formatting.LineWidth)
	}
}

func TestLoadConfig_CustomLineWidth(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	content := `formatting:
  line_width: 100
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() error = %v", err)
	}

	if cfg.Formatting.LineWidth != 100 {
		t.Errorf("LineWidth = %d, want 100", cfg.Formatting.LineWidth)
	}
}

func TestLoadConfig_ZeroLineWidthUsesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	content := `formatting:
  line_width: 0
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() error = %v", err)
	}

	if cfg.Formatting.LineWidth != 80 {
		t.Errorf("LineWidth = %d, want 80 (default for zero)", cfg.Formatting.LineWidth)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	content := `formatting: [invalid`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := LoadConfigFromPath(path)
	if err == nil {
		t.Error("LoadConfigFromPath() should return error for invalid YAML")
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() error = %v", err)
	}

	// Empty file should use defaults
	if cfg.Formatting.LineWidth != 80 {
		t.Errorf("LineWidth = %d, want 80 (default)", cfg.Formatting.LineWidth)
	}
}

func TestContext_ConfigPath(t *testing.T) {
	ctx := newContext("/project")
	expected := "/project/.rela/config.yaml"

	if ctx.ConfigPath() != expected {
		t.Errorf("ConfigPath() = %q, want %q", ctx.ConfigPath(), expected)
	}
}
