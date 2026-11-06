package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
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

func (a denyAndHideACL) PermitsReadFace(_ context.Context, _, id string, _ entity.Face) (bool, error) {
	return id != a.hidden, nil
}

func newScopedManager(t *testing.T, aud audit.Audit) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(patchMetamodelYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL:         denyAndHideACL{hidden: "TASK-HIDDEN"},
		Transitions: statemachine.EmptySet(),
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

// TestScopedUpdates_Transitions pins that an elevated update keeps the
// transition rules, including the caller's guard permission, and that a
// refused transition names neither the stored nor the requested value.
func TestScopedUpdates_Transitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		guard    statemachine.Guard
		to       string
		sentinel error // nil: the update lands
	}{
		{name: "legal move", guard: allowAllGuard{}, to: "approved"},
		{name: "undeclared move", guard: allowAllGuard{}, to: "established", sentinel: statemachine.ErrIllegalTransition},
		{name: "guard checks caller", guard: denyAllGuard{}, to: "approved", sentinel: acl.ErrForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr := newTransitionManager(t, tc.guard)
			snap := seedSnapshot(t, mgr, "")
			su, err := entitymanager.NewScopedUpdates(mgr,
				entitymanager.UpdateScope{"snapshot": {"status"}}, "action:approve")
			if err != nil {
				t.Fatalf("NewScopedUpdates: %v", err)
			}
			_, err = su.PatchEntity(context.Background(), snap.ID,
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
