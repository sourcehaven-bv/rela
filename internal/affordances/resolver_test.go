package affordances_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"

	"gopkg.in/yaml.v3"
)

// stubLookup is a fake RelationLookup driven by in-memory edges.
type stubLookup struct {
	// edges["from|type|to"] = true
	edges map[string]bool
}

func newStubLookup(edges ...[3]string) *stubLookup {
	s := &stubLookup{edges: map[string]bool{}}
	for _, e := range edges {
		s.edges[e[0]+"|"+e[1]+"|"+e[2]] = true
	}
	return s
}

func (s *stubLookup) OutgoingCounts(_ context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for k := range s.edges {
		parts := strings.SplitN(k, "|", 3)
		if parts[0] == fromID {
			counts[parts[1]]++
		}
	}
	return counts
}

func (s *stubLookup) HasEdge(_ context.Context, fromID, relType, toID string) bool {
	return s.edges[fromID+"|"+relType+"|"+toID]
}

// testMeta builds a minimal metamodel with a ticket type carrying the
// properties the tests reference.
func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":      {Type: metamodel.PropertyTypeString},
					"status":     {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "review", "done"}},
					"assignee":   {Type: metamodel.PropertyTypeString},
					"priority":   {Type: metamodel.PropertyTypeInteger},
					"tags":       {Type: metamodel.PropertyTypeString, List: true},
					"is_blocked": {Type: metamodel.PropertyTypeBoolean},
				},
			},
			"feature":   {Properties: map[string]metamodel.PropertyDef{"title": {Type: metamodel.PropertyTypeString}}},
			"checklist": {Properties: map[string]metamodel.PropertyDef{"note": {Type: metamodel.PropertyTypeString}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements":   {From: []string{"ticket"}, To: []string{"feature"}},
			"blocks":       {From: []string{"ticket"}, To: []string{"ticket"}},
			"has-planning": {From: []string{"ticket"}, To: []string{"checklist"}},
		},
	}
}

func policyFromYAML(t *testing.T, src string) *acl.Policy {
	t.Helper()
	var p acl.Policy
	if err := yaml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	return &p
}

// ctxAs builds a request context for the named principal. user varies
// across the table even where current tests happen to use one value;
// keeping it explicit documents which principal each case exercises.
//
//nolint:unparam // user is intentionally explicit per-test
func ctxAs(user string) context.Context {
	return principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry})
}

func ticket(id string, props map[string]interface{}) *entity.Entity {
	e := entity.New(id, "ticket")
	for k, v := range props {
		e.Properties[k] = v
	}
	return e
}

// AC2: a policy with no affordance blocks yields empty verdicts
// (permissive default — byte-identical to Nop downstream).
func TestResolver_NoAffordanceBlocks_EmptyVerdicts(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  admin: { write: ["*"] }
assignments:
  alice: admin
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil))
	if fv.Writable != nil || fv.Visible != nil || fv.Options != nil {
		t.Errorf("expected empty verdicts, got %+v", fv)
	}
}

// AC3 + DR-C4: an opted-in fields block is closed-world. A granted
// field with a passing predicate is writable; an unlisted field is
// denied (writable=false).
func TestResolver_FieldsClosedWorld(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
assignments:
  alice: triager
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil))

	// status granted unconditionally → not in the deny map.
	if _, denied := fv.Writable["status"]; denied {
		t.Errorf("status should be writable (granted), got deny")
	}
	// title not granted → closed-world deny.
	if v, ok := fv.Writable["title"]; !ok || v {
		t.Errorf("title should be writable=false (closed-world), got ok=%v v=%v", ok, v)
	}
}

// AC3: a field grant whose predicate evaluates false denies the field.
func TestResolver_FieldPredicateFalse_Denies(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.assignee == current_user.id"
assignments:
  alice: triager
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// alice is NOT the assignee → status denied.
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", map[string]interface{}{"assignee": "bob"}))
	if v, ok := fv.Writable["status"]; !ok || v {
		t.Errorf("status should be denied (predicate false), got ok=%v v=%v", ok, v)
	}

	// alice IS the assignee → status writable (absent from deny map).
	fv = r.FieldVerdicts(ctxAs("alice"), ticket("T-2", map[string]interface{}{"assignee": "alice"}))
	if _, denied := fv.Writable["status"]; denied {
		t.Errorf("status should be writable when predicate passes")
	}
}

// AC4: an opted-in option grant filters disallowed options.
func TestResolver_OptionFiltered(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    options:
      ticket:
        - field: status
          option: open
        - field: status
          option: review
assignments:
  alice: triager
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil))

	// "done" is not granted → denied; open/review granted → absent.
	statusOpts := fv.Options["status"]
	if v, ok := statusOpts["done"]; !ok || v {
		t.Errorf("status=done should be denied, got ok=%v v=%v", ok, v)
	}
	if _, denied := statusOpts["open"]; denied {
		t.Errorf("status=open should be allowed")
	}
}

// AC5: a relation grant with create:false denies creation.
func TestResolver_RelationCreateFalse(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: implements
          create: false
          remove: true
assignments:
  alice: triager
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rv := r.RelationVerdicts(ctxAs("alice"), ticket("T-1", nil))
	v, ok := rv.Types["implements"]
	if !ok {
		t.Fatalf("implements verdict missing")
	}
	if v.Creatable {
		t.Errorf("implements should not be creatable")
	}
	if !v.Removable {
		t.Errorf("implements should be removable")
	}
}

// DR-S3: cross-role union is monotonic. A user with two roles, one
// granting status and one granting title, can write BOTH.
func TestResolver_CrossRoleUnion(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  a:
    fields:
      ticket:
        - field: status
  b:
    fields:
      ticket:
        - field: title
  everyone:
    fields:
      ticket:
        - field: status
        - field: title
assignments:
  alice: a
`)
	// alice has role a + everyone. everyone grants both; a grants status.
	// Union → both writable.
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil))
	if _, denied := fv.Writable["status"]; denied {
		t.Errorf("status should be writable via union")
	}
	if _, denied := fv.Writable["title"]; denied {
		t.Errorf("title should be writable via everyone-role union")
	}
}

// DR-C6: has_role with a local role conferred by a role-relation edge.
func TestResolver_LocalRole_HasRole(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  owner:
    fields:
      ticket:
        - field: status
          when: "has_role(current_user, entity, 'owner')"
assignments: {}
role_relations:
  owner-of:
    confers: owner
`)
	// alice --owner-of--> T-1 confers owner on T-1.
	lookup := newStubLookup([3]string{"alice", "owner-of", "T-1"})
	r, err := affordances.New(p, testMeta(t), lookup)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// On T-1, alice is owner → status writable.
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil))
	if _, denied := fv.Writable["status"]; denied {
		t.Errorf("status should be writable: alice is owner of T-1")
	}

	// On T-2, alice is NOT owner → owner role doesn't apply, no grants,
	// so no opted-in block → permissive (empty verdicts).
	fv = r.FieldVerdicts(ctxAs("alice"), ticket("T-2", nil))
	if len(fv.Writable) != 0 {
		t.Errorf("T-2: expected no verdicts (owner role absent), got %+v", fv.Writable)
	}
}

// DR-C7: has_global_role checks only assignment-based roles.
func TestResolver_HasGlobalRole(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  admin: {}
  triager:
    visible:
      ticket:
        - field: assignee
          when: "has_global_role(current_user, 'admin')"
assignments:
  alice: triager
  bob: admin
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// alice is triager but not admin → assignee hidden.
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil))
	if v, ok := fv.Visible["assignee"]; !ok || v {
		t.Errorf("assignee should be hidden for non-admin alice, got ok=%v v=%v", ok, v)
	}
}

// DR-C2: an off-type stored property (integer where string declared,
// or vice versa) coerces to Nil rather than failing Eval and locking
// the field. A predicate referencing it just sees nil.
func TestResolver_OffTypeProperty_CoercesNotFails(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.priority == 5"
assignments:
  alice: triager
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// priority declared integer; store it as a string "5" — coerces to
	// number 5, predicate passes, no Eval failure.
	fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", map[string]interface{}{"priority": "5"}))
	if _, denied := fv.Writable["status"]; denied {
		t.Errorf("status should be writable: priority '5' coerces to number 5")
	}

	// store a map where integer expected — coerces to Nil, predicate
	// (nil == 5) is false → denied, but NO Eval error / no panic.
	fv = r.FieldVerdicts(ctxAs("alice"),
		ticket("T-2", map[string]interface{}{"priority": map[string]interface{}{"x": 1}}))
	if v, ok := fv.Writable["status"]; !ok || v {
		t.Errorf("status should be denied (priority uncoercible → nil != 5), got ok=%v v=%v", ok, v)
	}
}

// A malformed predicate fails construction with the grant path in the
// error (DR-S2 multi-error collection).
func TestResolver_New_CompileError_IncludesPath(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.nonexistent_field == 1"
`)
	_, err := affordances.New(p, testMeta(t), newStubLookup())
	if err == nil {
		t.Fatal("expected compile error for unknown field reference")
	}
	if !contains(err.Error(), "roles.triager.fields.ticket[0].when") {
		t.Errorf("error should include grant path, got: %v", err)
	}
}

// S3 / RR-QV18: when two roles both deny the same field, the
// attributed role is deterministic (first in sorted order), not
// dependent on map iteration. Run repeatedly to catch flakiness.
func TestResolver_MultiRoleDeny_DeterministicAttribution(t *testing.T) {
	// Both zeta and alpha opt-in for status with a false predicate, so
	// status is denied by both. Sorted order → "alpha" is attributed.
	p := policyFromYAML(t, `
roles:
  zeta:
    fields:
      ticket:
        - field: status
          when: "entity.title == 'never-zeta'"
  everyone:
    fields:
      ticket:
        - field: status
          when: "entity.title == 'never-everyone'"
assignments:
  alice: zeta
`)
	// alice holds zeta (assigned) + everyone (implicit). Both deny
	// status. Effective roles sorted: everyone, zeta → "everyone" is
	// the first denier and thus the attributed role.
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var first string
	for i := range 20 {
		fv := r.FieldVerdicts(ctxAs("alice"), ticket("T-1", nil))
		got := fv.Attribution["status"]
		if got == "" {
			t.Fatalf("expected attribution for denied status")
		}
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("attribution flaky: run %d got %q, want stable %q", i, got, first)
		}
	}
	// "everyone" sorts before "zeta", so it's the attributed (first) role.
	if !contains(first, "role=everyone") {
		t.Errorf("expected first sorted denier 'everyone', got %q", first)
	}
}

// S2 / RR-RTJE: a field grant naming a property the metamodel doesn't
// declare fails construction with the grant path — a typo can't
// silently invert closed-world intent.
func TestResolver_New_UnknownFieldTarget_Rejected(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: stauts
`)
	_, err := affordances.New(p, testMeta(t), newStubLookup())
	if err == nil {
		t.Fatal("expected error for unknown field target")
	}
	if !contains(err.Error(), "unknown field \"stauts\"") ||
		!contains(err.Error(), "roles.triager.fields.ticket[0]") {

		t.Errorf("error should name the bad field and path, got: %v", err)
	}
}

// S2: an option grant on a non-declared option value is rejected.
func TestResolver_New_UnknownOptionTarget_Rejected(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    options:
      ticket:
        - field: status
          option: nonexistent
`)
	_, err := affordances.New(p, testMeta(t), newStubLookup())
	if err == nil {
		t.Fatal("expected error for unknown option")
	}
	if !contains(err.Error(), "not a declared value") {
		t.Errorf("error should flag the bad option, got: %v", err)
	}
}

// S2: a relation grant on a type the metamodel lacks is rejected.
func TestResolver_New_UnknownRelationTarget_Rejected(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: nonexistent-rel
`)
	_, err := affordances.New(p, testMeta(t), newStubLookup())
	if err == nil {
		t.Fatal("expected error for unknown relation type")
	}
	if !contains(err.Error(), "unknown relation type") {
		t.Errorf("error should flag the bad relation, got: %v", err)
	}
}

// S4: relation meta-field grants gate per-link meta writability, and a
// failing meta predicate denies just that field (AND-ed with the
// grant). Exercises the previously-untested relation_accum meta path.
func TestResolver_RelationMetaField(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    relations:
      ticket:
        - relation: has-planning
          fields:
            - field: note
              when: "entity.status == 'open'"
assignments:
  alice: triager
`)
	r, err := affordances.New(p, testMeta(t), newStubLookup())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// status=done → note meta-field denied (predicate false).
	rv := r.RelationVerdicts(ctxAs("alice"), ticket("T-1", map[string]interface{}{"status": "done"}))
	v, ok := rv.Types["has-planning"]
	if !ok {
		t.Fatalf("has-planning verdict missing")
	}
	if w, ok := v.Fields["note"]; !ok || w {
		t.Errorf("note meta should be denied (predicate false), got ok=%v w=%v", ok, w)
	}

	// status=open → note meta-field allowed (sparse: absent from Fields,
	// or the type itself fully permissive and absent from Types).
	rv = r.RelationVerdicts(ctxAs("alice"), ticket("T-2", map[string]interface{}{"status": "open"}))
	if v, ok := rv.Types["has-planning"]; ok {
		if w, denied := v.Fields["note"]; denied && !w {
			t.Errorf("note meta should be writable when predicate passes")
		}
	}
}

// ctxKey is a private context key for the propagation test.
type ctxKey struct{}

// ctxRecordingLookup records the context handed to OutgoingCounts so a
// test can assert the caller's ctx (not context.Background()) reaches
// the host-function path during predicate evaluation.
type ctxRecordingLookup struct {
	*stubLookup
	gotMarker string
}

func (l *ctxRecordingLookup) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		l.gotMarker = v
	}
	return l.stubLookup.OutgoingCounts(ctx, fromID)
}

// IB-finding #2 (tschmits) / PR#825 pattern: the caller's request
// context must thread into predicate Eval and the host-function calls
// it makes, rather than being dropped for context.Background(). A
// predicate that invokes a relation host func is evaluated with a
// marker on the context; the lookup must observe it.
func TestResolver_CallerContext_ThreadsToHostFuncs(t *testing.T) {
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "has_relation(entity, 'blocks')"
assignments:
  alice: triager
`)
	lookup := &ctxRecordingLookup{stubLookup: newStubLookup([3]string{"T-1", "blocks", "T-9"})}
	r, err := affordances.New(p, testMeta(t), lookup)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.WithValue(ctxAs("alice"), ctxKey{}, "caller-marker")
	_ = r.FieldVerdicts(ctx, ticket("T-1", nil))

	if lookup.gotMarker != "caller-marker" {
		t.Errorf("host func saw ctx marker %q, want %q — caller context was not threaded into predicate Eval",
			lookup.gotMarker, "caller-marker")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
