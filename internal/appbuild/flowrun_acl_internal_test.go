package appbuild

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestFlowRunACL_ThroughMetamodelView wires a policy against a real schema
// with flow runs through the production adapter (metamodelView). The run
// gate follows `_carries` to the subject, and only system:flow may write a
// run (TKT-BC5G8RP).
func TestFlowRunACL_ThroughMetamodelView(t *testing.T) {
	base, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties: { title: { type: string } }
  secret:
    label: Secret
    id_prefix: "SEC-"
    properties: { title: { type: string } }
relations: {}
`))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := metamodel.WithFlowRuns(base)
	if err != nil {
		t.Fatal(err)
	}
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor: { create: ["*"], read: [ticket] }
assignments:
  alice: editor
`))
	if err != nil {
		t.Fatal(err)
	}
	if err = p.ValidateAgainstMetamodel(metamodelView{meta}); err != nil {
		t.Fatalf("ValidateAgainstMetamodel: %v", err)
	}

	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		entity.New("TKT-1", "ticket"), entity.New("SEC-1", "secret"),
		entity.New("RUN-1", entity.FlowRunType), entity.New("RUN-2", entity.FlowRunType),
	} {
		if err = st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "RUN-1", Type: entity.CarriesRelation, To: "TKT-1"},
		{From: "RUN-2", Type: entity.CarriesRelation, To: "SEC-1"},
	} {
		if _, err = st.CreateRelation(ctx, k, nil); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}

	alice := acltest.RequestFor(ctx, t, d, principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	v, err := alice.ReadableFacesMany(ctx, entity.FlowRunType, []string{"RUN-1", "RUN-2"})
	if err != nil {
		t.Fatal(err)
	}
	if v.For("RUN-1").None() || !v.For("RUN-2").None() {
		t.Errorf("alice reads RUN-1 %t, RUN-2 %t; want only RUN-1", !v.For("RUN-1").None(), !v.For("RUN-2").None())
	}

	run := acl.NewFacelessEntitySubject(entity.FlowRunType, "RUN-1")
	if alice.AuthorizeWrite(ctx, acl.WriteRequest{Op: acl.OpCreate, Subject: run}).Allow {
		t.Error(`alice's create: ["*"] covers a flow run`)
	}
	engine := acltest.RequestFor(ctx, t, d, principal.Principal{User: principal.UserFlow, Tool: principal.ToolFlow})
	if !engine.AuthorizeWrite(ctx, acl.WriteRequest{Op: acl.OpCreate, Subject: run}).Allow {
		t.Error("system:flow may not create a flow run")
	}
}
