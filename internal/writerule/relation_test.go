package writerule_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/writerule"
)

const grcYAML = `
version: "1.0"
entities:
  risico:
    label: Risico
    id_prefix: "RIS-"
    id_type: sequential
    properties:
      status: {type: string}
  behandelplan:
    label: Behandelplan
    id_prefix: "BP-"
    id_type: sequential
    properties:
      titel: {type: string}
  maatregel:
    label: Maatregel
    id_prefix: "MAAT-"
    id_type: sequential
    properties:
      toepasbaar: {type: boolean}
  control:
    label: Control
    id_prefix: "C-"
    id_type: sequential
    properties:
      titel: {type: string}
  pagina:
    label: Pagina
    id_prefix: "PG-"
    id_type: sequential
    faces:
      concept: {}
    properties:
      titel: {type: string}
relations:
  heeft_behandelplan:
    from: [risico]
    to: [behandelplan]
    inverse: behandelt
  implementeert:
    from: [maatregel]
    to: [control, behandelplan]
  linkt:
    from: [pagina]
    to: [control]
`

func parseGRC(t *testing.T, validations string) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(grcYAML + validations))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return meta
}

// fakeEdges answers Related from a fixed set of (relation, incoming, target)
// triples.
type fakeEdges struct {
	has map[writerule.Hop]map[string]bool
	err error
}

func edges(entries ...any) fakeEdges {
	f := fakeEdges{has: map[writerule.Hop]map[string]bool{}}
	for i := 0; i+1 < len(entries); i += 2 {
		hop := entries[i].(writerule.Hop)
		if f.has[hop] == nil {
			f.has[hop] = map[string]bool{}
		}
		f.has[hop][entries[i+1].(string)] = true
	}
	return f
}

func (f fakeEdges) Related(_ context.Context, hop writerule.Hop, target string) (bool, error) {
	return f.has[hop][target], f.err
}

const planRule = `
validations:
  - name: behandelplan-nodig
    description: Een risico in behandeling heeft een behandelplan.
    entity_type: risico
    enforce: write
    when: ["status=in_behandeling"]
    then_condition: "related(entity, 'heeft_behandelplan')"
`

var (
	planHop     = writerule.Hop{Relation: "heeft_behandelplan"}
	behandeltOp = writerule.Hop{Relation: "heeft_behandelplan", Incoming: true}
	implHop     = writerule.Hop{Relation: "implementeert"}
)

func risk(status string) *entity.Entity {
	return &entity.Entity{ID: "RIS-1", Type: "risico", Properties: map[string]any{"status": status}}
}

func TestCheckChange(t *testing.T) {
	none := edges()
	withPlan := edges(planHop, "behandelplan")
	tests := []struct {
		name        string
		validations string
		before      *entity.Entity
		after       *entity.Entity
		was, now    fakeEdges
		wantRules   []string
	}{
		{
			name: "status change without a plan is refused", validations: planRule,
			before: risk("nieuw"), after: risk("in_behandeling"), was: none, now: none,
			wantRules: []string{"behandelplan-nodig"},
		},
		{
			name: "status change with a plan is accepted", validations: planRule,
			before: risk("nieuw"), after: risk("in_behandeling"), was: withPlan, now: withPlan,
		},
		{
			name: "deleting the last plan is refused", validations: planRule,
			before: risk("in_behandeling"), after: risk("in_behandeling"), was: withPlan, now: none,
			wantRules: []string{"behandelplan-nodig"},
		},
		{
			name: "a row that already breaks the rule stays writable", validations: planRule,
			before: risk("in_behandeling"), after: risk("in_behandeling"), was: none, now: none,
		},
		{
			name: "a write that leaves the rule out is accepted", validations: planRule,
			before: risk("in_behandeling"), after: risk("gesloten"), was: withPlan, now: none,
		},
		{
			name: "inverse name reads incoming edges",
			validations: `
validations:
  - name: plan-behandelt-iets
    entity_type: behandelplan
    enforce: write
    then_condition: "related(entity, 'behandelt')"
`,
			before:    &entity.Entity{ID: "BP-1", Type: "behandelplan"},
			after:     &entity.Entity{ID: "BP-1", Type: "behandelplan"},
			was:       edges(behandeltOp, "risico"),
			now:       none,
			wantRules: []string{"plan-behandelt-iets"},
		},
		{
			name: "type ascription on a union target",
			validations: `
validations:
  - name: toepasbaar-implementeert-control
    entity_type: maatregel
    enforce: write
    when_condition: "entity.toepasbaar == true"
    then_condition: "related(entity, 'implementeert', {type = 'control'})"
`,
			before: &entity.Entity{ID: "MAAT-1", Type: "maatregel", Properties: map[string]any{"toepasbaar": false}},
			after:  &entity.Entity{ID: "MAAT-1", Type: "maatregel", Properties: map[string]any{"toepasbaar": true}},
			// An edge to a behandelplan does not satisfy a rule that asks
			// for a control.
			was: edges(implHop, "behandelplan"), now: edges(implHop, "behandelplan"),
			wantRules: []string{"toepasbaar-implementeert-control"},
		},
		{
			name: "relation in when",
			validations: `
validations:
  - name: met-plan-geen-nieuw
    entity_type: risico
    enforce: write
    when_condition: "related(entity, 'heeft_behandelplan')"
    then: ["status!=nieuw"]
`,
			before: risk("nieuw"), after: risk("nieuw"), was: none, now: withPlan,
			wantRules: []string{"met-plan-geen-nieuw"},
		},
		{
			name: "property rules are not checked here",
			validations: `
validations:
  - name: status-gezet
    entity_type: risico
    enforce: write
    then: ["status!="]
`,
			before: risk("nieuw"), after: risk(""), was: none, now: none,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set, err := writerule.NewSet(parseGRC(t, tc.validations))
			if err != nil {
				t.Fatalf("NewSet: %v", err)
			}
			got, err := set.CheckChange(context.Background(), tc.before, tc.after, tc.was, tc.now)
			if err != nil {
				t.Fatalf("CheckChange: %v", err)
			}
			if len(got) != len(tc.wantRules) {
				t.Fatalf("got %d violations %v, want rules %v", len(got), got, tc.wantRules)
			}
			for i, v := range got {
				if v.Type != metamodel.ValidationErrorRule || v.Property != "" {
					t.Errorf("violation %d = %+v, want a rule error without a property", i, v)
				}
				if !strings.Contains(v.Message, `"`+tc.wantRules[i]+`"`) {
					t.Errorf("violation %d message %q does not name rule %q", i, v.Message, tc.wantRules[i])
				}
			}
		})
	}
}

// TestCheck_LeavesRelationRulesOut pins that the property check, which
// runs on creates too, never evaluates a relation rule: a new entity has no
// edges until the request links it.
func TestCheck_LeavesRelationRulesOut(t *testing.T) {
	set, err := writerule.NewSet(parseGRC(t, planRule))
	if err != nil {
		t.Fatal(err)
	}
	if got := set.Check(context.Background(), risk("in_behandeling")); got != nil {
		t.Fatalf("Check = %v, want nil", got)
	}
}

func TestCheckChange_StoreFaultIsAnError(t *testing.T) {
	set, err := writerule.NewSet(parseGRC(t, planRule))
	if err != nil {
		t.Fatal(err)
	}
	fault := errors.New("store down")
	broken := fakeEdges{err: fault}
	_, err = set.CheckChange(context.Background(), risk("nieuw"), risk("in_behandeling"), broken, broken)
	if !errors.Is(err, fault) {
		t.Fatalf("err = %v, want the store fault", err)
	}
	if _, err := set.CheckChange(context.Background(), nil, risk("x"), broken, broken); err == nil {
		t.Fatal("nil before accepted")
	}
	var nilSet *writerule.Set
	if got, err := nilSet.CheckChange(context.Background(), nil, nil, nil, nil); got != nil || err != nil {
		t.Fatalf("nil set = %v, %v", got, err)
	}
}

func TestRelationReads(t *testing.T) {
	set, err := writerule.NewSet(parseGRC(t, planRule))
	if err != nil {
		t.Fatal(err)
	}
	if !set.ReadsRelation("heeft_behandelplan") || set.ReadsRelation("implementeert") {
		t.Error("ReadsRelation does not match the rule")
	}
	if !set.HasRelationRules("risico") || set.HasRelationRules("behandelplan") {
		t.Error("HasRelationRules does not match the rule")
	}
	if !set.ReadsHop("risico", planHop) || set.ReadsHop("risico", behandeltOp) || set.ReadsHop("behandelplan", planHop) {
		t.Error("ReadsHop does not match the rule")
	}
	var nilSet *writerule.Set
	if nilSet.ReadsRelation("x") || nilSet.HasRelationRules("x") || nilSet.ReadsHop("x", planHop) {
		t.Error("nil set reads something")
	}
}

func TestNewSet_RefusesRelationShapes(t *testing.T) {
	tests := []struct {
		name string
		cond string
		typ  string
		want string
	}{
		{"path of two hops", "related(entity, {'heeft_behandelplan', 'behandelt'})", "risico", "one relation"},
		{"property constraint", "related(entity, 'heeft_behandelplan', {titel = 'x'})", "risico", "only the `type`"},
		{"id constraint", "related(entity, 'heeft_behandelplan', {id = 'BP-1'})", "risico", "only the `type`"},
		{"unknown relation", "related(entity, 'nope')", "risico", "unknown relation"},
		{"union without type", "related(entity, 'implementeert')", "maatregel", "type"},
		{"faced type", "related(entity, 'linkt')", "pagina", "faces"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := writerule.NewSet(parseGRC(t, `
validations:
  - name: r
    entity_type: `+tc.typ+`
    enforce: write
    then_condition: "`+tc.cond+`"
`))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("NewSet error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
