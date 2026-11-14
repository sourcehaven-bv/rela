package project

import (
	"os"
	"path/filepath"
	"slices"
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

func TestLoadConfig_RemoteMCP(t *testing.T) {
	t.Parallel()

	on, off := true, false
	tests := []struct {
		name      string
		yaml      string
		want      *bool
		wantAllow []string
		wantDeny  []string
		wantErr   bool
	}{
		{name: "absent", yaml: "validation: strict\n"},
		{name: "empty block", yaml: "remote_mcp:\n"},
		{name: "enabled true", yaml: "remote_mcp:\n  enabled: true\n", want: &on},
		{name: "enabled false", yaml: "remote_mcp:\n  enabled: false\n", want: &off},
		{
			name:      "tool lists",
			yaml:      "remote_mcp:\n  tools:\n    allow: [show_entity]\n    deny: [attach_file, run_action]\n",
			wantAllow: []string{"show_entity"}, wantDeny: []string{"attach_file", "run_action"},
		},
		{name: "misspelled key is refused", yaml: "remote_mcp:\n  enable: false\n", wantErr: true},
		{name: "misspelled tools key is refused", yaml: "remote_mcp:\n  tools:\n    denied: [x]\n", wantErr: true},
		{name: "non-boolean enabled is refused", yaml: "remote_mcp:\n  enabled: maybe\n", wantErr: true},
		{name: "misspelled top-level key is refused", yaml: "remote-mcp:\n  enabled: false\n", wantErr: true},
		{name: "camel-case top-level key is refused", yaml: "remoteMCP:\n  enabled: false\n", wantErr: true},
		{name: "empty allow is refused", yaml: "remote_mcp:\n  tools:\n    allow: []\n", wantErr: true},
		{name: "merge key is refused", yaml: "x: &x {enabled: false}\nremote_mcp:\n  <<: *x\n", wantErr: true},
		{
			name:     "alias to a list elsewhere in the file",
			yaml:     "readonly: &ro [attach_file]\nremote_mcp:\n  tools:\n    deny: *ro\n",
			wantDeny: []string{"attach_file"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfigFromPath(path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("LoadConfigFromPath = %+v, want an error", cfg.RemoteMCP)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfigFromPath: %v", err)
			}
			got := cfg.RemoteMCP
			if (got.Enabled == nil) != (tc.want == nil) || (got.Enabled != nil && *got.Enabled != *tc.want) {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tc.want)
			}
			if !slices.Equal(got.Tools.Allow, tc.wantAllow) || !slices.Equal(got.Tools.Deny, tc.wantDeny) {
				t.Errorf("Tools = %+v, want allow %v deny %v", got.Tools, tc.wantAllow, tc.wantDeny)
			}
		})
	}
}
