package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// create_test.go only covers CLI-specific plumbing. Metamodel
// introspection tests (GetPrimaryProperty etc.) live with the
// metamodel package, and entity CRUD is covered by storetest.

func TestParsePropertyFlag(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{name: "simple key=value", input: "title=Hello World", wantKey: "title", wantValue: "Hello World"},
		{name: "key with spaces around equals", input: "title = Hello World", wantKey: "title", wantValue: "Hello World"},
		{name: "value with equals sign", input: "formula=a=b+c", wantKey: "formula", wantValue: "a=b+c"},
		{name: "empty value", input: "description=", wantKey: "description", wantValue: ""},
		{name: "missing equals sign", input: "title", wantErr: true},
		{name: "empty key", input: "=value", wantErr: true},
		{name: "only equals sign", input: "=", wantErr: true},
		{name: "key with underscore", input: "iso27001=A.5.15", wantKey: "iso27001", wantValue: "A.5.15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, err := parsePropertyFlag(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parsePropertyFlag(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parsePropertyFlag(%q) unexpected error: %v", tt.input, err)
				return
			}
			if key != tt.wantKey {
				t.Errorf("parsePropertyFlag(%q) key = %q, want %q", tt.input, key, tt.wantKey)
			}
			if value != tt.wantValue {
				t.Errorf("parsePropertyFlag(%q) value = %q, want %q", tt.input, value, tt.wantValue)
			}
		})
	}
}

func TestGetBodyContent(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		bodyFile    string
		fileContent string
		stdinData   string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "no body flags",
			want: "",
		},
		{
			name: "body flag with content",
			body: "## Description\n\nSome content here.",
			want: "## Description\n\nSome content here.",
		},
		{
			name:        "body-file flag with file",
			bodyFile:    "content.md",
			fileContent: "Content from file.\n",
			want:        "Content from file.",
		},
		{
			name:      "body-file flag with stdin",
			bodyFile:  "-",
			stdinData: "Content from stdin.\n",
			want:      "Content from stdin.",
		},
		{
			name:        "both flags specified",
			body:        "inline content",
			bodyFile:    "file.md",
			wantErr:     true,
			errContains: "cannot specify both",
		},
		{
			name:        "body-file with non-existent file",
			bodyFile:    "nonexistent.md",
			wantErr:     true,
			errContains: "failed to read body file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global flags
			createBody = tt.body
			createBodyFile = ""

			cmd := &cobra.Command{}

			// Handle file-based test
			if tt.bodyFile != "" && tt.bodyFile != "-" && tt.fileContent != "" {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, tt.bodyFile)
				if err := os.WriteFile(filePath, []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}
				createBodyFile = filePath
			} else if tt.bodyFile != "" {
				createBodyFile = tt.bodyFile
			}

			// Handle stdin test
			if tt.stdinData != "" {
				cmd.SetIn(bytes.NewBufferString(tt.stdinData))
			}

			got, err := getBodyContent(cmd)

			if tt.wantErr {
				if err == nil {
					t.Errorf("getBodyContent() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("getBodyContent() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("getBodyContent() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("getBodyContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
