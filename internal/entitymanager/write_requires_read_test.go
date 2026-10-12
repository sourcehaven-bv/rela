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
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// hideOneACL allows every write and hides one id from every reader.
type hideOneACL struct {
	acl.NopACL
	hidden string
}

func (h hideOneACL) PermitsReadFace(_ context.Context, _, id string, _ entity.Face) (bool, error) {
	return id != h.hidden, nil
}

// writeBlindManager seeds a graph in which DEC-1 is the decision a caller
// may be unable to read, and builds a manager over it with the ACL a makes.
func writeBlindManager(t *testing.T, a func(store.Store) acl.ACL) (*entitymanager.Manager, store.Store) {
	t.Helper()
	st := memstore.New()
	seedHidden(t, st,
		[]*entity.Entity{
			{ID: "DEC-1", Type: "decision", Properties: map[string]any{"title": "secret"}},
			{ID: "REQ-1", Type: "requirement", Properties: map[string]any{"title": "visible"}},
			entity.New("api", "component"),
			entity.New("db", "component"),
		},
		[]entity.RelationKey{
			{From: "api", Type: "depends-on", To: "DEC-1"},
			{From: "api", Type: "depends-on", To: "db"},
		})
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: parseMeta(t), Templater: nopTemplater{},
		Audit: audit.Nop{}, ACL: a(st),
		Transitions:    statemachine.EmptySet(),
		FieldGate:      entitymanager.AllowAllFieldGate{},
		CopyReadGate:   entitymanager.AllowAllCopyReadGate{},
		CopyVisibility: allowAllCopyVisibility(t, st),
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

// writer is the caller of the writes under test. hideOneACL ignores who it
// is, so any principal will do.
func writer() context.Context { return asUser("writer") }

// hiding hides DEC-1; seeing hides nothing.
func hiding(store.Store) acl.ACL { return hideOneACL{hidden: "DEC-1"} }
func seeing(store.Store) acl.ACL { return hideOneACL{} }

// sameAsMissing fails unless hidden (the answer for DEC-1) and missing (the
// answer for DEC-404) both match want and read the same once the ids are
// swapped.
func sameAsMissing(t *testing.T, op string, hidden, missing, want error) {
	t.Helper()
	const hiddenID, missingID = "DEC-1", "DEC-404"
	if !errors.Is(hidden, want) {
		t.Errorf("%s on a hidden row = %v, want %v", op, hidden, want)
		return
	}
	if missing == nil {
		t.Fatalf("%s on a missing row succeeded", op)
	}
	if got, ref := strings.ReplaceAll(hidden.Error(), hiddenID, missingID), missing.Error(); got != ref {
		t.Errorf("%s: hidden row answers %q, missing row %q", op, got, ref)
	}
}

func decisionTitle(t *testing.T, st store.Store) string {
	t.Helper()
	e, err := st.GetEntity(context.Background(), entity.Ref{ID: "DEC-1"})
	if err != nil {
		t.Fatalf("DEC-1 is gone: %v", err)
	}
	title, _ := e.Properties["title"].(string)
	return title
}

func TestWriteRequiresRead_Entity(t *testing.T) {
	t.Parallel()
	patch := entity.Patch{Properties: map[string]any{"title": "overwritten"}}

	t.Run("patch", func(t *testing.T) {
		t.Parallel()
		mgr, st := writeBlindManager(t, hiding)
		_, hidden := mgr.PatchEntity(writer(), "DEC-1", patch)
		_, missing := mgr.PatchEntity(writer(), "DEC-404", patch)
		sameAsMissing(t, "patch", hidden, missing, entitymanager.ErrEntityNotFound)
		if got := decisionTitle(t, st); got != "secret" {
			t.Errorf("DEC-1 title = %q, want it untouched", got)
		}
		seer, _ := writeBlindManager(t, seeing)
		if _, err := seer.PatchEntity(writer(), "DEC-1", patch); err != nil {
			t.Errorf("patch of a readable row: %v", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		write := func(mgr *entitymanager.Manager, id string) error {
			_, err := mgr.UpdateEntity(writer(), &entity.Entity{
				ID: id, Type: "decision", Properties: map[string]any{"title": "overwritten"},
			})
			return err
		}
		mgr, st := writeBlindManager(t, hiding)
		sameAsMissing(t, "update", write(mgr, "DEC-1"), write(mgr, "DEC-404"), entitymanager.ErrEntityNotFound)
		if got := decisionTitle(t, st); got != "secret" {
			t.Errorf("DEC-1 title = %q, want it untouched", got)
		}
		seer, _ := writeBlindManager(t, seeing)
		if err := write(seer, "DEC-1"); err != nil {
			t.Errorf("update of a readable row: %v", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		mgr, st := writeBlindManager(t, hiding)
		_, hidden := mgr.DeleteEntity(writer(), "DEC-1", true)
		_, missing := mgr.DeleteEntity(writer(), "DEC-404", true)
		sameAsMissing(t, "delete", hidden, missing, entitymanager.ErrEntityNotFound)
		decisionTitle(t, st)
		seer, _ := writeBlindManager(t, seeing)
		if _, err := seer.DeleteEntity(writer(), "DEC-1", true); err != nil {
			t.Errorf("delete of a readable row: %v", err)
		}
	})
}

func TestWriteRequiresRead_Relation(t *testing.T) {
	t.Parallel()

	t.Run("create to a hidden head", func(t *testing.T) {
		t.Parallel()
		mgr, _ := writeBlindManager(t, hiding)
		create := func(to string) error {
			_, err := mgr.CreateRelation(writer(),
				entity.RelationKey{From: "db", Type: "depends-on", To: to}, entity.RelationOptions{})
			return err
		}
		sameAsMissing(t, "create", create("DEC-1"), create("DEC-404"), entitymanager.ErrEntityNotFound)
		if err := create("api"); err != nil {
			t.Errorf("create between readable rows: %v", err)
		}
	})

	t.Run("create from a hidden tail", func(t *testing.T) {
		t.Parallel()
		mgr, _ := writeBlindManager(t, hiding)
		create := func(from string) error {
			_, err := mgr.CreateRelation(writer(),
				entity.RelationKey{From: from, Type: "addresses", To: "REQ-1"}, entity.RelationOptions{})
			return err
		}
		sameAsMissing(t, "create", create("DEC-1"), create("DEC-404"), entitymanager.ErrEntityNotFound)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		mgr, _ := writeBlindManager(t, hiding)
		update := func(to string) error {
			_, err := mgr.UpdateRelation(writer(),
				entity.RelationKey{From: "api", Type: "depends-on", To: to},
				entity.RelationOptions{Properties: map[string]any{"note": "x"}})
			return err
		}
		sameAsMissing(t, "update", update("DEC-1"), update("DEC-404"), entitymanager.ErrRelationNotFound)
		if err := update("db"); err != nil {
			t.Errorf("update of a visible relation: %v", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		mgr, st := writeBlindManager(t, hiding)
		key := entity.RelationKey{From: "api", Type: "depends-on", To: "DEC-1"}
		hidden := mgr.DeleteRelation(writer(), key)
		missing := mgr.DeleteRelation(writer(),
			entity.RelationKey{From: "api", Type: "depends-on", To: "DEC-404"})
		sameAsMissing(t, "delete", hidden, missing, store.ErrNotFound)
		if _, err := st.GetRelation(writer(), key); err != nil {
			t.Errorf("hidden relation is gone: %v", err)
		}
		if err := mgr.DeleteRelation(writer(), entity.RelationKey{From: "api", Type: "depends-on", To: "db"}); err != nil {
			t.Errorf("delete of a visible relation: %v", err)
		}
	})
}

// A copy onto a stored target the caller cannot read is refused as a taken
// id, the answer a create with that id gets. The refusal does not say the
// target has another type.
func TestWriteRequiresRead_CopyTarget(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"TKT-2", "NOTE-9"} {
		mgr, st := newCopyAuthzManager(t, hideOneACL{hidden: target})
		ctx := context.Background()
		seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "src"}})
		seedRaw(ctx, t, st, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "victim"}})
		seedRaw(ctx, t, st, &entity.Entity{ID: "NOTE-9", Type: "note", Properties: map[string]any{"title": "a note"}})

		_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "spawn", SourceID: "TKT-1", TargetID: target})
		if !errors.Is(err, entitymanager.ErrEntityAlreadyExists) {
			t.Errorf("copy onto hidden %s = %v, want ErrEntityAlreadyExists", target, err)
		}
		if err != nil && strings.Contains(err.Error(), "note") {
			t.Errorf("LEAK: %q names the hidden target's type", err)
		}
		v, _ := st.GetEntity(ctx, entity.Ref{ID: target})
		if title, _ := v.Properties["title"].(string); title == "src" {
			t.Errorf("%s was overwritten", target)
		}
	}
}

// A soft delete of a row the caller cannot read answers like one of a
// missing row and marks nothing.
func TestWriteRequiresRead_SoftDelete(t *testing.T) {
	t.Parallel()
	mgr, st := writeBlindManager(t, hiding)
	_, hidden := entitymanager.SoftDeleteEntity(writer(), mgr, "DEC-1")
	_, missing := entitymanager.SoftDeleteEntity(writer(), mgr, "DEC-404")
	sameAsMissing(t, "soft delete", hidden, missing, entitymanager.ErrEntityNotFound)
	decisionTitle(t, st)
}

// erringACL allows every write and fails every read verdict.
type erringACL struct{ acl.NopACL }

var errGate = errors.New("gate unavailable")

func (erringACL) PermitsReadFace(context.Context, string, string, entity.Face) (bool, error) {
	return false, errGate
}

// A read verdict that fails stops the write with that error. It is never
// taken as "readable", and never reported as not found.
func TestWriteRequiresRead_GateErrorFailsClosed(t *testing.T) {
	t.Parallel()
	mgr, st := writeBlindManager(t, func(store.Store) acl.ACL { return erringACL{} })
	patch := entity.Patch{Properties: map[string]any{"title": "overwritten"}}
	checks := map[string]error{}
	_, checks["patch"] = mgr.PatchEntity(writer(), "DEC-1", patch)
	_, checks["delete"] = mgr.DeleteEntity(writer(), "DEC-1", true)
	_, checks["create relation"] = mgr.CreateRelation(writer(),
		entity.RelationKey{From: "api", Type: "depends-on", To: "db"}, entity.RelationOptions{})
	checks["delete relation"] = mgr.DeleteRelation(writer(),
		entity.RelationKey{From: "api", Type: "depends-on", To: "db"})
	for op, err := range checks {
		if !errors.Is(err, errGate) {
			t.Errorf("%s with a failing gate = %v, want the gate error", op, err)
		}
	}
	if got := decisionTitle(t, st); got != "secret" {
		t.Errorf("DEC-1 title = %q, want it untouched", got)
	}
}

// ghostPolicy: ghost may create relations from decisions without reading
// them, because a create grant needs no covering read grant.
const ghostPolicy = `
roles:
  ghost:
    read: [requirement, component]
    create: [decision]
assignments:
  ghost: ghost
`

// Under a real policy, a relation from a source the caller cannot read gets
// the answer a missing source gets. Before, the ACL saw the hidden source's
// type and allowed what it refuses for a missing one.
func TestWriteRequiresRead_DeclarativeHiddenSource(t *testing.T) {
	t.Parallel()
	mgr, _ := writeBlindManager(t, func(st store.Store) acl.ACL {
		p, err := acl.LoadPolicyBytes([]byte(ghostPolicy))
		if err != nil {
			t.Fatalf("load policy: %v", err)
		}
		d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
		if err != nil {
			t.Fatalf("NewDeclarative: %v", err)
		}
		return d
	})
	create := func(from string) error {
		_, err := mgr.CreateRelation(asUser("ghost"),
			entity.RelationKey{From: from, Type: "addresses", To: "REQ-1"}, entity.RelationOptions{})
		return err
	}
	sameAsMissing(t, "create", create("DEC-1"), create("DEC-404"), acl.ErrForbidden)
}
