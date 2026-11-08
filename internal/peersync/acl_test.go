package peersync

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
)

// peerPolicy grants the peer's writer the mapped types when granted is
// true, and only reads otherwise.
func peerPolicy(granted bool) string {
	role := `    read: ["*"]
`
	if granted {
		role += `    create: [requirement, component]
    update: [requirement, component]
    delete: [requirement, component]
`
	}
	return "roles:\n  mirror:\n" + role + "assignments:\n  system:peer:cat: mirror\n"
}

// aclTarget is newTarget with the manager on a real acl.yaml policy and a
// memory audit sink, and ctx bound to the peer's writer as the scheduler
// binds it.
func aclTarget(t *testing.T, policy string) (*target, *audit.Memory, context.Context) {
	t.Helper()
	tg := newTarget(t)
	p, err := acl.LoadPolicyBytes([]byte(policy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(tg.st), tg.st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	copyVis, err := entitymanager.NewAllowAllCopyVisibility(tg.st)
	if err != nil {
		t.Fatalf("NewAllowAllCopyVisibility: %v", err)
	}
	sink := audit.NewMemory()
	tg.mgr, err = entitymanager.New(entitymanager.Deps{
		Store: tg.st, Meta: tg.meta, Templater: nopTemplater{}, Audit: sink, ACL: d,
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: copyVis,
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := principal.With(context.Background(), principal.Principal{
		User: principal.PeerPullUserPrefix + "cat", Tool: principal.ToolPeerPull,
	})
	ctx, err = acl.BindRequest(ctx, d)
	if err != nil {
		t.Fatalf("BindRequest: %v", err)
	}
	return tg, sink, ctx
}

func TestRun_LocalACLGatesThePeerWriter(t *testing.T) {
	t.Run("no grant writes nothing and audits the refusal", func(t *testing.T) {
		tg, sink, ctx := aclTarget(t, peerPolicy(false))
		rep, _ := Run(ctx, testMapping(t, tg.meta, nil), tg.deps(seedSource()), Options{})
		if rep.Entities.Created != 0 || rep.Relations.Created != 0 {
			t.Fatalf("created rows without a grant: %+v", rep)
		}
		if n := tg.count(t, "requirement") + tg.count(t, "component"); n != 0 {
			t.Fatalf("%d entities written without a grant", n)
		}
		recs := sink.Records()
		if len(recs) == 0 {
			t.Fatal("refused writes left no audit trail")
		}
		for _, r := range recs {
			if r.Op != audit.OpDeniedWrite || r.Principal.User != "system:peer:cat" {
				t.Fatalf("want only denied writes by the peer, got %+v", r)
			}
		}
	})
	t.Run("grant writes as the peer", func(t *testing.T) {
		tg, sink, ctx := aclTarget(t, peerPolicy(true))
		rep, err := Run(ctx, testMapping(t, tg.meta, nil), tg.deps(seedSource()), Options{})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if rep.Entities.Created != 4 || rep.Relations.Created != 2 {
			t.Fatalf("report: %+v", rep)
		}
		recs := sink.Records()
		if len(recs) == 0 {
			t.Fatal("no audit records")
		}
		for _, r := range recs {
			if r.Op == audit.OpDeniedWrite || r.Principal.User != "system:peer:cat" ||
				r.Principal.Tool != principal.ToolPeerPull {

				t.Fatalf("audit record not attributed to the peer: %+v", r)
			}
		}
	})
}
