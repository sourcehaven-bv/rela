package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func setupCreateTestEnv() {
	g = graph.New()
	meta = nil // Will be set by individual tests
	ws = nil   // Will be set by individual tests after meta is set
	out = output.New(output.FormatTable)
	projectCtx = &project.Context{
		Root:          "/tmp/test-project",
		EntitiesDir:   "/tmp/test-project/entities",
		RelationsDir:  "/tmp/test-project/relations",
		CachePath:     "/tmp/test-project/.rela/cache.json",
		MetamodelPath: "/tmp/test-project/metamodel.yaml",
	}
}

func TestParsePropertyFlag(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "simple key=value",
			input:     "title=Hello World",
			wantKey:   "title",
			wantValue: "Hello World",
			wantErr:   false,
		},
		{
			name:      "key with spaces around equals",
			input:     "title = Hello World",
			wantKey:   "title",
			wantValue: "Hello World",
			wantErr:   false,
		},
		{
			name:      "value with equals sign",
			input:     "formula=a=b+c",
			wantKey:   "formula",
			wantValue: "a=b+c",
			wantErr:   false,
		},
		{
			name:      "empty value",
			input:     "description=",
			wantKey:   "description",
			wantValue: "",
			wantErr:   false,
		},
		{
			name:    "missing equals sign",
			input:   "title",
			wantErr: true,
		},
		{
			name:    "empty key",
			input:   "=value",
			wantErr: true,
		},
		{
			name:    "only equals sign",
			input:   "=",
			wantErr: true,
		},
		{
			name:      "key with underscore",
			input:     "iso27001=A.5.15",
			wantKey:   "iso27001",
			wantValue: "A.5.15",
			wantErr:   false,
		},
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

func TestCreateEntityWithPrimaryPropertyName(t *testing.T) {
	setupCreateTestEnv()

	// Create a metamodel with stakeholder type that uses "name" as primary property
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"stakeholder": {
				Label:    "Stakeholder",
				IDPrefix: "SH-",
				Properties: map[string]metamodel.PropertyDef{
					"name":   {Type: "string", Required: true},
					"role":   {Type: "string"},
					"status": {Type: "status", Required: true},
				},
			},
			"requirement": {
				Label:    "Requirement",
				Aliases:  []string{"req"},
				IDPrefix: "REQ-",
				Properties: map[string]metamodel.PropertyDef{
					"title":       {Type: "string", Required: true},
					"description": {Type: "string"},
					"status":      {Type: "status", Required: true},
				},
			},
		},
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"draft", "proposed", "accepted"},
				Default: "draft",
			},
		},
	}
	ws = workspace.NewForTest(g, meta)

	// Test that GetPrimaryProperty returns correct property for each type
	stakeholderDef, _ := meta.GetEntityDef("stakeholder")
	if primary := stakeholderDef.GetPrimaryProperty(); primary != "name" {
		t.Errorf("stakeholder primary property = %q, want %q", primary, "name")
	}

	reqDef, _ := meta.GetEntityDef("requirement")
	if primary := reqDef.GetPrimaryProperty(); primary != "title" {
		t.Errorf("requirement primary property = %q, want %q", primary, "title")
	}
}

func TestCreateEntityWithMultipleProperties(t *testing.T) {
	setupCreateTestEnv()

	// Create a metamodel with control type that has multiple properties
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"control": {
				Label:    "Control",
				IDPrefix: "CTRL-",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"iso27001": {Type: "string"},
					"owner":    {Type: "string"},
					"status":   {Type: "status", Required: true},
				},
			},
		},
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"draft", "implemented", "verified"},
				Default: "draft",
			},
		},
	}
	ws = workspace.NewForTest(g, meta)

	// Test that GetPrimaryProperty returns "title" for control
	controlDef, _ := meta.GetEntityDef("control")
	if primary := controlDef.GetPrimaryProperty(); primary != "title" {
		t.Errorf("control primary property = %q, want %q", primary, "title")
	}
}

func TestCreateEntityTypeWithLabelProperty(t *testing.T) {
	setupCreateTestEnv()

	// Create a metamodel where "label" is the primary property
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"tag": {
				Label:    "Tag",
				IDPrefix: "TAG-",
				Properties: map[string]metamodel.PropertyDef{
					"label":       {Type: "string", Required: true},
					"description": {Type: "string"},
				},
			},
		},
	}
	ws = workspace.NewForTest(g, meta)

	// Test that GetPrimaryProperty returns "label" for tag
	tagDef, _ := meta.GetEntityDef("tag")
	if primary := tagDef.GetPrimaryProperty(); primary != "label" {
		t.Errorf("tag primary property = %q, want %q", primary, "label")
	}
}

func TestCreateEntityNoPrimaryProperty(t *testing.T) {
	setupCreateTestEnv()

	// Create a metamodel where there's no required string property
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"marker": {
				Label:    "Marker",
				IDPrefix: "MRK-",
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: "status", Required: true},
				},
			},
		},
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"active", "inactive"},
				Default: "active",
			},
		},
	}
	ws = workspace.NewForTest(g, meta)

	// Test that GetPrimaryProperty returns empty string
	markerDef, _ := meta.GetEntityDef("marker")
	if primary := markerDef.GetPrimaryProperty(); primary != "" {
		t.Errorf("marker primary property = %q, want empty string", primary)
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
