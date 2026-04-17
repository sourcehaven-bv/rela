package cli

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func setupUpdateTestEnv() {
	g = graph.New()
	out = output.New(output.FormatTable)
	projectCtx = &project.Context{
		Root:          "/tmp/test-project",
		EntitiesDir:   "/tmp/test-project/entities",
		RelationsDir:  "/tmp/test-project/relations",
		MetamodelPath: "/tmp/test-project/metamodel.yaml",
	}
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"control": {
				Label:    "Control",
				IDPrefix: "CTRL-",
				Properties: map[string]metamodel.PropertyDef{
					"title":         {Type: "string", Required: true},
					"iso27001":      {Type: "string"},
					"owner":         {Type: "string"},
					"review_status": {Type: "string"},
					"status":        {Type: "status", Required: true},
				},
			},
			"requirement": {
				Label:      "Requirement",
				Aliases:    []string{"req"},
				IDPrefixes: []string{"REQ-", "RB-"},
				Properties: map[string]metamodel.PropertyDef{
					"title":         {Type: "string", Required: true},
					"description":   {Type: "string"},
					"review_status": {Type: "string"},
					"status":        {Type: "status", Required: true},
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
}

func TestUpdateCmd_PropertyFlagExists(t *testing.T) {
	// This test verifies that the -P/--property flag exists on the update command
	flag := updateCmd.Flags().Lookup("property")
	if flag == nil {
		t.Error("update command should have --property flag")
	}
	if flag != nil && flag.Shorthand != "P" {
		t.Errorf("--property flag shorthand = %q, want %q", flag.Shorthand, "P")
	}
}

func TestUpdateCmd_PropertyFlagParsing(t *testing.T) {
	setupUpdateTestEnv()

	// Create an existing entity in the graph
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("RB-001").
		With("title", "Original Title").
		With("status", "draft").
		Build())

	tests := []struct {
		name       string
		properties []string
		wantKey    string
		wantValue  string
		wantErr    bool
	}{
		{
			name:       "single property update",
			properties: []string{"review_status=current"},
			wantKey:    "review_status",
			wantValue:  "current",
			wantErr:    false,
		},
		{
			name:       "property with spaces in value",
			properties: []string{"owner=Security Team"},
			wantKey:    "owner",
			wantValue:  "Security Team",
			wantErr:    false,
		},
		{
			name:       "invalid property format",
			properties: []string{"invalid"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset updateProperties for each test
			updateProperties = tt.properties

			if tt.wantErr {
				// For error cases, we just verify the parsing fails
				for _, prop := range tt.properties {
					_, _, err := parsePropertyFlag(prop)
					if err == nil {
						t.Errorf("parsePropertyFlag(%q) expected error, got nil", prop)
					}
				}
				return
			}

			// For success cases, verify parsing works
			for _, prop := range tt.properties {
				key, value, err := parsePropertyFlag(prop)
				if err != nil {
					t.Errorf("parsePropertyFlag(%q) unexpected error: %v", prop, err)
					continue
				}
				if key != tt.wantKey {
					t.Errorf("parsePropertyFlag(%q) key = %q, want %q", prop, key, tt.wantKey)
				}
				if value != tt.wantValue {
					t.Errorf("parsePropertyFlag(%q) value = %q, want %q", prop, value, tt.wantValue)
				}
			}
		})
	}
}

func TestUpdateCmd_MultiplePropertiesApplied(t *testing.T) {
	setupUpdateTestEnv()

	// Create an existing entity in the graph
	entity := testutil.EntityFor(meta, "control").
		ID("CTRL-001").
		With("title", "Access Control").
		With("status", "draft").
		Build()
	g.AddNode(entity)

	// Simulate applying multiple properties
	properties := []string{"iso27001=A.5.15", "owner=Security Team"}

	for _, prop := range properties {
		key, value, err := parsePropertyFlag(prop)
		if err != nil {
			t.Fatalf("parsePropertyFlag(%q) unexpected error: %v", prop, err)
		}
		entity.SetString(key, value)
	}

	// Verify properties were set
	if got := entity.GetString("iso27001"); got != "A.5.15" {
		t.Errorf("entity.GetString(\"iso27001\") = %q, want %q", got, "A.5.15")
	}
	if got := entity.GetString("owner"); got != "Security Team" {
		t.Errorf("entity.GetString(\"owner\") = %q, want %q", got, "Security Team")
	}
}
