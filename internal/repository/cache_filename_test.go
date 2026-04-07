package repository

import (
	"strings"
	"testing"
)

func TestValidateCacheFilename_Accepts(t *testing.T) {
	ok := []string{
		"cache.json",
		"user-defaults.yaml",
		"ui-state.json",
		"palette.yaml",
		"documents/render-abc123.html", // legitimate nested cache
	}
	for _, name := range ok {
		t.Run(name, func(t *testing.T) {
			if err := validateCacheFilename(name); err != nil {
				t.Fatalf("expected ok for %q, got %v", name, err)
			}
		})
	}
}

func TestValidateCacheFilename_Rejects(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"", "empty"},
		{"..", "traversal"},
		{".", "traversal"},
		{"/etc/passwd", "relative"},
		{"a\\b.yaml", "backslash"},
		{"../escape.yaml", "traversal"},
		{"sub/../escape.yaml", "traversal"},
		{"with\x00nul.yaml", "control character"},
		{"with\x01ctrl.yaml", "control character"},
		{"a//b.yaml", "empty segment"},
		{"c:file.yaml", "drive letter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCacheFilename(tc.name)
			if err == nil {
				t.Fatalf("expected error for %q", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q should mention %q", err.Error(), tc.want)
			}
		})
	}
}
