package userstate

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBase(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		userCfg     string
		userCfgErr  error
		want        string
		wantErrPart string
	}{
		{
			name:    "default path uses UserConfigDir",
			env:     map[string]string{},
			userCfg: "/home/u/.config",
			want:    "/home/u/.config",
		},
		{
			name:    "override takes precedence",
			env:     map[string]string{EnvOverride: "/opt/rela-state"},
			userCfg: "/home/u/.config",
			want:    "/opt/rela-state",
		},
		{
			name:    "override trims whitespace",
			env:     map[string]string{EnvOverride: "  /opt/rela  "},
			userCfg: "/home/u/.config",
			want:    "/opt/rela",
		},
		{
			name:        "relative override rejected",
			env:         map[string]string{EnvOverride: "relative/path"},
			userCfg:     "/home/u/.config",
			wantErrPart: "must be an absolute path",
		},
		{
			name:        "empty override falls through",
			env:         map[string]string{EnvOverride: "   "},
			userCfg:     "/home/u/.config",
			want:        "/home/u/.config",
			wantErrPart: "",
		},
		{
			name:        "userConfigDir failure surfaces",
			env:         map[string]string{},
			userCfgErr:  errors.New("no HOME"),
			wantErrPart: "no user config dir available",
		},
		{
			name:        "control char in override rejected",
			env:         map[string]string{EnvOverride: "/opt/bad\x00dir"},
			userCfg:     "/home/u/.config",
			wantErrPart: "control or NUL",
		},
		{
			name:    "override cleans path",
			env:     map[string]string{EnvOverride: "/opt/rela/../rela/state"},
			userCfg: "/home/u/.config",
			want:    "/opt/rela/state",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envFn := func(k string) string { return tc.env[k] }
			userCfgFn := func() (string, error) {
				if tc.userCfgErr != nil {
					return "", tc.userCfgErr
				}
				return tc.userCfg, nil
			}
			got, err := resolveBase(envFn, userCfgFn)
			if tc.wantErrPart != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil (result=%q)", tc.wantErrPart, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Fatalf("want error containing %q, got %q", tc.wantErrPart, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsInside(t *testing.T) {
	// Build platform-correct absolute paths so the test works on
	// Windows (where "/repo" is not absolute) as well as Unix.
	root := filepath.Join(t.TempDir(), "repo")
	other := filepath.Join(t.TempDir(), "other")

	tests := []struct {
		name      string
		candidate string
		boundary  string
		want      bool
	}{
		{"child", filepath.Join(root, ".rela"), root, true},
		{"deep child", filepath.Join(root, ".rela", "sub", "file"), root, true},
		{"equal", root, root, true},
		{"sibling", other, root, false},
		{"relative candidate", ".rela", root, false},
		{"relative boundary", filepath.Join(root, ".rela"), "repo", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isInside(tc.candidate, tc.boundary)
			if got != tc.want {
				t.Errorf("isInside(%q, %q) = %v, want %v",
					tc.candidate, tc.boundary, got, tc.want)
			}
		})
	}
}

func TestDetectSyncDir(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"dropbox linux", "/home/u/Dropbox/rela", "/Dropbox/"},
		{"icloud mac", "/Users/u/Library/Mobile Documents/com~apple~CloudDocs/rela", "/Library/Mobile Documents"},
		{"onedrive", "/Users/u/OneDrive/rela", "/OneDrive"},
		{"clean path", "/home/u/.config/rela", ""},
		{"clean mac", "/Users/u/Library/Application Support/rela", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectSyncDir(tc.path); got != tc.want {
				t.Errorf("detectSyncDir(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
