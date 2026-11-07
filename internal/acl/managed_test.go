package acl_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// flowMeta declares the flow run type the way metamodel.WithFlowRuns does:
// `_carries` and `_produced` from the run to every faceless operator type.
func flowMeta() fakeMeta {
	targets := []string{"person", "project", "ticket"}
	return fakeMeta{
		types: map[string]map[string]acl.PropertyInfo{
			"person": {}, "project": {}, "ticket": {}, entity.FlowRunType: {},
		},
		relations: []string{"owns"},
		relInfo: map[string]acl.RelationInfo{
			entity.CarriesRelation:  {From: []string{entity.FlowRunType}, To: targets},
			entity.ProducedRelation: {From: []string{entity.FlowRunType}, To: targets},
		},
	}
}

const flowPolicy = `
roles:
  admin: { create: ["*"], update: ["*"], delete: ["*"], rename: ["*"], convert: ["*"], read: ["*"] }
  owner: { read: [ticket] }
  projects: { read: [project] }
role_relations:
  owns: { confers: owner }
assignments:
  carol: admin
  dave: projects
`

type flowFixture struct {
	d     *acl.Declarative
	store store.Store
}

// newFlowFixture seeds four runs: RUN-1 carries TKT-1 (alice owns it),
// RUN-2 carries TKT-2, RUN-3 carries PRJ-1, RUN-4 carries nothing.
func newFlowFixture(t *testing.T, policyYAML string) flowFixture {
	t.Helper()
	ctx := context.Background()
	p, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	if err = p.ValidateAgainstMetamodel(flowMeta()); err != nil {
		t.Fatalf("ValidateAgainstMetamodel: %v", err)
	}
	st := memstore.New()
	for _, e := range []*entity.Entity{
		entity.New("alice", "person"), entity.New("bob", "person"),
		entity.New("TKT-1", "ticket"), entity.New("TKT-2", "ticket"), entity.New("PRJ-1", "project"),
		entity.New("RUN-1", entity.FlowRunType), entity.New("RUN-2", entity.FlowRunType),
		entity.New("RUN-3", entity.FlowRunType), entity.New("RUN-4", entity.FlowRunType),
	} {
		if err = st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "alice", Type: "owns", To: "TKT-1"},
		{From: "RUN-1", Type: entity.CarriesRelation, To: "TKT-1"},
		{From: "RUN-2", Type: entity.CarriesRelation, To: "TKT-2"},
		{From: "RUN-3", Type: entity.CarriesRelation, To: "PRJ-1"},
	} {
		if _, err = st.CreateRelation(ctx, k, nil); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return flowFixture{d: d, store: st}
}

func (f flowFixture) request(t *testing.T, user string) *acl.Request {
	t.Helper()
	tool := principal.ToolDataEntry
	if user == principal.UserFlow {
		tool = principal.ToolFlow
	}
	req, err := f.d.ForIdentity(acltest.IdentityFor(t.Context(), t, f.d, principal.Principal{User: user, Tool: tool}))
	if err != nil {
		t.Fatalf("ForIdentity(%q): %v", user, err)
	}
	return req
}

func (f flowFixture) visibleRuns(t *testing.T, user string) []string {
	t.Helper()
	ctx := t.Context()
	rq := f.request(t, user).ReadQuery(ctx, entity.FlowRunType)
	var ids []string
	switch {
	case rq.DenyAll:
	case rq.AllowAll:
		for e, err := range f.store.ListEntities(ctx, store.EntityQuery{Type: entity.FlowRunType, Faces: store.InWorld(store.TrivialScope())}) {
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, e.ID)
		}
	default:
		q := *rq.Query
		q.Faces = store.InWorld(store.TrivialScope())
		for e, err := range f.store.GraphQuery(ctx, q) {
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, e.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

// TestFlowRunReadQuery pins AC4: a reader sees a run only when it can see
// the run's subject, through `_carries`.
func TestFlowRunReadQuery(t *testing.T) {
	f := newFlowFixture(t, flowPolicy)
	cases := []struct {
		user string
		want []string
	}{
		// read: ["*"] reads every subject, and the run without a subject
		// stays hidden.
		{"carol", []string{"RUN-1", "RUN-2", "RUN-3"}},
		// A role relation on TKT-1 reads that ticket's run only.
		{"alice", []string{"RUN-1"}},
		// A global read on projects alone: the ticket branch is dropped.
		{"dave", []string{"RUN-3"}},
		{"bob", nil},
		// The engine names the type in its read grant.
		{principal.UserFlow, []string{"RUN-1", "RUN-2", "RUN-3", "RUN-4"}},
	}
	for _, tc := range cases {
		t.Run(tc.user, func(t *testing.T) {
			if got := f.visibleRuns(t, tc.user); !slices.Equal(got, tc.want) {
				t.Fatalf("visible runs = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("single run is a uniform not found", func(t *testing.T) {
		ok, err := f.request(t, "alice").PermitsRead(t.Context(), entity.FlowRunType, "RUN-2")
		if err != nil || ok {
			t.Fatalf("PermitsRead(RUN-2) = %v, %v; want false", ok, err)
		}
		ok, err = f.request(t, "alice").PermitsRead(t.Context(), entity.FlowRunType, "RUN-1")
		if err != nil || !ok {
			t.Fatalf("PermitsRead(RUN-1) = %v, %v; want true", ok, err)
		}
	})

	// The row gate of every read-out path (visibility.PolicyReader) asks
	// per id through ReadableFacesMany.
	t.Run("row gate per id", func(t *testing.T) {
		v, err := f.request(t, "alice").ReadableFacesMany(t.Context(), entity.FlowRunType,
			[]string{"RUN-1", "RUN-2", "RUN-3", "RUN-4"})
		if err != nil {
			t.Fatal(err)
		}
		for id, want := range map[string]bool{"RUN-1": true, "RUN-2": false, "RUN-3": false, "RUN-4": false} {
			if got := !v.For(id).None(); got != want {
				t.Errorf("%s readable = %t, want %t", id, got, want)
			}
		}
	})
}

// A client ceiling narrows the subjects, and with them the runs; the
// run type itself needs no ceiling entry (RR-1SYZD5J).
func TestFlowRunReadQuery_CeilingAppliesPerSubject(t *testing.T) {
	f := newFlowFixture(t, flowPolicy+`
client_baselines:
  apps:
    applies_to: [app]
    read: [project]
`)
	req, err := f.d.ForIdentity(acltest.IdentityFor(t.Context(), t, f.d,
		principal.VerifiedFrom("carol", principal.ToolDataEntry, principal.Claims{PrincipalType: "app"})))
	if err != nil {
		t.Fatal(err)
	}
	rq := req.ReadQuery(t.Context(), entity.FlowRunType)
	if rq.DenyAll || rq.AllowAll || rq.Query == nil {
		t.Fatalf("ReadQuery = %+v, want a query", rq)
	}
	q := *rq.Query
	q.Faces = store.InWorld(store.TrivialScope())
	var ids []string
	for e, err := range f.store.GraphQuery(t.Context(), q) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID)
	}
	if !slices.Equal(ids, []string{"RUN-3"}) {
		t.Fatalf("runs under the ceiling = %v, want [RUN-3]", ids)
	}
}

// TestFlowRunWrite_OnlyEngine pins AC3: "*" grants nothing on the managed
// type or its relations; the engine role does.
func TestFlowRunWrite_OnlyEngine(t *testing.T) {
	f := newFlowFixture(t, flowPolicy)
	run := acl.NewFacelessEntitySubject(entity.FlowRunType, "RUN-1")
	subjects := []struct {
		name string
		op   acl.Op
		subj acl.Subject
	}{
		{"create", acl.OpCreate, run},
		{"update", acl.OpUpdate, run},
		{"delete", acl.OpDelete, run},
		{"rename", acl.OpRename, run},
		{"carries edge", acl.OpCreate, acl.RelationSubject{
			Type: entity.CarriesRelation, FromType: entity.FlowRunType, FromID: "RUN-1",
		}},
		{"produced edge", acl.OpDelete, acl.RelationSubject{
			Type: entity.ProducedRelation, FromType: entity.FlowRunType, FromID: "RUN-1",
		}},
	}
	for _, s := range subjects {
		t.Run(s.name, func(t *testing.T) {
			d := f.request(t, "carol").AuthorizeWrite(t.Context(), acl.WriteRequest{Op: s.op, Subject: s.subj})
			if d.Allow {
				t.Errorf("admin with \"*\" allowed %s", s.name)
			}
			if s.op == acl.OpRename {
				return
			}
			d = f.request(t, principal.UserFlow).AuthorizeWrite(t.Context(), acl.WriteRequest{Op: s.op, Subject: s.subj})
			if !d.Allow {
				t.Errorf("engine denied %s: %s", s.name, d.Reason)
			}
		})
	}

	t.Run("wildcard still covers operator types", func(t *testing.T) {
		d := f.request(t, "carol").AuthorizeWrite(t.Context(), acl.WriteRequest{
			Op: acl.OpUpdate, Subject: acl.NewFacelessEntitySubject("ticket", "TKT-1"),
		})
		if !d.Allow {
			t.Fatalf("admin denied a ticket update: %s", d.Reason)
		}
	})

	t.Run("grant helpers agree", func(t *testing.T) {
		admin := acl.RoleDef{Update: []string{"*"}, Read: []string{"*"}}
		if acl.GrantsVerb(admin, acl.OpUpdate, entity.FlowRunType) || acl.GrantsRead(admin, entity.FlowRunType) {
			t.Fatal("\"*\" covers the managed type")
		}
	})
}

// TestValidate_ReservedNames pins AC2: acl.yaml may not name a managed
// type, relation or role, nor assign the engine identity.
func TestValidate_ReservedNames(t *testing.T) {
	cases := map[string]string{
		"role grants run":         "roles: { r: { read: [_flow-run] } }",
		"role grants run face":    "roles: { r: { update: [\"_flow-run@x\"] } }",
		"reserved role name":      "roles: { _flow-engine: { read: [ticket] } }",
		"field block":             "roles: { r: { read: [ticket], fields: { _flow-run: [{field: status}] } } }",
		"relation grant":          "roles: { r: { read: [ticket], relations: { ticket: [{relation: _carries}] } } }",
		"assign engine":           "roles: { r: { read: [ticket] } }\nassignments: { \"system:flow\": r }",
		"assert engine":           "roles: { r: { read: [ticket] } }\nasserted_role_assignments: { \"system:flow\": [r] }",
		"assign managed role":     "roles: { r: { read: [ticket] } }\nassignments: { alice: _flow-engine }",
		"role relation":           "roles: { r: { read: [ticket] } }\nrole_relations: { _carries: { confers: r } }",
		"inherit through":         "roles: { r: { read: [ticket] } }\ninherit_roles_through: [_carries]",
		"relation_grants key":     "relation_grants: { _carries: { permission: x } }",
		"ceiling allows run":      "roles: { r: { read: [\"*\"] } }\nclient_baselines: { a: { applies_to: [app], read: [_flow-run] } }",
		"membership relation":     "membership_relation: _carries",
		"user entity type":        "user_entity_type: _flow-run",
		"namespaced managed type": "roles: { r: { read: [isms._run] } }",
	}
	for name, yaml := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := acl.LoadPolicyBytes([]byte(yaml))
			if err == nil {
				t.Fatal("policy loaded, want a refusal")
			}
			if !strings.Contains(err.Error(), "reserved for rela") && !strings.Contains(err.Error(), "assigns this identity") {
				t.Fatalf("err = %v, want a reserved-name refusal", err)
			}
		})
	}
}

// The injection is idempotent and survives a second Validate.
func TestInjectFlowEngine_Idempotent(t *testing.T) {
	p, err := acl.LoadPolicyBytes([]byte(flowPolicy))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := p.ValidateAgainstMetamodel(flowMeta()); err != nil {
			t.Fatalf("ValidateAgainstMetamodel: %v", err)
		}
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate after injection: %v", err)
	}
	if p.Assignments[principal.UserFlow] != acl.FlowEngineRole {
		t.Fatalf("engine assignment = %q", p.Assignments[principal.UserFlow])
	}
	if _, ok := p.Roles[acl.FlowEngineRole]; !ok {
		t.Fatal("engine role missing")
	}

	t.Run("no injection without the run type", func(t *testing.T) {
		q, err := acl.LoadPolicyBytes([]byte(flowPolicy))
		if err != nil {
			t.Fatal(err)
		}
		meta := flowMeta()
		delete(meta.types, entity.FlowRunType)
		if err := q.ValidateAgainstMetamodel(meta); err != nil {
			t.Fatal(err)
		}
		if _, ok := q.Roles[acl.FlowEngineRole]; ok {
			t.Fatal("engine role injected without the run type")
		}
	})
}

// who-can read on a run reports the routes by which the reader reads the
// run's subject: no operator role grants read on the run type itself.
func TestFlowRunAccessRoutes(t *testing.T) {
	f := newFlowFixture(t, flowPolicy)
	cases := []struct {
		user, run string
		want      []string
	}{
		{"alice", "RUN-1", []string{"owner"}},
		{"alice", "RUN-2", nil},
		{"carol", "RUN-1", []string{"admin"}},
		{"dave", "RUN-3", []string{"projects"}},
		{"carol", "RUN-4", nil},
	}
	for _, tc := range cases {
		t.Run(tc.user+"/"+tc.run, func(t *testing.T) {
			attrs, err := f.request(t, tc.user).AccessRoutes(t.Context(), acl.VerbRead, entity.FlowRunType, tc.run)
			if err != nil {
				t.Fatal(err)
			}
			var roles []string
			for _, a := range attrs {
				roles = append(roles, a.Role)
			}
			slices.Sort(roles)
			if !slices.Equal(roles, tc.want) {
				t.Errorf("roles = %v, want %v", roles, tc.want)
			}
		})
	}
}

// A stored `_carries` edge to a type outside the run's targets matches no
// branch, even for a reader of every target type: each branch keeps its
// endpoint type.
func TestFlowRunReadQuery_EdgeOutsideTargets(t *testing.T) {
	f := newFlowFixture(t, flowPolicy)
	if err := f.store.CreateEntity(t.Context(), entity.New("RUN-5", entity.FlowRunType)); err != nil {
		t.Fatal(err)
	}
	key := entity.RelationKey{From: "RUN-5", Type: entity.CarriesRelation, To: "RUN-1"}
	if _, err := f.store.CreateRelation(t.Context(), key, nil); err != nil {
		t.Fatal(err)
	}
	if got := f.visibleRuns(t, "carol"); !slices.Equal(got, []string{"RUN-1", "RUN-2", "RUN-3"}) {
		t.Fatalf("visible runs = %v, want RUN-1..3", got)
	}
}
