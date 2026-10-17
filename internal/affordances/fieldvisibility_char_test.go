package affordances_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// fieldWorld is the fixture for the `visible:` characterization and the
// search field-scope differential. It covers every way a `visible:` block
// reaches a row:
//
//   - staff (global) declares {title, status}: closes the type everywhere.
//   - auditor (via the audit-team group) declares secret when the ticket is
//     done: a role union with staff, conditional per row.
//   - owner (conferred by owner-of, inherited through belongs-to) declares
//     title, and code when the ticket is open: closes the type only on the
//     rows the principal owns.
//   - reader (global, rita) and watcher (conferred by watches) read tickets
//     and declare no block: each shows every field where it applies, so
//     holding one never narrows what a declaring role shows (BUG-Y5UF3K).
//   - pubreader (global, pia) reads only published pages and declares no
//     block: it opens a page row by face.
//   - every ticket carries an undeclared `extra` property, which no block
//     can name.
//   - page is a faced type (draft, published). Its rows exercise grants
//     whose condition is evaluated per face, and the owner role held
//     directly and inherited through belongs-to.
type fieldWorld struct {
	meta     *metamodel.Metamodel
	ms       *memstore.MemStore
	decl     *acl.Declarative
	resolver *affordances.PolicyResolver
	tickets  map[string]*entity.Entity
	pages    []*entity.Entity
}

const fieldWorldPolicy = `
roles:
  everyone:
    read: [project]
  reader:
    read: [ticket]
  watcher:
    read: [ticket]
  pubreader:
    read: [page@published]
  staff:
    read: [ticket, page]
    visible:
      ticket:
        - field: title
        - field: status
      page:
        - field: title
  auditor:
    read: [ticket]
    visible:
      ticket:
        - field: secret
          when: "entity.status == 'done'"
  owner:
    read: [ticket, page]
    visible:
      ticket:
        - field: title
        - field: code
          when: "entity.status == 'open'"
      page:
        - field: title
        - field: code
          when: "entity.status == 'open'"
        - field: secret
          when: "related(entity, 'belongs-to')"
assignments:
  alice: staff
  bob: staff
  rita: reader
  pia: pubreader
  audit-team: auditor
role_relations:
  owner-of:
    confers: owner
  watches:
    confers: watcher
inherit_roles_through:
  - belongs-to
`

func newFieldWorld(t *testing.T, policyYAML string) fieldWorld {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title":    {Type: metamodel.PropertyTypeString},
				"status":   {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "done"}},
				"assignee": {Type: metamodel.PropertyTypeString},
				"secret":   {Type: metamodel.PropertyTypeString},
				"code":     {Type: metamodel.PropertyTypeString},
			}},
			"page": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString},
					"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "done"}},
					"secret": {Type: metamodel.PropertyTypeString},
					"code":   {Type: metamodel.PropertyTypeString},
				},
				Faces: map[string]metamodel.FaceDef{"draft": {}, "published": {}},
			},
			"project": {Properties: map[string]metamodel.PropertyDef{}},
			"person":  {Properties: map[string]metamodel.PropertyDef{}},
			"team":    {Properties: map[string]metamodel.PropertyDef{}},
		},
		Relations: map[string]metamodel.RelationDef{
			"owner-of":   {From: []string{"person"}, To: []string{"ticket", "project", "page"}},
			"belongs-to": {From: []string{"ticket", "page"}, To: []string{"project"}},
			"member-of":  {From: []string{"person"}, To: []string{"team"}},
			"watches":    {From: []string{"person"}, To: []string{"ticket"}},
		},
	}
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	ms := memstore.New()
	for _, p := range []string{"alice", "bob", "carol", "dave", "rita", "pia"} {
		mustCreateEntity(t, ms, p, "person")
	}
	mustCreateEntity(t, ms, "audit-team", "team")
	mustCreateEntity(t, ms, "PRJ-1", "project")
	tickets := map[string]*entity.Entity{}
	for _, tc := range []struct{ id, status string }{
		{"T-1", "open"}, {"T-2", "done"}, {"T-3", "open"}, {"T-4", "open"},
	} {
		e := entity.New(tc.id, "ticket")
		e.SetString("title", "title "+tc.id)
		e.SetString("status", tc.status)
		e.SetString("assignee", "someone")
		if tc.id == "T-3" {
			e.SetString("assignee", "bob")
		}
		e.SetString("secret", "secret "+tc.id)
		e.SetString("code", "code "+tc.id)
		e.SetString("extra", "undeclared "+tc.id)
		mustCreate(t, ms, e)
		tickets[tc.id] = e
	}
	mustCreateRelation(t, ms, "bob", "member-of", "audit-team")
	mustCreateRelation(t, ms, "carol", "owner-of", "T-1")
	mustCreateRelation(t, ms, "dave", "owner-of", "T-2")
	mustCreateRelation(t, ms, "carol", "owner-of", "PRJ-1")
	mustCreateRelation(t, ms, "T-4", "belongs-to", "PRJ-1")
	mustCreateRelation(t, ms, "rita", "owner-of", "T-1")
	mustCreateRelation(t, ms, "alice", "watches", "T-2")

	// P-1: carol owns it directly; its faces differ in status. P-2: carol
	// owns it through PRJ-1. P-3: nobody owns it.
	var pages []*entity.Entity
	for _, pc := range []struct {
		id     string
		face   entity.Face
		status string
	}{
		{"P-1", "draft", "open"}, {"P-1", "published", "done"},
		{"P-2", "draft", "open"}, {"P-2", "published", "open"},
		{"P-3", "published", "open"},
	} {
		e := entity.New(pc.id, "page")
		e.Face = pc.face
		e.SetString("title", "page "+pc.id)
		e.SetString("status", pc.status)
		e.SetString("secret", "secret "+pc.id)
		e.SetString("code", "code "+pc.id)
		mustCreate(t, ms, e)
		pages = append(pages, e)
	}
	mustCreateRelation(t, ms, "carol", "owner-of", "P-1")
	mustCreateRelation(t, ms, "pia", "owner-of", "P-1")
	mustCreateRelation(t, ms, "P-2", "belongs-to", "PRJ-1")

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
	return fieldWorld{meta: meta, ms: ms, decl: decl, resolver: resolver, tickets: tickets, pages: pages}
}

func (w fieldWorld) ctxFor(t *testing.T, user string) context.Context {
	t.Helper()
	return acltest.Bind(principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}), t, w.decl)
}

// hiddenFields lists the properties FieldVerdicts hides, sorted.
func hiddenFields(v affordances.FieldVerdicts) []string {
	var out []string
	for f, ok := range v.Visible {
		if !ok {
			out = append(out, f)
		}
	}
	slices.Sort(out)
	return out
}

// TestFieldVerdicts_VisibleCharacterization pins the `visible:` verdicts the
// search field scope must reproduce: the union of the roles that apply to the
// row (BUG-Y5UF3K). If no role on the row declares `visible:` for the type,
// every field is visible. Otherwise a field is visible when a declaring role
// lists it (each grant gated by its own `when:`) or a role without a block
// reads the row's type and face.
func TestFieldVerdicts_VisibleCharacterization(t *testing.T) {
	t.Parallel()
	w := newFieldWorld(t, fieldWorldPolicy)
	cases := []struct {
		user, ticket string
		want         []string
	}{
		// staff alone: closed world, title and status only.
		{"alice", "T-1", []string{"assignee", "code", "secret"}},
		// staff ∪ watcher on T-2: watcher declares no block and reads tickets.
		{"alice", "T-2", nil},
		// staff ∪ auditor: secret joins on a done ticket only.
		{"bob", "T-1", []string{"assignee", "code", "secret"}},
		{"bob", "T-2", []string{"assignee", "code"}},
		// owner on T-1 (open): title and code.
		{"carol", "T-1", []string{"assignee", "secret", "status"}},
		// no role declares on T-3 for carol: open world.
		{"carol", "T-3", nil},
		// owner inherited through belongs-to PRJ-1.
		{"carol", "T-4", []string{"assignee", "secret", "status"}},
		// owner on a done ticket: code's condition fails.
		{"dave", "T-2", []string{"assignee", "code", "secret", "status"}},
		{"dave", "T-1", nil},
		// a principal with only the everyone role: no role on the row declares.
		{"erin", "T-1", nil},
		// reader (global, no block) ∪ owner conferred on T-1: every field.
		// Gaining owner does not narrow what reader shows.
		{"rita", "T-1", nil},
		{"rita", "T-3", nil},
	}
	for _, tc := range cases {
		t.Run(tc.user+"/"+tc.ticket, func(t *testing.T) {
			t.Parallel()
			got := hiddenFields(w.resolver.FieldVerdicts(w.ctxFor(t, tc.user), w.tickets[tc.ticket]))
			if !slices.Equal(got, tc.want) {
				t.Errorf("hidden = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFieldVerdicts_FacedCharacterization pins the page rows, so the
// differential below compares something. Each face is judged on its own
// properties: P-1's published face is done, so code's condition fails there
// only. P-2's owner role is inherited through PRJ-1. A related() condition
// never holds on a named face, so secret stays hidden on every page for
// carol. pia also owns P-1, and her pubreader role reads published pages
// without a block, so P-1's published face shows every field to her.
func TestFieldVerdicts_FacedCharacterization(t *testing.T) {
	t.Parallel()
	w := newFieldWorld(t, fieldWorldPolicy)
	want := map[string]map[string][]string{
		"carol": {
			"P-1@draft":     {"secret", "status"},
			"P-1@published": {"code", "secret", "status"},
			"P-2@draft":     {"secret", "status"},
			"P-2@published": {"secret", "status"},
			"P-3@published": nil,
		},
		"pia": {
			"P-1@draft":     {"secret", "status"},
			"P-1@published": nil,
			"P-2@draft":     nil,
			"P-2@published": nil,
			"P-3@published": nil,
		},
	}
	for user, rows := range want {
		ctx := w.ctxFor(t, user)
		for _, row := range w.pages {
			key := row.ID + "@" + row.Face.String()
			if got := hiddenFields(w.resolver.FieldVerdicts(ctx, row)); !slices.Equal(got, rows[key]) {
				t.Errorf("%s %s: hidden = %v, want %v", user, key, got, rows[key])
			}
		}
	}
}

// ctxVia binds user arriving through a client of principalType, so a
// client baseline (a field ceiling) applies.
func (w fieldWorld) ctxVia(t *testing.T, user, principalType string) context.Context {
	t.Helper()
	p := principal.VerifiedFrom(user, principal.ToolDataEntry, principal.Claims{PrincipalType: principalType})
	req, err := w.decl.ForIdentity(acltest.IdentityFor(t.Context(), t, w.decl, p))
	if err != nil {
		t.Fatalf("ForIdentity: %v", err)
	}
	return acl.WithRequest(principal.With(context.Background(), p), req)
}

// scopeHidden evaluates scope against row the way a search backend does: a
// gated property is visible when a grant lists it and the grant's When
// matches the row's own face. The memstore answers each When.
func scopeHidden(t *testing.T, w fieldWorld, scope *search.FieldScope, row *entity.Entity) []string {
	t.Helper()
	if scope == nil {
		return nil
	}
	var out []string
	for _, f := range scope.Gated {
		visible := false
		for _, g := range scope.Grants {
			if !slices.Contains(g.Fields, f) {
				continue
			}
			if g.When == nil {
				visible = true
				break
			}
			q := *g.When
			q.Faces = store.AllFaces()
			m, err := w.ms.MatchingFaces(t.Context(), q, []string{row.ID})
			if err != nil {
				t.Fatalf("MatchingFaces: %v", err)
			}
			if slices.Contains(m[row.ID], row.Face) {
				visible = true
				break
			}
		}
		if !visible {
			out = append(out, f)
		}
	}
	return out
}

// TestSearchFieldScope_AgreesWithFieldVerdicts is the differential the search
// path rests on: for every principal and row, the hidden set the store
// computes from [affordances.PolicyResolver.SearchFieldScope] equals the one
// FieldVerdicts computes per row. It covers global roles, a role union
// through a group, local roles direct and inherited, the open rows of a
// local-only declaration, both client ceiling spellings, and the faces of
// a faced type.
func TestSearchFieldScope_AgreesWithFieldVerdicts(t *testing.T) {
	t.Parallel()
	// The differential adds a current_user grant and a related() grant to
	// the characterized policy.
	policy := strings.Replace(fieldWorldPolicy, `          when: "entity.status == 'done'"`,
		`          when: "entity.status == 'done'"
        - field: assignee
          when: "is_current_user(entity.assignee)"`, 1)
	policy = strings.Replace(policy, `          when: "entity.status == 'open'"`,
		`          when: "entity.status == 'open'"
        - field: secret
          when: "related(entity, 'belongs-to')"`, 1)
	if strings.Count(policy, "\n") != strings.Count(fieldWorldPolicy, "\n")+4 {
		t.Fatal("the differential policy did not take both extra grants")
	}
	w := newFieldWorld(t, policy+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      ticket: [title, nonexistent]
  bots:
    applies_to: [bot]
    visible:
      ticket: [status, code, phantom]
`)
	type ctxCase struct {
		name string
		ctx  func(t *testing.T) context.Context
	}
	var ctxs []ctxCase
	for _, u := range []string{"alice", "bob", "carol", "dave", "erin", "rita", "pia"} {
		ctxs = append(ctxs,
			ctxCase{u, func(t *testing.T) context.Context { t.Helper(); return w.ctxFor(t, u) }},
			ctxCase{u + "@app", func(t *testing.T) context.Context { t.Helper(); return w.ctxVia(t, u, "app") }},
			ctxCase{u + "@bot", func(t *testing.T) context.Context { t.Helper(); return w.ctxVia(t, u, "bot") }},
		)
	}
	for _, c := range ctxs {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ctx := c.ctx(t)
			scope, err := w.resolver.SearchFieldScope(ctx, "ticket")
			if err != nil {
				t.Fatalf("SearchFieldScope: %v", err)
			}
			for _, id := range []string{"T-1", "T-2", "T-3", "T-4"} {
				row := w.tickets[id]
				want := hiddenFields(w.resolver.FieldVerdicts(ctx, row))
				got := scopeHidden(t, w, scope, row)
				if !slices.Equal(got, want) {
					t.Errorf("%s: scope hides %v, FieldVerdicts hides %v", id, got, want)
				}
			}
			pageScope, err := w.resolver.SearchFieldScope(ctx, "page")
			if err != nil {
				t.Fatalf("SearchFieldScope(page): %v", err)
			}
			for _, row := range w.pages {
				want := hiddenFields(w.resolver.FieldVerdicts(ctx, row))
				got := scopeHidden(t, w, pageScope, row)
				if !slices.Equal(got, want) {
					t.Errorf("%s@%s: scope hides %v, FieldVerdicts hides %v", row.ID, row.Face, got, want)
				}
			}
		})
	}
}

// TestSearchFieldScope_NoRequestFailsClosed: without a bound request there is
// no scope, only an error.
func TestSearchFieldScope_NoRequestFailsClosed(t *testing.T) {
	t.Parallel()
	w := newFieldWorld(t, fieldWorldPolicy)
	if _, err := w.resolver.SearchFieldScope(ctxAs("alice"), "ticket"); err == nil {
		t.Fatal("SearchFieldScope without a request: want error")
	}
}

// TestNew_VisibleWhenMustLower pins the load-time rule: a `visible:` `when:`
// must lower exactly to a store query, or the policy fails to load with an
// error naming the role, type and field. Other blocks keep the full
// predicate vocabulary.
func TestNew_VisibleWhenMustLower(t *testing.T) {
	t.Parallel()
	cases := []struct {
		when string
		ok   bool
	}{
		{"entity.status == 'open'", true},
		{"entity.assignee == current_user.id", true},
		{"is_current_user(entity.assignee)", true},
		{"related(entity, 'blocks')", true},
		{"entity.status == 'open' and related(entity, 'implements')", true},
		{"has_role(current_user, entity, 'owner')", false},
		{"has_global_role(current_user, 'admin')", false},
		{"has_relation(entity, 'blocks')", false},
		{"count_relations(entity, 'blocks') > 0", false},
		{"entity.status == 'open' or entity.status == 'done'", false},
		{"not (entity.status == 'open')", false},
		{"entity.status ~= 'open'", false},
		{"has_current_user(entity.tags)", false},
	}
	for _, tc := range cases {
		t.Run(tc.when, func(t *testing.T) {
			t.Parallel()
			p := policyFromYAML(t, `
roles:
  triager:
    visible:
      ticket:
        - field: assignee
          when: `+quoteYAML(tc.when)+`
`)
			_, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p),
				affordances.WithTraversals(&stubBinder{}))
			if tc.ok {
				if err != nil {
					t.Fatalf("New: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("New: want a load error")
			}
			for _, part := range []string{"roles.triager.visible.ticket[0]", `field "assignee"`, "has_current_user"} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("error %q does not name %s", err, part)
				}
			}
		})
	}

	t.Run("other blocks keep host functions", func(t *testing.T) {
		t.Parallel()
		p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: assignee
          when: "has_role(current_user, entity, 'triager')"
`)
		if _, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p)); err != nil {
			t.Fatalf("New: %v", err)
		}
	})
}
