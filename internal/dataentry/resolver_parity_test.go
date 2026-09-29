package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestResolverParity_GetVisibleRef pins visibility.Resolver.Address to the
// dataentry read it replaces (TKT-2528AB PR 2): for every gate branch, over
// faced and faceless types and across worlds, the two agree on hit or miss,
// on a returned gate error, on the served row, and on its provenance.
//
// The comparison target is the entity GET's verdict: getVisibleRef plus the
// stored-type check the handler applies after it. The resolver is built over
// the same per-request gate (ctxRowGate) and the same store, with no
// redaction, so the only thing compared is the gate order and the load.
func TestResolverParity_GetVisibleRef(t *testing.T) {
	st := appbuildtest.New(facedMeta(t)).Store()
	seedPolicyFaces(t, st)
	vr := newVisibleReader(st)
	res, err := visibility.NewResolver(ctxRowGate{}, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all":            {Read: []string{"*"}},
			"published-only": {Read: []string{"policy@published", "feature"}},
			"feature-only":   {Read: []string{"feature"}},
		},
		Assignments: map[string]string{"alice": "all", "bob": "published-only", "carol": "feature-only"},
	}, st)
	as := func(user string) context.Context { return gateCtxFor(principalCtx(user), t, d) }
	gateDown := withReadGate(context.Background(), fakeGate{permitsErr: errors.New("gate down")})

	worlds := map[string]worldHandle{
		"default":   {},
		"published": {name: "published", scope: policyPublishedScope()},
		// The chain prefers the draft, so a published-only reader is served
		// its second choice: the ACL trims first, the world ranks the rest.
		"review": {name: "review", scope: store.NewWorldScope(map[string]store.TypeResolution{
			"policy": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
		})},
		"denied": {name: "editorial", denied: true},
	}
	principals := map[string]context.Context{
		"all": as("alice"), "published-only": as("bob"), "feature-only": as("carol"), "gate error": gateDown,
	}
	addrs := []struct{ typ, addr string }{
		{"policy", "POL-1"},
		{"policy", "POL-1@draft"},
		{"policy", "POL-1@published"},
		{"policy", "POL-1@nope"},
		{"policy", "POL-404"},
		{"policy", "FEAT-1"}, // stored type differs
		{"feature", "FEAT-1"},
		{"feature", "FEAT-1@draft"},
		{"feature", "POL-1@published"}, // stored type differs, named face
	}

	hits := 0
	for pname, pctx := range principals {
		for wname, w := range worlds {
			for _, a := range addrs {
				t.Run(pname+"/"+wname+"/"+a.typ+":"+a.addr, func(t *testing.T) {
					ctx := withWorld(pctx, w)
					ref, ok := parseEntityRef(a.addr)
					if !ok {
						t.Fatalf("fixture address %q does not parse", a.addr)
					}
					want, wantOK, wantErr := vr.getVisibleRef(ctx, a.typ, ref)
					// getVisibleRef leaves the stored-type check to its
					// caller; the entity GET applies it right after
					// (api_v1.go). The resolver owns it, so the parity
					// target is the GET handler's verdict.
					wantOK = wantOK && want.Type == a.typ
					got, gotOK, gotErr := res.Address(ctx, w.visibility(), a.typ, a.addr)

					if (wantErr != nil) != (gotErr != nil) {
						t.Fatalf("error: getVisibleRef %v, resolver %v", wantErr, gotErr)
					}
					if wantOK != gotOK {
						t.Fatalf("hit: getVisibleRef %v, resolver %v", wantOK, gotOK)
					}
					if !wantOK {
						return
					}
					hits++
					if want.Ref() != got.Entity.Ref() || want.Type != got.Entity.Type {
						t.Fatalf("served: getVisibleRef %v, resolver %v", want.Ref(), got.Entity.Ref())
					}
					wantRule, wantPos := resolutionRuleAt(w.scope, a.typ, want.Face)
					if got.Via.String() != wantRule {
						t.Errorf("via: want %s, resolver %s", wantRule, got.Via)
					}
					switch {
					case wantPos != nil && *wantPos != got.ChainPosition:
						t.Errorf("chain position: want %d, resolver %d", *wantPos, got.ChainPosition)
					case wantPos == nil && got.ChainPosition != 0:
						t.Errorf("chain position: want none, resolver %d", got.ChainPosition)
					}
				})
			}
		}
	}
	// An address the grammar refuses: the GET 404s before any read
	// (parseEntityRef), and the resolver misses without an error.
	for _, bad := range []string{"not an id", "POL-1@@", "POL-1@Published"} {
		if _, ok := parseEntityRef(bad); ok {
			t.Fatalf("fixture %q parses; pick an address the grammar refuses", bad)
		}
		got, ok, err := res.Address(as("alice"), visibility.World{}, "policy", bad)
		if ok || err != nil || got.Entity != nil {
			t.Errorf("resolver on %q = (%v, %v), want a clean miss", bad, ok, err)
		}
	}

	// Guard the fixture: a matrix in which every read misses would pass
	// vacuously.
	if hits == 0 {
		t.Fatal("no case produced a hit; the fixture is not exercising the load")
	}
}

// TestResolverParity_WorldLoadErrorIsAMiss pins the one intended difference.
// getVisible returns a world-path store failure as an error (a 500); the
// resolver answers the same uniform miss a missing row gets and logs a
// warning (ruling 2, RR-FE1EGP), so a backend fault cannot be told apart
// from an entity with no face in the world.
func TestResolverParity_WorldLoadErrorIsAMiss(t *testing.T) {
	st := appbuildtest.New(facedMeta(t)).Store()
	seedPolicyFaces(t, st)
	broken := failingListStore{Store: st}
	ctx := withWorld(context.Background(), worldHandle{name: "published", scope: policyPublishedScope()})

	if _, _, err := newVisibleReader(broken).getVisible(ctx, "policy", "POL-1"); err == nil {
		t.Fatal("getVisible no longer surfaces a world-path load error; update this test and the resolver doc")
	}
	res, err := visibility.NewResolver(ctxRowGate{}, visibility.NopRedactor{}, broken)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := res.Address(ctx, worldFromContext(ctx).visibility(), "policy", "POL-1")
	if err != nil || ok || got.Entity != nil {
		t.Fatalf("resolver = (%v, %v, %v), want a clean miss", got.Entity, ok, err)
	}
}
