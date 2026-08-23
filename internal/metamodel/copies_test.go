package metamodel

import (
	"strings"
	"testing"
)

// copyFixture is a metamodel with two pointered types and one relation, so
// each validation case varies only the copy definition.
func copyFixture(copies map[string]CopyDef) *Metamodel {
	m := &Metamodel{
		Entities: map[string]EntityDef{
			"page": {
				Label:    "Page",
				Pointers: map[string]PointerDef{"draft": {Default: true}, "published": {}},
				Properties: map[string]PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
			},
			"ticket": {
				Label:      "Ticket",
				Properties: map[string]PropertyDef{"title": {Type: "string"}},
			},
		},
		Relations: map[string]RelationDef{
			// Content-scoped: copyable.
			"implements": {
				From: []string{"page"}, To: []string{"ticket"},
				Scope: ScopeContent,
			},
			// Identity-scoped: NOT copyable — it attaches to the bare id.
			"owned-by": {
				From: []string{"page"}, To: []string{"ticket"},
				Scope: ScopeIdentityExplicit,
			},
			// Content-scoped but originating elsewhere.
			"blocks": {
				From: []string{"ticket"}, To: []string{"page"},
				Scope: ScopeContent,
			},
		},
		Copies: copies,
	}
	m.InitAliases()
	return m
}

// TestValidateCopies_FieldsAllRejectedCrossEntity is the load error the
// cross-entity half of the elevation split turns on.
//
// A cross-entity copy reads through the CALLER'S visibility gate, so
// `fields: all` would copy whatever survived redaction as though it were the
// whole entity — destroying every field the principal could not see. That is
// the redacted read-modify-write forbidden everywhere else in this codebase,
// and it is refused at LOAD so an operator gets a startup message instead of
// silent field destruction on the first copy.
func TestValidateCopies_FieldsAllRejectedCrossEntity(t *testing.T) {
	t.Parallel()

	// Cross-entity: refused.
	errs := validateCopies(copyFixture(map[string]CopyDef{
		"spawn": {From: "ticket", To: "new ticket", AllFields: true},
	}))
	if len(errs) == 0 {
		t.Fatal("`fields: all` on a cross-entity copy must be a load error — it " +
			"would write a redacted entity")
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "visibility gate") {
		t.Errorf("the error must explain WHY, so the operator does not read it as "+
			"an arbitrary restriction; got: %s", joined)
	}

	// Same-entity: allowed — it runs elevated, so every field is readable and
	// a full replace is the promote case.
	if errs := validateCopies(copyFixture(map[string]CopyDef{
		"promote": {From: "page@draft", To: "page@published", AllFields: true, Guard: guarded},
	})); len(errs) != 0 {
		t.Errorf("`fields: all` is the promote case and must be allowed on a "+
			"same-entity copy; got: %v", errs)
	}
}

// guarded is the guard every definition targeting a non-default face must
// carry — see validateCopy's guarded-face check.
var guarded = CopyGuard{Permission: "promote-page"}

func TestValidateCopies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		def     CopyDef
		wantErr string // substring; empty = must validate
	}{
		{
			name: "valid same-entity promote",
			def:  CopyDef{From: "page@draft", To: "page@published", AllFields: true, Guard: guarded},
		},
		{
			name: "valid cross-entity spawn",
			def: CopyDef{
				From: "ticket", To: "new ticket",
				Fields: map[string]string{"title": "Follow-up: {{source.title}}"},
			},
		},
		{
			name:    "undeclared source type",
			def:     CopyDef{From: "nosuchtype@draft", To: "page@published", AllFields: true},
			wantErr: "entity type \"nosuchtype\" is not declared",
		},
		{
			name:    "undeclared pointer on a declared type",
			def:     CopyDef{From: "page@nosuchstate", To: "page@published", AllFields: true},
			wantErr: "declares no content state \"nosuchstate\"",
		},
		{
			name:    "a type with no pointers cannot name a face",
			def:     CopyDef{From: "ticket@draft", To: "page@published", AllFields: true},
			wantErr: "declares no content state \"draft\"",
		},
		{
			name: "undeclared target field",
			def: CopyDef{
				From: "page@draft", To: "page@published", Guard: guarded,
				Fields: map[string]string{"nosuchfield": "x"},
			},
			wantErr: "declares no property \"nosuchfield\"",
		},
		{
			name: "invalid relation mode",
			def: CopyDef{
				From: "page@draft", To: "page@published", AllFields: true, Guard: guarded,
				Relations: map[string]string{"implements": "copy"},
			},
			wantErr: "is not a valid mode",
		},
		{
			name: "undeclared relation type",
			def: CopyDef{
				From: "page@draft", To: "page@published", AllFields: true, Guard: guarded,
				Relations: map[string]string{"nosuchrel": "merge"},
			},
			wantErr: "relation type \"nosuchrel\" is not declared",
		},
		{
			name: "relation cannot originate from the target type",
			def: CopyDef{
				From: "page@draft", To: "page@published", AllFields: true, Guard: guarded,
				// blocks runs FROM ticket, so a page face cannot hold one.
				Relations: map[string]string{"blocks": "merge"},
			},
			wantErr: "cannot originate from entity type \"page\"",
		},
		{
			name:    "copies nothing",
			def:     CopyDef{From: "page@draft", To: "page@published", Guard: guarded},
			wantErr: "copies no fields",
		},
		{
			name: "both fields: all and a mapping",
			def: CopyDef{
				From: "page@draft", To: "page@published", AllFields: true, Guard: guarded,
				Fields: map[string]string{"title": "x"},
			},
			wantErr: "declares both",
		},
		{
			name:    "a new target must not name a face",
			def:     CopyDef{From: "ticket", To: "new ticket@draft", AllFields: true},
			wantErr: "must not name one",
		},
		{
			name:    "empty from",
			def:     CopyDef{From: "", To: "page@published", AllFields: true, Guard: guarded},
			wantErr: "empty target",
		},
		{
			name:    "a guarded face requires a guard",
			def:     CopyDef{From: "page@draft", To: "page@published", AllFields: true},
			wantErr: "declares no `guard:",
		},
		{
			name: "an identity-scoped relation cannot be copied",
			def: CopyDef{
				From: "page@draft", To: "page@published", AllFields: true, Guard: guarded,
				Relations: map[string]string{"owned-by": "merge"},
			},
			wantErr: "is identity-scoped",
		},
		{
			name: "guard.when is refused while unimplemented",
			def: CopyDef{
				From: "page@draft", To: "page@published", AllFields: true,
				Guard: CopyGuard{Permission: "promote", When: "source.status == 'approved'"},
			},
			wantErr: "not implemented yet",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := validateCopies(copyFixture(map[string]CopyDef{"c": tc.def}))
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

// TestCopyDef_IsSameEntity pins the ELEVATION BOUNDARY. Getting this
// predicate wrong in the permissive direction means a cross-entity copy runs
// elevated, which launders fields the principal cannot read into an entity
// with a different audience.
func TestCopyDef_IsSameEntity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		from, to string
		want     bool
	}{
		{"page@draft", "page@published", true},
		{"page", "page@draft", true},
		{"ticket", "new ticket", false},
		{"page@draft", "new page", false},
		{"page@draft", "ticket@draft", false},
		{"", "page@published", false}, // unparseable: not same-entity
		{"page@draft", "", false},     // unparseable: not same-entity
		{"page@draft", "new ticket@draft", false},
	}
	for _, tc := range tests {
		def := CopyDef{From: tc.from, To: tc.to}
		if got := def.IsSameEntity(); got != tc.want {
			t.Errorf("IsSameEntity(%q -> %q) = %v, want %v — this predicate decides "+
				"whether the copy runs ELEVATED", tc.from, tc.to, got, tc.want)
		}
	}
}
