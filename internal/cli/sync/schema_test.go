package sync

import (
	"strings"
	"testing"
)

// TestCheckSchemaCompatible pins the handshake's compatibility rules: it fails
// on a missing type, a plural mismatch, a missing property, and — the point of
// RR-SYNCR5's shape fix — a property whose TYPE or LIST-ness drifted. A remote
// that merely has EXTRA types/properties the replica ignores is compatible.
func TestCheckSchemaCompatible(t *testing.T) {
	remote := &RemoteSchema{
		Entities: map[string]remoteEntityType{
			"ticket": {Plural: "tickets", Properties: map[string]remotePropDef{
				"title":    {Type: "string"},
				"priority": {Type: "number"},
				"tags":     {Type: "string", List: true},
				"extra":    {Type: "string"}, // remote-only; replica ignores → fine
			}},
		},
		Relations: map[string]remoteRelationType{
			"blocks": {Properties: map[string]remotePropDef{"reason": {Type: "string"}}},
		},
	}

	tests := []struct {
		name      string
		local     LocalSchema
		wantOK    bool
		wantMatch string // substring the error must contain (when !wantOK)
	}{
		{
			name: "identical subset is compatible",
			local: LocalSchema{Entities: map[string]LocalType{
				"ticket": {Plural: "tickets", Properties: map[string]LocalProp{
					"title": {Type: "string"}, "priority": {Type: "number"}, "tags": {Type: "string", List: true},
				}},
			}},
			wantOK: true,
		},
		{
			name: "remote-only extras are fine (replica ignores them)",
			local: LocalSchema{Entities: map[string]LocalType{
				"ticket": {Plural: "tickets", Properties: map[string]LocalProp{"title": {Type: "string"}}},
			}},
			wantOK: true,
		},
		{
			name: "unknown entity type",
			local: LocalSchema{Entities: map[string]LocalType{
				"gadget": {Plural: "gadgets"},
			}},
			wantMatch: `entity type "gadget" exists locally but not on the remote`,
		},
		{
			name: "plural mismatch",
			local: LocalSchema{Entities: map[string]LocalType{
				"ticket": {Plural: "tix"},
			}},
			wantMatch: "has plural",
		},
		{
			name: "missing property",
			local: LocalSchema{Entities: map[string]LocalType{
				"ticket": {Plural: "tickets", Properties: map[string]LocalProp{"ghost": {Type: "string"}}},
			}},
			wantMatch: `property "ghost" exists locally but not on the remote`,
		},
		{
			name: "property TYPE drift (the corruption vector)",
			local: LocalSchema{Entities: map[string]LocalType{
				"ticket": {Plural: "tickets", Properties: map[string]LocalProp{"priority": {Type: "string"}}},
			}},
			wantMatch: `is type "string" locally but "number" on the remote`,
		},
		{
			name: "property LIST drift",
			local: LocalSchema{Entities: map[string]LocalType{
				"ticket": {Plural: "tickets", Properties: map[string]LocalProp{"tags": {Type: "string", List: false}}},
			}},
			wantMatch: "list=false locally but list=true on the remote",
		},
		{
			name: "relation property drift",
			local: LocalSchema{Relations: map[string]LocalType{
				"blocks": {Properties: map[string]LocalProp{"reason": {Type: "number"}}},
			}},
			wantMatch: `relation type "blocks" property "reason" is type "number" locally but "string"`,
		},
		{
			name: "unknown relation type",
			local: LocalSchema{Relations: map[string]LocalType{
				"needs": {},
			}},
			wantMatch: `relation type "needs" exists locally but not on the remote`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := remote.CheckSchemaCompatible(tc.local)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("want compatible, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want incompatible (%q), got nil", tc.wantMatch)
			}
			if !strings.Contains(err.Error(), tc.wantMatch) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantMatch)
			}
		})
	}
}
