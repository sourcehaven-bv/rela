package userstate

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateRepoID(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		valid bool
	}{
		{"canonical 32-hex", "0123456789abcdef0123456789abcdef", true},
		{"upper case rejected", "0123456789ABCDEF0123456789ABCDEF", false},
		{"with dashes rejected", "01234567-89ab-cdef-0123-456789abcdef", false},
		{"too short", "0123456789abcdef", false},
		{"too long", "0123456789abcdef0123456789abcdef00", false},
		{"empty", "", false},
		{"non-hex", "ghijklmnghijklmnghijklmnghijklmn", false},
		{"with whitespace", " 0123456789abcdef0123456789abcdef ", false},
		{"path-traversal attempt", "../../etc/passwd0123456789abcdef", false},
		{"slash in id", "/0123456789abcdef0123456789abcde", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRepoID(tc.in)
			if tc.valid && err != nil {
				t.Errorf("valid id %q rejected: %v", tc.in, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("invalid id %q accepted", tc.in)
			}
			if !tc.valid && err != nil && !errors.Is(err, ErrInvalidRepoID) {
				t.Errorf("wrong sentinel for %q: %v", tc.in, err)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"plain", "ui-state.json", ""},
		{"nested", "documents/abc.html", ""},
		{"empty", "", "must not be empty"},
		{"dotdot", "../escape", "traversal"},
		{"absolute", "/abs", "relative"},
		{"backslash", "a\\b", "backslash"},
		{"nul", "a\x00b", "control character"},
		{"drive letter", "C:file", "drive letter"},
		{"trailing slash", "foo/", "empty segment"},
		{"dot segment", "./foo", "traversal or empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateKey(tc.in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
