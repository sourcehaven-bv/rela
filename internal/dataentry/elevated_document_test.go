package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	scriptpkg "github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// stubACL is an acl.ACL implementation the switch has never been taught
// about — it stands in for a future one added without a matching arm. It
// deliberately AUTHORIZES every write, so a switch that fell through to some
// "ask the ACL" default would grant: the test proves the default arm denies
// on identity alone, not because this stub happens to say no.
type stubACL struct{}

func (stubACL) AuthorizeWrite(context.Context, acl.WriteRequest) acl.Decision {
	return acl.Decision{Allow: true}
}

// elevatedDoc is a document declaring allow_acl_bypass: read with the
// permission validateDocuments requires alongside it.
func elevatedDoc() dataentryconfig.DocumentConfig {
	return dataentryconfig.DocumentConfig{
		Script:         "docs/benchmark.lua",
		AllowACLBypass: metamodel.ACLBypassRead,
		Permission:     "report:sales",
	}
}

// TestElevatedDocument_ClosedSwitch is the table that pins the gate across
// every ACL implementation, including both the value and pointer forms of the
// nop/read-only types.
//
// The default arm is the point: an implementation nobody taught the switch
// about must not silently grant an ungated raw read.
func TestElevatedDocument_ClosedSwitch(t *testing.T) {
	t.Parallel()

	holds := permGate{perms: map[string]bool{"report:sales": true}}

	for _, tc := range []struct {
		name    string
		aclImpl acl.ACL
		want    bool
	}{
		{name: "nil ACL denies (wiring bug fails closed)", aclImpl: nil},
		{name: "NopACL value denies", aclImpl: acl.NopACL{}},
		{name: "NopACL pointer denies", aclImpl: &acl.NopACL{}},
		{name: "ReadOnlyACL value denies", aclImpl: acl.ReadOnlyACL{}},
		{name: "ReadOnlyACL pointer denies", aclImpl: &acl.ReadOnlyACL{}},
		{name: "nil *Declarative denies", aclImpl: (*acl.Declarative)(nil)},
		{name: "unknown implementation denies (default arm)", aclImpl: stubACL{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := withReadGate(t.Context(), holds)
			if got := authorizeElevatedDocument(ctx, tc.aclImpl, elevatedDoc()); got != tc.want {
				t.Errorf("authorizeElevatedDocument = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestElevatedDocument_NopACLDeniesDespitePermittingGate is the RR-CWWJGW
// canary, in the shape TestNavPermission_ReadOnlyArmIsExplicit established.
//
// Under NopACL and ReadOnlyACL readGateFromContext returns nopReadGate, whose
// HoldsPermission returns TRUE unconditionally. A gate written against the read
// gate alone would therefore ALLOW an elevated render on a policy-less
// deployment — serving whatever the script reads to every caller.
//
// Attaching a gate that GRANTS the permission proves the denial comes from the
// explicit arm rather than from the gate happening to agree: if someone deletes
// the NopACL arm, the value falls to the default arm (which also denies) — so
// to make this test meaningful the assertion is that the answer is deny while a
// permitting gate is present, i.e. the gate was never consulted.
func TestElevatedDocument_NopACLDeniesDespitePermittingGate(t *testing.T) {
	t.Parallel()

	// A gate that grants everything — the nopReadGate posture, made explicit.
	ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"report:sales": true}})

	for _, impl := range []acl.ACL{acl.NopACL{}, &acl.NopACL{}, acl.ReadOnlyACL{}, &acl.ReadOnlyACL{}} {
		if authorizeElevatedDocument(ctx, impl, elevatedDoc()) {
			t.Errorf("%T allowed an elevated render with no configured policy; the permission "+
				"names a capability nothing can withhold there, so the document would serve "+
				"its raw reads to every principal", impl)
		}
	}
}

// TestElevatedDocument_UnelevatedIsUnaffected pins that this gate is inert for
// the overwhelmingly common document. The nested CLAUDE.md is explicit that
// `permission:` must not be hardened into a required field for ordinary
// documents; this ticket narrows that only for elevated ones.
func TestElevatedDocument_UnelevatedIsUnaffected(t *testing.T) {
	t.Parallel()

	plain := dataentryconfig.DocumentConfig{Script: "docs/report.lua"}
	gated := dataentryconfig.DocumentConfig{Script: "docs/report.lua", Permission: "report:sales"}
	ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{}})

	// Every implementation, including the ones that deny elevation outright.
	for _, impl := range []acl.ACL{nil, acl.NopACL{}, acl.ReadOnlyACL{}, stubACL{}} {
		if !authorizeElevatedDocument(ctx, impl, plain) {
			t.Errorf("%T: an unelevated document must pass this gate untouched", impl)
		}
		// A permission on an unelevated document is still enforced, but by
		// gateDocumentPermission — not here.
		if !authorizeElevatedDocument(ctx, impl, gated) {
			t.Errorf("%T: an unelevated document with a permission must pass THIS gate; "+
				"gateDocumentPermission is what enforces it", impl)
		}
	}
}

// TestElevatedDocument_DeclarativeRequiresHeldPermission pins the arm that
// actually authorizes: under a real policy the principal must hold it.
func TestElevatedDocument_DeclarativeRequiresHeldPermission(t *testing.T) {
	t.Parallel()

	policy := &acl.Declarative{}

	t.Run("holder allowed", func(t *testing.T) {
		t.Parallel()
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"report:sales": true}})
		if !authorizeElevatedDocument(ctx, policy, elevatedDoc()) {
			t.Error("a principal holding the permission must render the elevated document")
		}
	})

	t.Run("non-holder denied", func(t *testing.T) {
		t.Parallel()
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"other:perm": true}})
		if authorizeElevatedDocument(ctx, policy, elevatedDoc()) {
			t.Error("a principal without the permission must not render the elevated document")
		}
	})

	t.Run("empty permission denied even under a permitting gate", func(t *testing.T) {
		t.Parallel()
		// validateDocuments rejects this config, so it is unreachable today.
		// The guard must not depend on validation having run: a new
		// construction path that skipped it would otherwise fail open.
		doc := elevatedDoc()
		doc.Permission = ""
		ctx := withReadGate(t.Context(), permGate{perms: map[string]bool{"report:sales": true}})
		if authorizeElevatedDocument(ctx, policy, doc) {
			t.Error("an elevated document with no permission must be denied, not allowed " +
				"on the assumption that config validation caught it")
		}
	})
}

// --- End-to-end HTTP behavior (AC4, AC5, AC9) ---

// TestElevatedDocument_DeniedPrincipalNeverReachesRenderer pins AC9. The
// status code alone is not enough: the point of gating BEFORE the render is
// that an unauthorized caller cannot trigger an expensive Lua aggregation, so
// the assertion is on the renderer call count.
func TestElevatedDocument_DeniedPrincipalNeverReachesRenderer(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"benchmark": {
			Script:         "docs/benchmark.lua",
			AllowACLBypass: metamodel.ACLBypassRead,
			Permission:     "report:sales",
		},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, standaloneDocReq(gateCtxFor(principalCtx("bob"), t, d), "benchmark"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("a principal without the permission must be denied: got %d; body=%s", rec.Code, rec.Body)
	}
	if fake.callCount() != 0 {
		t.Errorf("the renderer ran %d times for a denied principal; an unauthorized caller must "+
			"not be able to trigger an elevated aggregation", fake.callCount())
	}
}

// TestElevatedDocument_PermittedPrincipalRenders pins AC4: the holder of the
// declared permission gets the document.
func TestElevatedDocument_PermittedPrincipalRenders(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"benchmark": {
			Script:         "docs/benchmark.lua",
			AllowACLBypass: metamodel.ACLBypassRead,
			Permission:     "report:sales",
		},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"analyst": {Read: []string{"ticket"}, Permissions: []string{"report:sales"}},
		},
		Assignments: map[string]string{"alice": "analyst"},
	}, app.store)
	app.acl = d

	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, standaloneDocReq(gateCtxFor(principalCtx("alice"), t, d), "benchmark"))

	if rec.Code != http.StatusOK {
		t.Fatalf("the permission holder must render the document: got %d; body=%s", rec.Code, rec.Body)
	}
	if fake.callCount() != 1 {
		t.Errorf("expected exactly one render, got %d", fake.callCount())
	}
}

// TestElevatedDocument_NopACLRefusesEndToEnd is the deployment-shaped version
// of the NopACL arm: a project with no acl.yaml must not serve an elevated
// document, even though its permission "passes" the read gate there.
func TestElevatedDocument_NopACLRefusesEndToEnd(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"benchmark": {
			Script:         "docs/benchmark.lua",
			AllowACLBypass: metamodel.ACLBypassRead,
			Permission:     "report:sales",
		},
	}
	app.acl = acl.NopACL{}

	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, standaloneDocReq(principalCtx("anyone"), "benchmark"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("an elevated document must not serve without a configured policy: got %d; body=%s",
			rec.Code, rec.Body)
	}
	if fake.callCount() != 0 {
		t.Errorf("the renderer ran %d times under NopACL", fake.callCount())
	}
}

// TestElevatedDeps_GrantsBypassBinding checks that elevatedDeps populates the
// deps struct such that a runtime built from it exposes the right bindings.
//
// Scope, precisely: it builds lua.NewWriter DIRECTLY, which is NOT the
// production path (that is ExecuteStandaloneDocument -> runDocumentScript ->
// NewWriterRuntime, with a different options chain). So this pins the
// deps-to-bindings mapping, not the end-to-end wiring — if NewWriterRuntime
// ever transformed deps for document mode, this would keep passing.
//
// TestElevatedRender_ReadsHiddenEntityAndAudits is the one that covers the
// real path; keep both, since this one localizes a failure to elevatedDeps
// while that one proves the feature works.
func TestElevatedDeps_GrantsBypassBinding(t *testing.T) {
	for _, tc := range []struct {
		name     string
		elevated bool
		script   string
	}{
		{
			name:     "elevated render has bypass_acl",
			elevated: true,
			script:   `if rela.bypass_acl == nil then error("bypass_acl absent") end`,
		},
		{
			// The structural guarantee, stated precisely: the ELEVATED handle
			// cannot mutate, because the write methods do not exist on it. The
			// render can still write through the ordinary gated rela.*
			// bindings (TKT-PX5YL7) — what this pins is that elevation adds no
			// write capability, not that the surface is read-only.
			name:     "elevated handle carries reads and no writes",
			elevated: true,
			script: `rela.bypass_acl(function(admin)
				if admin.delete_entity ~= nil then error("delete_entity present") end
				if admin.create_relation ~= nil then error("create_relation present") end
				if admin.delete_relation ~= nil then error("delete_relation present") end
				if admin.get_entity == nil then error("get_entity absent") end
				if admin.list_entities == nil then error("list_entities absent") end
				if admin.get_relations == nil then error("get_relations absent") end
			end)`,
		},
		{
			name:     "unelevated render has no bypass_acl",
			elevated: false,
			script:   `if rela.bypass_acl ~= nil then error("bypass_acl present") end`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			deps := app.documents.elevatedDeps(documentRenderConfig{Elevated: tc.elevated})
			var out strings.Builder
			r := lua.NewWriter(deps, &out)
			defer r.Close()
			if err := r.RunString(tc.script); err != nil {
				t.Errorf("script assertion failed: %v", err)
			}
		})
	}
}

// denyMutator satisfies lua.Mutator so NewWriter accepts the deps. Its methods
// are never called: these tests render, they do not write.
//
// Its existence is itself informative — lua.NewWriter PANICS without an
// EntityManager, which is the clearest available proof that a document render
// is a writer runtime and not a read-only one (RR-DOCWRT, TKT-PX5YL7).
type denyMutator struct{}

func (denyMutator) CreateEntity(context.Context, *entity.Entity, entity.CreateOptions) (*entity.CreateResult, error) {
	return nil, errors.New("writes are not expected in this test")
}

func (denyMutator) UpdateEntity(context.Context, *entity.Entity) (*entity.UpdateResult, error) {
	return nil, errors.New("writes are not expected in this test")
}

func (denyMutator) PatchEntity(context.Context, string, entity.Patch) (*entity.UpdateResult, error) {
	return nil, errors.New("writes are not expected in this test")
}

func (denyMutator) DeleteEntity(context.Context, string, bool) (*entity.DeleteResult, error) {
	return nil, errors.New("writes are not expected in this test")
}

func (denyMutator) CreateRelation(
	context.Context, string, string, string, entity.RelationOptions,
) (*entity.Relation, error) {
	return nil, errors.New("writes are not expected in this test")
}

func (denyMutator) DeleteRelation(context.Context, string, string, string) error {
	return errors.New("writes are not expected in this test")
}

// --- Integration: the elevated reader actually arrives, and is audited ---

// capturingAudit records every audit row for assertion.
type capturingAudit struct {
	mu   sync.Mutex
	recs []audit.Record
}

func (c *capturingAudit) Record(rec audit.Record) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recs = append(c.recs, rec)
}

func (c *capturingAudit) ops() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.recs))
	for _, r := range c.recs {
		out = append(out, r.Op)
	}
	return out
}

// TestElevatedRender_ReadsHiddenEntityAndAudits is the integration test the
// unit tests in this file cannot be: it runs a REAL Lua script through the
// REAL script.Engine, so the whole production path is exercised —
// ExecuteStandaloneDocument -> runDocumentScript -> NewWriterRuntime.
//
// Two properties, both of which the other tests can pass without:
//
//  1. The elevated reader ARRIVES. Every HTTP test here stubs the renderer via
//     withFakeDocRenderer, whose newTestService passes a nil elevation func —
//     so those tests render with elevation disabled and would pass identically
//     if the wiring were broken or absent.
//  2. The elevated read is AUDITED. document.go calls the acl-bypass-read trail
//     "the exact gap TKT-ACSBSA closed; it must not reopen on a new surface",
//     and nothing else in this package tests it.
func TestElevatedRender_ReadsHiddenEntityAndAudits(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// The script reads through the ELEVATED handle. If the elevated reader
	// never arrives, admin.get_entity raises and the render fails — so this
	// script failing is itself the assertion.
	script := `rela.bypass_acl(function(admin)
  local e = admin.get_entity("SECRET-1")
  print("# " .. (e and e.id or "MISSING"))
end)`
	if err := os.WriteFile(filepath.Join(root, "scripts", "docs", "bench.lua"), []byte(script), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	st := memstore.New()
	if err := st.CreateEntity(t.Context(), &entity.Entity{ID: "SECRET-1", Type: "ticket"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	sink := &capturingAudit{}
	svc := newDocumentService(st, nil, root, scriptpkg.NewEngine(),
		// A DENY-ALL gated reader: the ordinary rela.* reads see nothing, so
		// only the elevated handle can reach SECRET-1. This is what makes the
		// test prove elevation rather than merely "a read happened".
		func() lua.WriteDeps {
			return lua.WriteDeps{
				ReadDeps: lua.ReadDeps{
					VisibleReader: visibility.DenyReader{},
					ProjectRoot:   root,
				},
				EntityManager: denyMutator{},
			}
		},
		func() documentElevation {
			return documentElevation{
				Reader:   visibility.Unrestricted(st),
				Recorder: elevationRecorder(sink),
			}
		})

	out, err := svc.RenderStandalone(t.Context(), documentRenderConfig{
		ConfigID: "bench",
		Script:   "docs/bench.lua",
		Elevated: true,
	})
	if err != nil {
		t.Fatalf("elevated render failed (the elevated reader did not arrive): %v", err)
	}
	if !strings.Contains(out, "SECRET-1") {
		t.Errorf("render did not read the hidden entity through the elevated handle; got %q", out)
	}

	// Exactly one row, and it is the elevated-read op.
	ops := sink.ops()
	var bypassReads int
	for _, op := range ops {
		if op == audit.OpACLBypassRead {
			bypassReads++
		}
	}
	if bypassReads != 1 {
		t.Errorf("acl-bypass-read rows = %d, want exactly 1 (ops: %v); granting the "+
			"capability without the audit trace is the gap TKT-ACSBSA closed", bypassReads, ops)
	}
}

// TestUnelevatedRender_CannotReachHiddenEntity is the negative half: the same
// script on an UNELEVATED document must fail, because rela.bypass_acl is not
// registered at all. Without this, the test above could pass on a build that
// elevated every render.
func TestUnelevatedRender_CannotReachHiddenEntity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	script := `rela.bypass_acl(function(admin) print(admin.get_entity("SECRET-1").id) end)`
	if err := os.WriteFile(filepath.Join(root, "scripts", "docs", "bench.lua"), []byte(script), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	st := memstore.New()
	if err := st.CreateEntity(t.Context(), &entity.Entity{ID: "SECRET-1", Type: "ticket"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sink := &capturingAudit{}
	svc := newDocumentService(st, nil, root, scriptpkg.NewEngine(),
		func() lua.WriteDeps {
			return lua.WriteDeps{
				ReadDeps: lua.ReadDeps{
					VisibleReader: visibility.DenyReader{},
					ProjectRoot:   root,
				},
				EntityManager: denyMutator{},
			}
		},
		func() documentElevation {
			return documentElevation{Reader: visibility.Unrestricted(st), Recorder: elevationRecorder(sink)}
		})

	// Elevated:false — the capability is available to the service but this
	// document did not declare it.
	if _, err := svc.RenderStandalone(t.Context(), documentRenderConfig{
		ConfigID: "bench",
		Script:   "docs/bench.lua",
	}); err == nil {
		t.Fatal("an unelevated render called rela.bypass_acl successfully; the binding " +
			"must be absent unless the document declares allow_acl_bypass")
	}
	if n := len(sink.ops()); n != 0 {
		t.Errorf("unelevated render produced %d audit rows, want 0 (ops: %v)", n, sink.ops())
	}
}
