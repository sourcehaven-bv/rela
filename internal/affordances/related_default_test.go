package affordances_test

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// defaultWorld builds tickets that each implement one feature. The feature's
// blocked property defaults to false and its stage to draft; owner has no
// default. T-UNSET's feature has none written; T-NONE implements nothing.
// editor may write secret and viewer may see it only when the given constraint holds.
func defaultWorld(t *testing.T, constraint string) fieldWorld {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: metamodel.PropertyTypeString},
				"secret": {Type: metamodel.PropertyTypeString},
			}},
			"feature": {Properties: map[string]metamodel.PropertyDef{
				"blocked": {Type: metamodel.PropertyTypeBoolean, Default: "false"},
				"stage":   {Type: metamodel.PropertyTypeEnum, Values: []string{"draft", "live"}, Default: "draft"},
				"owner":   {Type: metamodel.PropertyTypeString},
			}},
			"person": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {From: []string{"ticket"}, To: []string{"feature"}},
		},
	}
	when := fmt.Sprintf("related(entity, 'implements', { %s })", constraint)
	policy, err := acl.LoadPolicyBytes(fmt.Appendf(nil, `
roles:
  editor:
    read: [ticket, feature]
    update: [ticket]
    fields:
      ticket:
        - field: title
        - field: secret
          when: %q
  viewer:
    read: [ticket, feature]
    visible:
      ticket:
        - field: title
        - field: secret
          when: %q
assignments:
  erin: editor
  vic: viewer
`, when, when))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	ms := memstore.New()
	for _, p := range []string{"erin", "vic"} {
		mustCreateEntity(t, ms, p, "person")
	}
	features := map[string]map[string]any{
		"F-UNSET": nil,
		"F-FALSE": {"blocked": false, "stage": "draft"},
		"F-TRUE":  {"blocked": true, "stage": "live", "owner": "x"},
		"F-EMPTY": {"blocked": "", "stage": "", "owner": ""},
	}
	for id, props := range features {
		e := entity.New(id, "feature")
		maps.Copy(e.Properties, props)
		mustCreate(t, ms, e)
	}
	tickets := map[string]*entity.Entity{}
	for _, id := range []string{"T-UNSET", "T-FALSE", "T-TRUE", "T-EMPTY", "T-NONE"} {
		e := entity.New(id, "ticket")
		e.SetString("title", "title "+id)
		e.SetString("secret", "secret "+id)
		mustCreate(t, ms, e)
		tickets[id] = e
	}
	for _, suffix := range []string{"UNSET", "FALSE", "TRUE", "EMPTY"} {
		mustCreateRelation(t, ms, "T-"+suffix, "implements", "F-"+suffix)
	}
	decl, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	binder, err := relresolve.NewStoreBinder(meta, relresolve.Ungated, ms)
	if err != nil {
		t.Fatalf("NewStoreBinder: %v", err)
	}
	resolver, err := affordances.New(meta, storeRelationLookup{ms}, decl, affordances.WithTraversals(binder))
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	return fieldWorld{meta: meta, ms: ms, decl: decl, resolver: resolver, tickets: tickets}
}

// allowsSecret reads a verdict map, in which an absent field is allowed.
func allowsSecret(verdicts map[string]bool) bool {
	v, present := verdicts["secret"]
	return !present || v
}

// A related() constraint in a grant's or a visible: rule's when: that compares
// against the target's schema default also holds when the target never had
// the property written. A non-default value, another value, and a missing
// target do not satisfy it, nor does an unset or empty target on a property
// with no declared default. The visible: rule is checked both per row and
// through the store query search runs.
func TestRelatedWhen_DefaultMatchesUnsetTarget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		constraint string
		applies    []string
	}{
		{"blocked = false", []string{"T-EMPTY", "T-FALSE", "T-UNSET"}},
		{"blocked = true", []string{"T-TRUE"}},
		{"stage = 'draft'", []string{"T-EMPTY", "T-FALSE", "T-UNSET"}},
		{"stage = 'live'", []string{"T-TRUE"}},
		{"blocked = false, stage = 'draft'", []string{"T-EMPTY", "T-FALSE", "T-UNSET"}},
		{"blocked = false, stage = 'live'", nil},
		{"owner = 'x'", []string{"T-TRUE"}},
	}
	for _, tc := range cases {
		t.Run(tc.constraint, func(t *testing.T) {
			t.Parallel()
			w := defaultWorld(t, tc.constraint)
			erin, vic := w.ctxFor(t, "erin"), w.ctxFor(t, "vic")
			scope, err := w.resolver.SearchFieldScope(vic, "ticket")
			if err != nil {
				t.Fatalf("SearchFieldScope: %v", err)
			}
			for id, row := range w.tickets {
				want := slices.Contains(tc.applies, id)
				if got := allowsSecret(w.resolver.FieldVerdicts(erin, row).Writable); got != want {
					t.Errorf("%s: fields: grant writable = %v, want %v", id, got, want)
				}
				if got := allowsSecret(w.resolver.FieldVerdicts(vic, row).Visible); got != want {
					t.Errorf("%s: visible: grant shows secret = %v, want %v", id, got, want)
				}
				hidden := slices.Contains(scopeHidden(t, w, scope, row), "secret")
				if hidden == want {
					t.Errorf("%s: search scope hides secret = %v, want %v", id, hidden, !want)
				}
			}
		})
	}
}
