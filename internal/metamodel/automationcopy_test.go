package metamodel

import (
	"strings"
	"testing"
)

// TestValidateAutomationCopies pins the load errors of `on.copy:`
// (TKT-50DZEHI): each one names a scope that could never match.
func TestValidateAutomationCopies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		on      AutomationTrigger
		wantErr string // substring; empty = must validate
	}{
		{name: "copy alone", on: AutomationTrigger{Copy: StringOrSlice{"promote"}}},
		{name: "copy with matching entity and face", on: AutomationTrigger{
			Entity: StringOrSlice{"page"}, Faces: StringOrSlice{"published"}, Copy: StringOrSlice{"promote"}, Created: true,
		}},
		{name: "cross-entity copy into the default face", on: AutomationTrigger{
			Entity: StringOrSlice{"ticket"}, Copy: StringOrSlice{"spawn"},
		}},
		{name: "undeclared copy", on: AutomationTrigger{Copy: StringOrSlice{"nope"}},
			wantErr: "`on.copy: [nope]` names no declared copy"},
		{name: "entity the copy does not write", on: AutomationTrigger{
			Entity: StringOrSlice{"ticket"}, Copy: StringOrSlice{"promote"},
		}, wantErr: "writes \"page\", which `on.entity` does not name"},
		{name: "face the copy does not write", on: AutomationTrigger{
			Faces: StringOrSlice{"draft"}, Copy: StringOrSlice{"promote"},
		}, wantErr: "writes face \"published\", which `on.faces` does not name"},
		{name: "relation event", on: AutomationTrigger{
			Copy: StringOrSlice{"promote"}, RelationCreated: "implements",
		}, wantErr: "a copy raises only entity events"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := copyFixture(map[string]CopyDef{
				"promote": {From: "page@draft", To: "page@published", AllFields: true, Guard: guarded},
				"spawn":   {From: "ticket", To: "new ticket", Fields: map[string]string{"title": "{{new.title}}"}},
			})
			m.Automations = []AutomationDef{{Name: "a", On: tc.on}}
			errs := validateAutomationCopies(m)
			joined := strings.Join(errs, "\n")
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("must validate; got: %s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantErr) {
				t.Errorf("want an error containing %q; got: %s", tc.wantErr, joined)
			}
		})
	}
}
