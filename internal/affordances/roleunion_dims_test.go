package affordances_test

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// unionMeta declares a faceless ticket with a `cites` relation carrying meta,
// a policy type with a draft and a published face, and the relations that
// confer roles in the monotonicity test.
func unionMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString},
					"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "review", "done"}},
					"secret": {Type: metamodel.PropertyTypeString},
				},
			},
			"policy": {
				Properties: map[string]metamodel.PropertyDef{
					"body":  {Type: metamodel.PropertyTypeString},
					"owner": {Type: metamodel.PropertyTypeString},
				},
				Faces: map[string]metamodel.FaceDef{"draft": {}, "published": {}},
			},
			"person": {Properties: map[string]metamodel.PropertyDef{}},
			"team":   {Properties: map[string]metamodel.PropertyDef{}},
		},
		Relations: map[string]metamodel.RelationDef{
			"member-of": {From: []string{"person"}, To: []string{"team"}},
			"watches":   {From: []string{"person"}, To: []string{"ticket", "policy"}},
			"edits":     {From: []string{"person"}, To: []string{"ticket", "policy"}},
			"reviews":   {From: []string{"person"}, To: []string{"ticket", "policy"}},
			"drafts":    {From: []string{"person"}, To: []string{"ticket", "policy"}},
			"cites": {
				From: []string{"ticket"}, To: []string{"ticket"},
				Properties: map[string]metamodel.PropertyDef{
					"note":   {Type: metamodel.PropertyTypeString},
					"reason": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}
}

// unionPolicy restricts every dimension through `clerk` (held by the clerks
// team) and `policyclerk` (held by the policyclerks team). Each user holds one
// role directly and joins the team the test names.
const unionPolicy = `
roles:
  clerk:
    read: [ticket]
    create: [ticket]
    update: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
    fields:
      ticket:
        - field: title
    options:
      ticket:
        - field: status
          option: open
    relations:
      ticket:
        - relation: cites
          visible:
            - field: note
  creator:
    read: [ticket]
    create: [ticket]
  updater:
    read: [ticket, policy]
    update: [ticket]
  policyclerk:
    read: [policy]
    visible:
      policy:
        - field: body
  pubreader:
    read: ["policy@published"]
client_baselines:
  apps:
    applies_to: [app]
    deny_update: [ticket]
    visible:
      ticket: [title]
assignments:
  clerks: clerk
  policyclerks: policyclerk
  carl: clerk
  cora: creator
  ulla: updater
  pia: pubreader
`

// unionWorld is the resolver over unionPolicy plus the rows the tests ask
// about.
type unionWorld struct {
	resolver *affordances.PolicyResolver
	decl     *acl.Declarative
}

func newUnionWorld(t *testing.T, policyYAML string) unionWorld {
	t.Helper()
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	ms := memstore.New()
	for _, team := range []string{"clerks", "policyclerks"} {
		mustCreateEntity(t, ms, team, "team")
	}
	for _, user := range []string{"carl", "cora", "ulla", "pia"} {
		mustCreateEntity(t, ms, user, "person")
	}
	mustCreateRelation(t, ms, "cora", "member-of", "clerks")
	mustCreateRelation(t, ms, "ulla", "member-of", "clerks")
	mustCreateRelation(t, ms, "pia", "member-of", "policyclerks")
	decl, err := acl.NewDeclarative(policy, aclstore.NewGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(unionMeta(), storeRelationLookup{ms}, decl)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	return unionWorld{resolver: resolver, decl: decl}
}

// clientCtx is a request context for user acting through a client of
// principalType, with the acl.Request attached the way the middleware does.
func (w unionWorld) clientCtx(t *testing.T, user, principalType string) context.Context {
	t.Helper()
	p := principal.VerifiedFrom(user, principal.ToolDataEntry, principal.Claims{PrincipalType: principalType})
	req, err := w.decl.ForPrincipal(p)
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	return acl.WithRequest(principal.With(context.Background(), p), req)
}

func unionTicket() *entity.Entity {
	e := entity.New("TKT-1", "ticket")
	e.SetString("status", "open")
	return e
}

func unionPolicyRow(face entity.Face) *entity.Entity {
	e := entity.New("POL-1", "policy")
	e.Face = face
	return e
}

// denied returns the keys a sparse verdict map marks false, sorted.
func denied(m map[string]bool) []string {
	var out []string
	for k, v := range m {
		if !v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// deniedOptionPaths returns "field=option" for every option marked false.
func deniedOptionPaths(m map[string]map[string]bool) []string {
	var out []string
	for field, opts := range m {
		for opt, v := range opts {
			if !v {
				out = append(out, field+"="+opt)
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestRoleUnion_PerDimension pins each dimension's verb: read opens `visible:`,
// update opens `fields:` and `options:` on an existing row, create opens them
// on a create, and face-qualified grants and the client ceiling are respected.
func TestRoleUnion_PerDimension(t *testing.T) {
	t.Parallel()
	w := newUnionWorld(t, unionPolicy)
	user := func(name string) func(*testing.T) context.Context {
		return func(*testing.T) context.Context { return userCtx(name) }
	}
	app := func(name string) func(*testing.T) context.Context {
		return func(t *testing.T) context.Context {
			t.Helper()
			return w.clientCtx(t, name, "app")
		}
	}
	update := w.resolver.FieldVerdicts
	create := w.resolver.CreateFieldVerdicts

	tests := []struct {
		name     string
		ctx      func(*testing.T) context.Context
		verdicts func(context.Context, *entity.Entity) affordances.FieldVerdicts
		row      *entity.Entity
		hidden   []string
		readOnly []string
		options  []string
	}{
		{
			name: "clerk alone keeps every restriction", ctx: user("carl"), verdicts: update, row: unionTicket(),
			hidden: []string{"secret"}, readOnly: []string{"secret", "status"},
			options: []string{"status=done", "status=review"},
		},
		{
			name: "update grant opens every dimension on update", ctx: user("ulla"), verdicts: update,
			row: unionTicket(),
		},
		{
			name: "update grant does not open writes on create", ctx: user("ulla"), verdicts: create,
			row: unionTicket(), readOnly: []string{"secret", "status"},
			options: []string{"status=done", "status=review"},
		},
		{
			name: "create grant does not open writes on update", ctx: user("cora"), verdicts: update,
			row: unionTicket(), readOnly: []string{"secret", "status"},
			options: []string{"status=done", "status=review"},
		},
		{
			name: "create grant opens every dimension on create", ctx: user("cora"), verdicts: create,
			row: unionTicket(),
		},
		{
			name: "face read grant opens its face", ctx: user("pia"), verdicts: update,
			row: unionPolicyRow("published"),
		},
		{
			name: "face read grant does not open another face", ctx: user("pia"), verdicts: update,
			row: unionPolicyRow("draft"), hidden: []string{"owner"},
		},
		{
			name: "ceiling still removes after the union", ctx: app("ulla"), verdicts: update,
			row: unionTicket(), hidden: []string{"secret", "status"}, readOnly: []string{"secret", "status"},
			options: []string{"status=done", "status=review"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fv := tc.verdicts(tc.ctx(t), tc.row)
			if got := denied(fv.Visible); !slices.Equal(got, tc.hidden) {
				t.Errorf("hidden = %v, want %v", got, tc.hidden)
			}
			if got := denied(fv.Writable); !slices.Equal(got, tc.readOnly) {
				t.Errorf("read-only = %v, want %v", got, tc.readOnly)
			}
			if got := deniedOptionPaths(fv.Options); !slices.Equal(got, tc.options) {
				t.Errorf("denied options = %v, want %v", got, tc.options)
			}
		})
	}
}

// TestRoleUnion_RelationMeta: a role without a meta `visible:` list for the
// relation shows every meta key when it reads the source row.
func TestRoleUnion_RelationMeta(t *testing.T) {
	t.Parallel()
	w := newUnionWorld(t, unionPolicy)
	keys := []string{"note", "reason"}
	tests := []struct {
		user   string
		hidden []string
	}{
		{"carl", []string{"reason"}},
		{"ulla", nil},
		{"cora", nil},
	}
	for _, tc := range tests {
		t.Run(tc.user, func(t *testing.T) {
			t.Parallel()
			got := denied(w.resolver.RelationFieldVerdicts(userCtx(tc.user), unionTicket(), "cites", keys))
			if !slices.Equal(got, tc.hidden) {
				t.Errorf("hidden meta = %v, want %v", got, tc.hidden)
			}
		})
	}
}

// TestRoleUnion_History pins the historical reading: a global role without a
// `visible:` block that reads the snapshot's type and face shows every field,
// a role conferred by the live graph is dropped, and a reader without such a
// role still fails closed.
func TestRoleUnion_History(t *testing.T) {
	t.Parallel()
	w := newUnionWorld(t, unionPolicy)
	conferred := newRoleUnionWorld(t, roleUnionPolicy)
	hist := func(user string) context.Context { return affordances.WithHistoricalSubject(userCtx(user)) }

	tests := []struct {
		name     string
		resolver *affordances.PolicyResolver
		user     string
		row      *entity.Entity
		hidden   []string
	}{
		{"global read role shows every field", w.resolver, "ulla", unionTicket(), nil},
		{"global block alone keeps its restriction", w.resolver, "carl", unionTicket(), []string{"secret"}},
		{"face read grant shows its face", w.resolver, "pia", unionPolicyRow("published"), nil},
		{"face read grant does not show another face", w.resolver, "pia", unionPolicyRow("draft"),
			[]string{"owner"}},
		{"face read grant does not show an unnamed face", w.resolver, "pia", unionPolicyRow(""),
			[]string{"owner"}},
		{"global wildcard read shows every field", conferred.resolver, "alice", conferred.watched, nil},
		{"global type read shows every field", conferred.resolver, "rita", conferred.watched, nil},
		{"conferred role is dropped and the reader fails closed", conferred.resolver, "vic", conferred.watched,
			[]string{"secret", "status", "title"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := denied(tc.resolver.FieldVerdicts(hist(tc.user), tc.row).Visible)
			if !slices.Equal(got, tc.hidden) {
				t.Errorf("historical hidden = %v, want %v", got, tc.hidden)
			}
		})
	}
}

// TestRoleUnion_Everyone covers the role every principal holds, including one
// with no identity: it joins the union like any other role.
func TestRoleUnion_Everyone(t *testing.T) {
	t.Parallel()
	const restricted = `
roles:
  everyone:
    read: [%s]
  clerk:
    read: [ticket]
    visible:
      ticket:
        - field: title
assignments:
  carl: clerk
`
	tests := []struct {
		name     string
		everyone string
		user     string
		hidden   []string
	}{
		{"everyone without ticket read adds nothing", "person", "carl", []string{"secret", "status"}},
		{"everyone with ticket read opens the block", "ticket", "carl", nil},
		{"an unidentified caller holds only everyone", "ticket", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := newUnionWorld(t, fmt.Sprintf(restricted, tc.everyone))
			ctx := context.Background() // no principal: only everyone applies
			if tc.user != "" {
				ctx = userCtx(tc.user)
			}
			got := denied(w.resolver.FieldVerdicts(ctx, unionTicket()).Visible)
			if !slices.Equal(got, tc.hidden) {
				t.Errorf("hidden = %v, want %v", got, tc.hidden)
			}
		})
	}
}

// monotonicRoles is the role pool the monotonicity test draws subsets from.
// Roles in monotonicConferred reach the principal through an edge from the
// user to the row, the way BUG-Y5UF3K was found; the rest through a team. The
// pool mixes blocks with and without `when:`, block-less roles holding each
// verb, `*` grants, face-limited grants, a relation meta block and a role
// holding nothing on either type. Every principal also holds everyone, which
// declares a block of its own.
var monotonicRoles = []string{"blockvis", "blockwrite", "reader", "writer", "maker", "pubreader", "drafter", "idle"}

// monotonicConferred maps each conferred role to the relation that confers it.
var monotonicConferred = map[string]string{
	"blockwrite": "watches",
	"writer":     "edits",
	"pubreader":  "reviews",
	"drafter":    "drafts",
}

const monotonicPolicy = `
role_relations:
  watches:
    confers: blockwrite
  edits:
    confers: writer
  reviews:
    confers: pubreader
  drafts:
    confers: drafter
roles:
  everyone:
    read: ["policy@published"]
    visible:
      policy:
        - field: owner
  blockvis:
    read: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
          when: "entity.status == 'open'"
    relations:
      ticket:
        - relation: cites
          visible:
            - field: note
  blockwrite:
    read: [ticket]
    update: [ticket]
    create: [ticket]
    fields:
      ticket:
        - field: title
        - field: secret
          when: "entity.status == 'review'"
    options:
      ticket:
        - field: status
          option: open
  reader:
    read: [ticket, policy]
  writer:
    read: ["*"]
    update: ["*"]
  maker:
    create: [ticket]
  pubreader:
    read: ["policy@published"]
  drafter:
    read: ["policy@draft"]
    update: ["policy@draft"]
    create: ["policy@draft"]
    fields:
      policy:
        - field: body
  idle:
    read: [person]
`

// monotonicCeiling is a client baseline for app principals that closes update
// on ticket and narrows ticket visibility.
const monotonicCeiling = `client_baselines:
  apps:
    applies_to: [app]
    deny_update: [ticket]
    visible:
      ticket: [title, status]
`

// monotonicGrant is one verb a role holds on one face of a type.
type monotonicGrant struct {
	verb       acl.Verb
	entityType string
	face       entity.Face
}

// monotonicGrants is the oracle: what each role in monotonicPolicy holds on
// the rows the test asks about, written out by hand. A `*` update covers the
// unnamed face only, so writer holds no update on a policy face.
var monotonicGrants = map[string][]monotonicGrant{
	acl.EveryoneRole: {{acl.VerbRead, "policy", "published"}},
	"blockvis":       {{acl.VerbRead, "ticket", ""}},
	"blockwrite":     {{acl.VerbRead, "ticket", ""}, {acl.VerbUpdate, "ticket", ""}, {acl.VerbCreate, "ticket", ""}},
	"reader":         {{acl.VerbRead, "ticket", ""}, {acl.VerbRead, "policy", "draft"}, {acl.VerbRead, "policy", "published"}},
	"writer": {
		{acl.VerbRead, "ticket", ""}, {acl.VerbRead, "policy", "draft"}, {acl.VerbRead, "policy", "published"},
		{acl.VerbUpdate, "ticket", ""},
	},
	"maker":     {{acl.VerbCreate, "ticket", ""}},
	"pubreader": {{acl.VerbRead, "policy", "published"}},
	"drafter":   {{acl.VerbRead, "policy", "draft"}, {acl.VerbUpdate, "policy", "draft"}, {acl.VerbCreate, "policy", "draft"}},
}

// monotonicOracle answers whether a principal holding the roles in mask (and
// everyone) holds verb on row. A historical reading drops conferred roles,
// and the app ceiling closes update on ticket for every role.
func monotonicOracle(mask int, verb acl.Verb, row *entity.Entity, app, history bool) bool {
	if app && verb == acl.VerbUpdate && row.Type == "ticket" {
		return false
	}
	want := monotonicGrant{verb, row.Type, row.Face}
	for _, role := range append(rolesOf(mask), acl.EveryoneRole) {
		if _, conferred := monotonicConferred[role]; conferred && history {
			continue
		}
		if slices.Contains(monotonicGrants[role], want) {
			return true
		}
	}
	return false
}

// monotonicRows are the rows the monotonicity test asks about: a ticket in
// each status and a policy on each face.
func monotonicRows() []*entity.Entity {
	rows := []*entity.Entity{unionPolicyRow("published"), unionPolicyRow("draft")}
	for _, status := range []string{"open", "review", "done"} {
		e := entity.New("TKT-"+status, "ticket")
		e.SetString("status", status)
		rows = append(rows, e)
	}
	return rows
}

// newMonotonicWorld builds the resolver over monotonicPolicy plus extraPolicy,
// with one user per entry of users holding the roles in its mask: global
// roles through the team g-<role>, conferred roles through an edge to every
// row of monotonicRows.
func newMonotonicWorld(tb testing.TB, extraPolicy string, users map[string]int) (*affordances.PolicyResolver, *acl.Declarative) {
	tb.Helper()
	var assignments strings.Builder
	for _, role := range monotonicRoles {
		if _, conferred := monotonicConferred[role]; !conferred {
			fmt.Fprintf(&assignments, "  g-%s: %s\n", role, role)
		}
	}
	policy, err := acl.LoadPolicyBytes([]byte(monotonicPolicy + extraPolicy + "assignments:\n" + assignments.String()))
	if err != nil {
		tb.Fatalf("LoadPolicyBytes: %v", err)
	}
	ctx := context.Background()
	ms := memstore.New()
	create := func(e *entity.Entity) {
		if createErr := ms.CreateEntity(ctx, e); createErr != nil {
			tb.Fatalf("create %s: %v", e.ID, createErr)
		}
	}
	link := func(from, relType, to string) {
		if _, linkErr := ms.CreateRelation(ctx, entity.RelationKey{From: from, Type: relType, To: to}, nil); linkErr != nil {
			tb.Fatalf("link %s --%s--> %s: %v", from, relType, to, linkErr)
		}
	}
	rowIDs := []string{}
	for _, row := range monotonicRows() {
		create(row)
		if !slices.Contains(rowIDs, row.ID) {
			rowIDs = append(rowIDs, row.ID)
		}
	}
	for _, role := range monotonicRoles {
		create(entity.New("g-"+role, "team"))
	}
	for user, mask := range users {
		create(entity.New(user, "person"))
		for _, role := range rolesOf(mask) {
			relType, conferred := monotonicConferred[role]
			if !conferred {
				link(user, "member-of", "g-"+role)
				continue
			}
			for _, id := range rowIDs {
				link(user, relType, id)
			}
		}
	}
	decl, err := acl.NewDeclarative(policy, aclstore.NewGraph(ms), ms)
	if err != nil {
		tb.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(unionMeta(), storeRelationLookup{ms}, decl)
	if err != nil {
		tb.Fatalf("affordances.New: %v", err)
	}
	return resolver, decl
}

// TestRoleUnion_Monotonic is MEAS-ROLE-UNION-MONOTONIC: for every pair of role
// sets S ⊆ T drawn from monotonicRoles, every field visible or writable, every
// option allowed and every relation meta key visible under S is still so
// under T. It runs every subset on rows in different states and faces, for
// update and create verdicts, live and historical, for a user and for an app
// client under a ceiling.
//
// It compares effective access, not the raw verdict maps. A field counts as
// writable only when the principal holds the write verb on the row, and as
// visible only when it reads the row, as monotonicOracle answers. Without
// that, a principal whose roles declare no block and hold no verb has every
// dimension open by default, which grants nothing because the type gate
// refuses the operation.
func TestRoleUnion_Monotonic(t *testing.T) {
	t.Parallel()
	subsets := 1 << len(monotonicRoles)
	users := make(map[string]int, subsets)
	for mask := range subsets {
		users[fmt.Sprintf("u%03d", mask)] = mask
	}
	r, decl := newMonotonicWorld(t, monotonicCeiling, users)
	world := unionWorld{resolver: r, decl: decl}
	meta := unionMeta()

	type mode struct {
		name      string
		app       bool
		history   bool
		create    bool
		writeVerb acl.Verb
	}
	var modes []mode
	for _, app := range []bool{false, true} {
		modes = append(modes,
			mode{name: "update", app: app, writeVerb: acl.VerbUpdate},
			mode{name: "create", app: app, create: true, writeVerb: acl.VerbCreate},
			mode{name: "history", app: app, history: true, writeVerb: acl.VerbUpdate})
	}
	for _, m := range modes {
		t.Run(fmt.Sprintf("%s/app=%t", m.name, m.app), func(t *testing.T) {
			t.Parallel()
			for _, row := range monotonicRows() {
				access := make([]map[string]bool, subsets)
				for mask := range subsets {
					user := fmt.Sprintf("u%03d", mask)
					ctx := userCtx(user)
					if m.app {
						ctx = world.clientCtx(t, user, "app")
					}
					if m.history {
						ctx = affordances.WithHistoricalSubject(ctx)
					}
					verdicts := r.FieldVerdicts
					if m.create {
						verdicts = r.CreateFieldVerdicts
					}
					reads := monotonicOracle(mask, acl.VerbRead, row, m.app, m.history)
					access[mask] = effectiveAccess(meta.Entities[row.Type], verdicts(ctx, row),
						reads, monotonicOracle(mask, m.writeVerb, row, m.app, m.history))
					if reads && row.Type == "ticket" && !m.history && !m.create {
						metaKeys := []string{"note", "reason"}
						metaVerdicts := r.RelationFieldVerdicts(ctx, row, "cites", metaKeys)
						for _, key := range metaKeys {
							if !isDenied(metaVerdicts, key) {
								access[mask]["meta:"+key] = true
							}
						}
					}
				}
				assertMonotonic(t, m.name, row, access)
			}
		})
	}
}

// assertMonotonic fails for every key some role set has that the same set
// plus one more role lacks.
func assertMonotonic(t *testing.T, modeName string, row *entity.Entity, access []map[string]bool) {
	t.Helper()
	for small := range access {
		for i, role := range monotonicRoles {
			bit := 1 << i
			if small&bit != 0 {
				continue
			}
			for path := range access[small] {
				if !access[small|bit][path] {
					t.Errorf("%s %s/%s: adding %s to %v removes %s",
						modeName, row.Type, row.Face, role, rolesOf(small), path)
				}
			}
		}
	}
}

// effectiveAccess is the set of "dimension:path" keys v grants on a row of
// def: every declared field (or enum option) not denied, in a dimension whose
// verb the principal holds.
func effectiveAccess(def metamodel.EntityDef, v affordances.FieldVerdicts, reads, writes bool) map[string]bool {
	out := map[string]bool{}
	for name, pd := range def.Properties {
		if reads && !isDenied(v.Visible, name) {
			out["visible:"+name] = true
		}
		if !writes {
			continue
		}
		if !isDenied(v.Writable, name) {
			out["writable:"+name] = true
		}
		for _, option := range pd.Values {
			if !isDenied(v.Options[name], option) {
				out["option:"+name+"="+option] = true
			}
		}
	}
	return out
}

func isDenied(m map[string]bool, key string) bool {
	allowed, present := m[key]
	return present && !allowed
}

func rolesOf(mask int) []string {
	var out []string
	for i, role := range monotonicRoles {
		if mask&(1<<i) != 0 {
			out = append(out, role)
		}
	}
	return out
}

// TestRoleUnion_WriteGateJudgesCreateByCreate: the write gate asks create
// verdicts on a create and update verdicts on an update, so a role that may
// create tickets writes every field on create and only the clerk's on update.
func TestRoleUnion_WriteGateJudgesCreateByCreate(t *testing.T) {
	t.Parallel()
	w := newUnionWorld(t, unionPolicy)
	gate, err := affordances.NewWriteGate(w.resolver)
	if err != nil {
		t.Fatalf("NewWriteGate: %v", err)
	}
	set := map[string]any{"secret": "s"}
	if err := gate.CheckCreateFieldWrite(userCtx("cora"), unionTicket(), set); err != nil {
		t.Errorf("create as creator and clerk: %v", err)
	}
	if err := gate.CheckFieldWrite(userCtx("cora"), unionTicket(), set, nil); err == nil {
		t.Error("update as creator and clerk: secret write allowed, want refused")
	}
	if err := gate.CheckCreateFieldWrite(userCtx("ulla"), unionTicket(), set); err == nil {
		t.Error("create as updater and clerk: secret write allowed, want refused")
	}
	if err := gate.CheckFieldWrite(userCtx("ulla"), unionTicket(), set, nil); err != nil {
		t.Errorf("update as updater and clerk: %v", err)
	}
}

// BenchmarkFieldVerdicts_RoleUnion measures the per-row cost of the verb
// question block-less roles add: a principal holding every role in the
// monotonicity pool, global and conferred, through an app client whose
// ceiling closes update on the row's type and narrows its visibility.
func BenchmarkFieldVerdicts_RoleUnion(b *testing.B) {
	all := 1<<len(monotonicRoles) - 1
	r, decl := newMonotonicWorld(b, monotonicCeiling, map[string]int{"bench": all})
	p := principal.VerifiedFrom("bench", principal.ToolDataEntry, principal.Claims{PrincipalType: "app"})
	req, err := decl.ForPrincipal(p)
	if err != nil {
		b.Fatal(err)
	}
	reqCtx := acl.WithRequest(principal.With(context.Background(), p), req)
	row := entity.New("TKT-review", "ticket")
	row.SetString("status", "review")

	b.ReportAllocs()
	for b.Loop() {
		_ = r.FieldVerdicts(reqCtx, row)
	}
}
