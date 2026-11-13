package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// denyAndHideACL denies every write and hides one id from every reader: the
// caller of a scoped elevated update who may read rows but not write them.
type denyAndHideACL struct {
	acl.NopACL
	hidden string
}

func (denyAndHideACL) AuthorizeWrite(context.Context, acl.WriteRequest) acl.Decision {
	return acl.Decision{Allow: false, RuleKind: "read-only", RuleID: "test-deny-all"}
}

// AuthorizePreRoleWrite allows: the fake's refusal stands for the role
// grants, which elevation lifts, and it has no other check.
func (denyAndHideACL) AuthorizePreRoleWrite(context.Context, acl.WriteRequest) acl.Decision {
	return acl.Decision{Allow: true, RuleKind: "pre-role", RuleID: "-"}
}

func (a denyAndHideACL) PermitsReadFace(_ context.Context, _, id string, _ entity.Face) (bool, error) {
	return id != a.hidden, nil
}

func newScopedManager(t *testing.T, aud audit.Audit) (*entitymanager.Manager, store.Store) {
	t.Helper()
	return newScopedManagerWithACL(t, aud, denyAndHideACL{hidden: "TASK-HIDDEN"})
}

func newScopedManagerWithACL(t *testing.T, aud audit.Audit, a acl.ACL) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(patchMetamodelYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL:         a,
		Transitions: flowcfg.NoTransitions(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	seedTask(t, st, "TASK-1", map[string]any{"title": "T", "status": "draft"}, "body")
	seedTask(t, st, "TASK-HIDDEN", map[string]any{"title": "H", "status": "draft"}, "")
	return mgr, st
}

func TestNewScopedUpdates_RejectsInvalidInput(t *testing.T) {
	t.Parallel()
	mgr, _ := newScopedManager(t, audit.Nop{})
	tests := []struct {
		name   string
		m      *entitymanager.Manager
		scope  entitymanager.UpdateScope
		source string
	}{
		{name: "nil manager", scope: entitymanager.UpdateScope{"task": {"status"}}, source: "action:a"},
		{name: "empty source", m: mgr, scope: entitymanager.UpdateScope{"task": {"status"}}},
		{name: "empty scope", m: mgr, source: "action:a"},
		{name: "type without properties", m: mgr, scope: entitymanager.UpdateScope{"task": nil}, source: "action:a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := entitymanager.NewScopedUpdates(tc.m, tc.scope, tc.source); err == nil {
				t.Error("NewScopedUpdates accepted invalid input")
			}
		})
	}
}

// TestScopedUpdates_PatchEntity pins the scope of an elevated update
// (TKT-JA5SABG): the caller's write ACL denies everything, a patch inside the
// scope still lands, and everything else is refused with the row unchanged.
func TestScopedUpdates_PatchEntity(t *testing.T) {
	t.Parallel()
	body := "changed"
	tests := []struct {
		name       string
		id         string
		patch      entity.Patch
		wantErr    string
		notFound   bool
		wantStatus string
	}{
		{
			name:       "declared property",
			id:         "TASK-1",
			patch:      entity.Patch{Properties: map[string]any{"status": "submitted"}},
			wantStatus: "submitted",
		},
		{
			name:       "declared property cleared",
			id:         "TASK-1",
			patch:      entity.Patch{MetaUnset: []string{"status"}},
			wantStatus: "",
		},
		{
			name:       "undeclared property",
			id:         "TASK-1",
			patch:      entity.Patch{Properties: map[string]any{"status": "submitted", "title": "x"}},
			wantErr:    `property "title"`,
			wantStatus: "draft",
		},
		{
			name:       "undeclared property cleared",
			id:         "TASK-1",
			patch:      entity.Patch{MetaUnset: []string{"title"}},
			wantErr:    `property "title"`,
			wantStatus: "draft",
		},
		{
			name: "content",
			id:   "TASK-1",
			patch: entity.Patch{
				Properties: map[string]any{"status": "submitted"}, Content: &body,
			},
			wantErr:    "content",
			wantStatus: "draft",
		},
		{
			name: "external refs",
			id:   "TASK-1",
			patch: entity.Patch{
				Properties: map[string]any{"status": "submitted"}, WriteExternalRefs: true,
			},
			wantErr:    "external refs",
			wantStatus: "draft",
		},
		{
			name:       "empty patch",
			id:         "TASK-1",
			wantErr:    "no properties",
			wantStatus: "draft",
		},
		{
			name:     "unreadable row",
			id:       "TASK-HIDDEN",
			patch:    entity.Patch{Properties: map[string]any{"status": "submitted"}},
			notFound: true,
		},
		{
			name:     "unreadable row, property outside the scope",
			id:       "TASK-HIDDEN",
			patch:    entity.Patch{Properties: map[string]any{"title": "x"}},
			notFound: true,
		},
		{
			name:     "missing row",
			id:       "TASK-404",
			patch:    entity.Patch{Properties: map[string]any{"status": "submitted"}},
			notFound: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr, st := newScopedManager(t, audit.Nop{})
			su, err := entitymanager.NewScopedUpdates(mgr, entitymanager.UpdateScope{"task": {"status"}}, "action:a")
			if err != nil {
				t.Fatal(err)
			}
			_, err = su.PatchEntity(asUser("bob"), tc.id, tc.patch)
			switch {
			case tc.notFound:
				if !errors.Is(err, entitymanager.ErrEntityNotFound) {
					t.Fatalf("err = %v, want ErrEntityNotFound", err)
				}
				if got := mustGet(t, st, "TASK-HIDDEN").Properties["status"]; got != "draft" {
					t.Errorf("hidden row status = %v, want draft", got)
				}
				return
			case tc.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
			case err != nil:
				t.Fatalf("PatchEntity: %v", err)
			}
			got := mustGet(t, st, "TASK-1")
			if status, _ := got.Properties["status"].(string); status != tc.wantStatus {
				t.Errorf("status = %q, want %q", status, tc.wantStatus)
			}
			if got.Properties["title"] != "T" || got.Content != "body" {
				t.Errorf("unnamed fields changed: %v %q", got.Properties, got.Content)
			}
		})
	}
}

// TestScopedUpdates_WrongType pins that a type outside the scope is refused.
func TestScopedUpdates_WrongType(t *testing.T) {
	t.Parallel()
	mgr, st := newScopedManager(t, audit.Nop{})
	su, err := entitymanager.NewScopedUpdates(mgr, entitymanager.UpdateScope{"other": {"status"}}, "action:a")
	if err != nil {
		t.Fatal(err)
	}
	_, err = su.PatchEntity(asUser("bob"), "TASK-1", entity.Patch{Properties: map[string]any{"status": "x"}})
	if err == nil || !strings.Contains(err.Error(), `entity type "task"`) {
		t.Fatalf("err = %v, want a type scope error", err)
	}
	if got := mustGet(t, st, "TASK-1").Properties["status"]; got != "draft" {
		t.Errorf("status = %v, want draft", got)
	}
}

// TestScopedUpdates_Audit pins the audit rows of a scoped elevated update:
// the bypass marker and the update, both naming the real principal and the
// source.
func TestScopedUpdates_Audit(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr, _ := newScopedManager(t, mem)
	su, err := entitymanager.NewScopedUpdates(mgr, entitymanager.UpdateScope{"task": {"status"}}, "action:submit")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := su.PatchEntity(asUser("bob"), "TASK-1",
		entity.Patch{Properties: map[string]any{"status": "submitted"}}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, rec := range mem.Records() {
		if rec.Principal.User != "bob" {
			t.Errorf("%s row principal = %q, want bob", rec.Op, rec.Principal.User)
		}
		if rec.TriggeredBy != "action:submit" {
			t.Errorf("%s row triggered_by = %q, want action:submit", rec.Op, rec.TriggeredBy)
		}
		seen[rec.Op] = true
	}
	if !seen[audit.OpACLBypass] || !seen[audit.OpUpdateEntity] {
		t.Errorf("audit ops = %v, want %s and %s", seen, audit.OpACLBypass, audit.OpUpdateEntity)
	}
}

// userGuard grants a guard permission to one user only, so a test can tell
// that the guard asked about the caller.
type userGuard struct{ user string }

func (g userGuard) HoldsPermission(ctx context.Context, _, _ string) bool {
	return principal.From(ctx).User == g.user
}

// TestScopedUpdates_Transitions pins that an elevated update keeps the
// transition rules, including the caller's guard permission, and that a
// refused transition names neither the stored nor the requested value.
func TestScopedUpdates_Transitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		user     string
		to       string
		sentinel error // nil: the update lands
	}{
		{name: "guard admits the caller", user: "bob", to: "approved"},
		{name: "undeclared move", user: "bob", to: "established", sentinel: flowcfg.ErrIllegalTransition},
		{name: "guard refuses another caller", user: "alice", to: "approved", sentinel: acl.ErrForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr := newTransitionManager(t, userGuard{user: "bob"})
			snap := seedSnapshot(t, mgr, "")
			su, err := entitymanager.NewScopedUpdates(mgr,
				entitymanager.UpdateScope{"snapshot": {"status"}}, "action:approve")
			if err != nil {
				t.Fatalf("NewScopedUpdates: %v", err)
			}
			_, err = su.PatchEntity(asUser(tc.user), snap.ID,
				entity.Patch{Properties: map[string]any{"status": tc.to}})
			if tc.sentinel == nil {
				if err != nil {
					t.Fatalf("PatchEntity: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("err = %v, want %v", err, tc.sentinel)
			}
			for _, value := range []string{"in-review", tc.to} {
				if strings.Contains(err.Error(), value) {
					t.Errorf("error %q names value %q", err, value)
				}
			}
		})
	}
}

// TestScopedUpdates_ReadOnly pins that a read-only instance refuses a scoped
// elevated update as it refuses any update: a 403-class error, a
// denied-write audit row, and the row unchanged. Elevation skips
// AuthorizeWrite, so without this check --read-only would not hold.
func TestScopedUpdates_ReadOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		acl        acl.ACL
		wantDenied bool
	}{
		{name: "read-only value", acl: acl.ReadOnlyACL{}, wantDenied: true},
		{name: "read-only pointer", acl: &acl.ReadOnlyACL{}, wantDenied: true},
		{name: "write-denying policy still elevates", acl: denyAndHideACL{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mem := audit.NewMemory()
			mgr, st := newScopedManagerWithACL(t, mem, tc.acl)
			su, err := entitymanager.NewScopedUpdates(mgr, entitymanager.UpdateScope{"task": {"status"}}, "action:submit")
			if err != nil {
				t.Fatal(err)
			}
			_, err = su.PatchEntity(asUser("bob"), "TASK-1",
				entity.Patch{Properties: map[string]any{"status": "submitted"}})
			if got := errors.Is(err, acl.ErrForbidden); got != tc.wantDenied {
				t.Fatalf("forbidden = %v (err %v), want %v", got, err, tc.wantDenied)
			}
			want := "submitted"
			if tc.wantDenied {
				want = "draft"
			}
			stored, gerr := st.GetEntity(context.Background(), entity.Ref{ID: "TASK-1"})
			if gerr != nil {
				t.Fatal(gerr)
			}
			if got := stored.GetString("status"); got != want {
				t.Errorf("status = %q, want %q", got, want)
			}
			ops := map[string]bool{}
			for _, rec := range mem.Records() {
				ops[rec.Op] = true
			}
			if tc.wantDenied && (!ops[audit.OpDeniedWrite] || ops[audit.OpACLBypass] || ops[audit.OpUpdateEntity]) {
				t.Errorf("audit ops = %v, want only %s", ops, audit.OpDeniedWrite)
			}
		})
	}
}

// scopedCeilingManager builds a Manager under ceilingPolicy with a memory
// audit log. bob holds only the reader role, so an update by bob lands only
// through elevation.
func scopedCeilingManager(t *testing.T, aud audit.Audit) (*entitymanager.Manager, store.Store) {
	t.Helper()
	return scopedPolicyManager(t, aud, ceilingPolicy)
}

// scopedPolicyManager is scopedCeilingManager under policyYAML. The principal
// lookup is wired, so a policy that names user_entity_type and
// principal_property marks a verified principal without a user entity as
// unmatched.
func scopedPolicyManager(t *testing.T, aud audit.Audit, policyYAML string) (*entitymanager.Manager, store.Store) {
	t.Helper()
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	st := memstore.New()
	decl, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(st), st,
		acl.WithPrincipalLookup(aclstore.MustNewPrincipalLookup(st)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	seeded := entity.New("REQ-001", "requirement")
	seeded.SetString("title", "seeded")
	if cErr := st.CreateEntity(context.Background(), seeded); cErr != nil {
		t.Fatalf("seed: %v", cErr)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: parseMeta(t), Templater: nopTemplater{}, Audit: aud, ACL: decl,
		Transitions:    flowcfg.NoTransitions(),
		FieldGate:      entitymanager.AllowAllFieldGate{},
		CopyReadGate:   entitymanager.AllowAllCopyReadGate{},
		CopyVisibility: allowAllCopyVisibility(t, st),
		AnchorGuard:    entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

// TestScopedUpdates_ClientCeiling pins that elevation lifts the role grants
// but not the client ceiling: a client under `deny_write: ["*"]` cannot make
// an elevated update, and the refusal is audited as a denied write. A
// ceiling that permits update on the type still elevates, although bob holds
// no update grant.
func TestScopedUpdates_ClientCeiling(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		principalType string
		scopes        []string
		wantDenied    bool
	}{
		{name: "deny_write baseline refuses", principalType: "app", wantDenied: true},
		{name: "scope grant re-opens update", principalType: "app", scopes: []string{"rela.req.write"}},
		{name: "interactive caller elevates"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mem := audit.NewMemory()
			mgr, st := scopedCeilingManager(t, mem)
			su, err := entitymanager.NewScopedUpdates(mgr,
				entitymanager.UpdateScope{"requirement": {"title"}}, "action:retitle")
			if err != nil {
				t.Fatal(err)
			}
			ctx := principal.With(context.Background(), principal.VerifiedFrom(
				"bob", principal.ToolDataEntry, principal.Claims{
					PrincipalType: tc.principalType, Scopes: tc.scopes,
				}))
			_, err = su.PatchEntity(entitymanager.BindAs(ctx, t, mgr), "REQ-001",
				entity.Patch{Properties: map[string]any{"title": "elevated"}})
			want := "elevated"
			if tc.wantDenied {
				want = "seeded"
				var fe *acl.ForbiddenError
				if !errors.As(err, &fe) || fe.Decision.RuleKind != "client-ceiling" {
					t.Fatalf("err = %v, want a client-ceiling 403", err)
				}
			} else if err != nil {
				t.Fatalf("PatchEntity: %v", err)
			}
			if got := mustGet(t, st, "REQ-001").GetString("title"); got != want {
				t.Errorf("title = %q, want %q", got, want)
			}
			ops := map[string]bool{}
			for _, rec := range mem.Records() {
				ops[rec.Op] = true
			}
			if tc.wantDenied && (!ops[audit.OpDeniedWrite] || ops[audit.OpACLBypass] || ops[audit.OpUpdateEntity]) {
				t.Errorf("audit ops = %v, want only %s", ops, audit.OpDeniedWrite)
			}
			if !tc.wantDenied && !ops[audit.OpACLBypass] {
				t.Errorf("audit ops = %v, want %s", ops, audit.OpACLBypass)
			}
		})
	}
}

// TestScopedUpdates_UnmatchedPrincipal pins that elevation does not lift
// `unmatched_principal: reject`: a verified principal with no user entity is
// refused its elevated write with the ACL's 403 and a denied-write audit row,
// and the row is unchanged. Under `anonymous` the same principal elevates.
func TestScopedUpdates_UnmatchedPrincipal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mode       string
		wantDenied bool
	}{
		{mode: acl.UnmatchedReject, wantDenied: true},
		{mode: acl.UnmatchedAnonymous},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			mem := audit.NewMemory()
			policy := ceilingPolicy + "user_entity_type: person\nprincipal_property: sub\n" +
				"asserted_role_assignments:\n  member: reader\n" +
				"unmatched_principal: " + tc.mode + "\n"
			mgr, st := scopedPolicyManager(t, mem, policy)
			su, err := entitymanager.NewScopedUpdates(mgr,
				entitymanager.UpdateScope{"requirement": {"title"}}, "action:retitle")
			if err != nil {
				t.Fatal(err)
			}
			// The asserted role makes the row readable; no role grants update.
			ctx := principal.With(context.Background(),
				principal.Verified("usr_nobody", principal.ToolDataEntry, "", "", []string{"member"}))
			ctx = acltest.BindVerified(ctx, t, entitymanager.DeclarativeOf(t, mgr))
			_, err = su.PatchEntity(ctx, "REQ-001", entity.Patch{Properties: map[string]any{"title": "elevated"}})

			want := "elevated"
			if tc.wantDenied {
				want = "seeded"
				var fe *acl.ForbiddenError
				if !errors.As(err, &fe) || fe.Decision.RuleKind != "unmatched-principal" {
					t.Fatalf("err = %v, want an unmatched-principal 403", err)
				}
			} else if err != nil {
				t.Fatalf("PatchEntity: %v", err)
			}
			if got := mustGet(t, st, "REQ-001").GetString("title"); got != want {
				t.Errorf("title = %q, want %q", got, want)
			}
			ops := map[string]bool{}
			for _, rec := range mem.Records() {
				ops[rec.Op] = true
			}
			if tc.wantDenied && (!ops[audit.OpDeniedWrite] || ops[audit.OpACLBypass] || ops[audit.OpUpdateEntity]) {
				t.Errorf("audit ops = %v, want only %s", ops, audit.OpDeniedWrite)
			}
			if !tc.wantDenied && !ops[audit.OpACLBypass] {
				t.Errorf("audit ops = %v, want %s", ops, audit.OpACLBypass)
			}
		})
	}
}

// TestScopedUpdates_Pin pins that an elevated write to the row the action
// started on fails when that row changed since, and that the handle's own
// writes move the pin.
func TestScopedUpdates_Pin(t *testing.T) {
	t.Parallel()
	status := func(v string) entity.Patch { return entity.Patch{Properties: map[string]any{"status": v}} }

	t.Run("concurrent change refuses", func(t *testing.T) {
		t.Parallel()
		mgr, st := newScopedManager(t, audit.Nop{})
		su, err := entitymanager.NewScopedUpdates(mgr, entitymanager.UpdateScope{"task": {"status"}}, "action:a")
		if err != nil {
			t.Fatal(err)
		}
		su.Pin(mustGet(t, st, "TASK-1"))
		changed := mustGet(t, st, "TASK-1")
		changed.SetString("title", "changed elsewhere")
		if uerr := st.UpdateEntity(context.Background(), changed); uerr != nil {
			t.Fatal(uerr)
		}
		_, err = su.PatchEntity(asUser("bob"), "TASK-1", status("submitted"))
		if !errors.Is(err, store.ErrConflict) {
			t.Fatalf("err = %v, want store.ErrConflict", err)
		}
		if got := mustGet(t, st, "TASK-1").GetString("status"); got != "draft" {
			t.Errorf("status = %q, want draft", got)
		}
	})

	t.Run("own writes move the pin", func(t *testing.T) {
		t.Parallel()
		mgr, st := newScopedManager(t, audit.Nop{})
		su, err := entitymanager.NewScopedUpdates(mgr, entitymanager.UpdateScope{"task": {"status"}}, "action:a")
		if err != nil {
			t.Fatal(err)
		}
		su.Pin(mustGet(t, st, "TASK-1"))
		for _, v := range []string{"submitted", "approved"} {
			if _, err := su.PatchEntity(asUser("bob"), "TASK-1", status(v)); err != nil {
				t.Fatalf("PatchEntity(%s): %v", v, err)
			}
		}
		if got := mustGet(t, st, "TASK-1").GetString("status"); got != "approved" {
			t.Errorf("status = %q, want approved", got)
		}
	})
}

// TestScopedUpdates_Faces pins the face dimension of the scope: a faced type
// is in the scope one face at a time, as `type@face`, and a bare type key
// covers no face of it.
func TestScopedUpdates_Faces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		scope   entitymanager.UpdateScope
		target  string
		wantErr bool
	}{
		{name: "named face", scope: entitymanager.UpdateScope{"beleid@concept": {"title"}}, target: "POL-1@concept"},
		{
			name: "other face", scope: entitymanager.UpdateScope{"beleid@concept": {"title"}},
			target: "POL-1@vastgesteld", wantErr: true,
		},
		{
			name: "bare type key", scope: entitymanager.UpdateScope{"beleid": {"title"}},
			target: "POL-1@concept", wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr, st := facedWriteManager(t, denyAndHideACL{})
			for _, face := range []entity.Face{"concept", "vastgesteld"} {
				e := entity.New("POL-1", "beleid")
				e.Face = face
				e.SetString("title", "old")
				if err := st.CreateEntity(context.Background(), e); err != nil {
					t.Fatalf("seed %s: %v", face, err)
				}
			}
			su, err := entitymanager.NewScopedUpdates(mgr, tc.scope, "action:a")
			if err != nil {
				t.Fatal(err)
			}
			_, err = su.PatchEntity(asUser("bob"), tc.target,
				entity.Patch{Properties: map[string]any{"title": "new"}})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "outside the scope") {
					t.Fatalf("err = %v, want a scope error", err)
				}
			} else if err != nil {
				t.Fatalf("PatchEntity: %v", err)
			}
			ref, perr := entity.ParseRef(tc.target)
			if perr != nil {
				t.Fatal(perr)
			}
			got, gerr := st.GetEntity(context.Background(), ref)
			if gerr != nil {
				t.Fatal(gerr)
			}
			want := "new"
			if tc.wantErr {
				want = "old"
			}
			if title := got.GetString("title"); title != want {
				t.Errorf("title = %q, want %q", title, want)
			}
		})
	}
}
