package automation

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// filePropMeta declares a type with a file property next to a plain one.
func filePropMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"doc": {
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: metamodel.PropertyTypeString},
					"spec":   {Type: metamodel.PropertyTypeFile},
				},
			},
			"note": {
				Properties: map[string]metamodel.PropertyDef{
					"status": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}
}

// A file value is the capability to download the bytes it names
// (BUG-CTUW2N), so an automation may not write one. The engine applies
// `set:` after the manager's file-property check, so the refusal has to
// happen when the automation loads.
func TestEngine_FilePropertyWriteRefusedAtLoad(t *testing.T) {
	tests := []struct {
		name    string
		def     metamodel.AutomationDef
		wantErr bool
	}{
		{
			name: "set on a file property of the trigger type",
			def: metamodel.AutomationDef{
				Name: "stamp", On: metamodel.AutomationTrigger{Entity: metamodel.StringOrSlice{"doc"}, Created: true},
				Do: []metamodel.AutomationAction{{Set: "spec", Value: "attachments/X/spec/a.txt"}},
			},
			wantErr: true,
		},
		{
			name: "set on a file property of some type when the trigger names none",
			def: metamodel.AutomationDef{
				Name: "stamp", On: metamodel.AutomationTrigger{Created: true},
				Do: []metamodel.AutomationAction{{Set: "spec", Value: "a.txt"}},
			},
			wantErr: true,
		},
		{
			name: "create_entity setting a file property",
			def: metamodel.AutomationDef{
				Name: "stamp", On: metamodel.AutomationTrigger{Entity: metamodel.StringOrSlice{"note"}, Created: true},
				Do: []metamodel.AutomationAction{{CreateEntity: &metamodel.CreateEntityAction{
					Type: "doc", Properties: map[string]string{"spec": "{{new.status}}"},
				}}},
			},
			wantErr: true,
		},
		{
			name: "set on a plain property",
			def: metamodel.AutomationDef{
				Name: "stamp", On: metamodel.AutomationTrigger{Entity: metamodel.StringOrSlice{"doc"}, Created: true},
				Do: []metamodel.AutomationAction{{Set: "status", Value: "new"}},
			},
		},
		{
			name: "set on a name that is a file property only of another type",
			def: metamodel.AutomationDef{
				Name: "stamp", On: metamodel.AutomationTrigger{Entity: metamodel.StringOrSlice{"note"}, Created: true},
				Do: []metamodel.AutomationAction{{Set: "spec", Value: "x"}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEngineFromMetamodel(filePropMeta(), []metamodel.AutomationDef{tc.def})
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("unexpected load error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("want a load error for an automation writing a file property, got none")
			}
			if !strings.Contains(err.Error(), "stamp") || !strings.Contains(err.Error(), "spec") {
				t.Errorf("error %q does not name the automation and the property", err)
			}
		})
	}
}
