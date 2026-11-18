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
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// fakeCreateHook records what the manager told it.
type fakeCreateHook struct {
	typ    string
	err    error
	seen   []string
	inTx   bool
	viewOK bool
	after  int
}

func (h *fakeCreateHook) Watches(typ string) bool { return typ == h.typ }

func (h *fakeCreateHook) EntityCreated(ctx context.Context, view store.Store, e *entity.Entity,
) (func(context.Context), error) {
	h.seen = append(h.seen, e.ID)
	h.inTx = store.InTx(ctx)
	_, gErr := view.GetEntity(ctx, e.Ref())
	h.viewOK = gErr == nil
	if h.err != nil {
		return nil, h.err
	}
	return func(context.Context) { h.after++ }, nil
}

func newManagerWithHook(t *testing.T, hook entitymanager.CreateHook) *entitymanager.Manager {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       memstore.New(),
		Meta:        parseMeta(t),
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: transitions.NoTransitions(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
		CreateHook:  hook,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

// TestCreateHook pins the manager's side of FEAT-J0O10QI: a watched create
// runs the hook in a transaction that already holds the row, the follow-up
// runs once after the commit, an unwatched type is left alone, and a hook
// error fails the create.
func TestCreateHook(t *testing.T) {
	errHook := errors.New("hook failed")
	tests := []struct {
		name      string
		hookType  string
		hookErr   error
		wantSeen  int
		wantAfter int
		wantErr   error
	}{
		{name: "watched", hookType: "requirement", wantSeen: 1, wantAfter: 1},
		{name: "not watched", hookType: "decision"},
		{name: "hook error fails the create", hookType: "requirement", hookErr: errHook,
			wantSeen: 1, wantErr: errHook},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hook := &fakeCreateHook{typ: tc.hookType, err: tc.hookErr}
			mgr := newManagerWithHook(t, hook)
			e := entity.New("", "requirement")
			e.SetString("title", "a")
			_, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateEntity error = %v, want %v", err, tc.wantErr)
			}
			if len(hook.seen) != tc.wantSeen {
				t.Fatalf("hook saw %d creates, want %d", len(hook.seen), tc.wantSeen)
			}
			if hook.after != tc.wantAfter {
				t.Fatalf("follow-up ran %d times, want %d", hook.after, tc.wantAfter)
			}
			if tc.wantSeen > 0 && (!hook.inTx || !hook.viewOK) {
				t.Fatalf("hook ran in tx = %v, saw its row = %v; want both", hook.inTx, hook.viewOK)
			}
		})
	}
}

// txNotes records, per created id, whether its create notification ran
// inside a store transaction.
type txNotes struct{ inTx map[string]bool }

func (n *txNotes) EntityRenamed(context.Context, string, string) error { return nil }
func (n *txNotes) EntityDeleted(context.Context, string) error         { return nil }
func (n *txNotes) EntityFaceDeleted(context.Context, string, entity.Face) error {
	return nil
}

func (n *txNotes) EntityCreated(ctx context.Context, id string, _ entity.Face) error {
	n.inTx[id] = store.InTx(ctx)
	return nil
}

// linkingHook creates a decision addressing each new requirement, inside
// the requirement's transaction, as the flow runtime writes a run header.
type linkingHook struct {
	mgr     *entitymanager.Manager
	created string
}

func (h *linkingHook) Watches(typ string) bool { return typ == "requirement" }

func (h *linkingHook) EntityCreated(ctx context.Context, _ store.Store, e *entity.Entity,
) (func(context.Context), error) {
	d := entity.New("", "decision")
	d.SetString("title", "d")
	res, err := entitymanager.CreateLinked(ctx, h.mgr, entitymanager.LinkedCreate{
		Entity: d, RelateType: "addresses", RelateTo: e.ID,
	})
	if err != nil {
		return nil, err
	}
	h.created = res.Create.Entity.ID
	return nil, nil
}

// TestCreateHook_NestedCreateFollowsUpAfterCommit: a linked create made
// inside another create's transaction runs its follow-ups once that
// transaction commits, not inside it.
func TestCreateHook_NestedCreateFollowsUpAfterCommit(t *testing.T) {
	notes := &txNotes{inTx: map[string]bool{}}
	hook := &linkingHook{}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:         memstore.New(),
		Meta:          parseMeta(t),
		Templater:     nopTemplater{},
		Audit:         audit.Nop{},
		ACL:           acl.NopACL{},
		Transitions:   transitions.NoTransitions(),
		FieldGate:     entitymanager.AllowAllFieldGate{},
		AnchorGuard:   entitymanager.NopAnchorGuard{},
		AliasRewriter: notes,
		CreateHook:    hook,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	hook.mgr = mgr
	r := entity.New("", "requirement")
	r.SetString("title", "r")
	if _, err := mgr.CreateEntity(context.Background(), r, entity.CreateOptions{}); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	inTx, ok := notes.inTx[hook.created]
	if !ok {
		t.Fatalf("the nested create %q was never followed up", hook.created)
	}
	if inTx {
		t.Fatalf("the nested create was followed up inside the transaction")
	}
}

// TestCreateLinked_InTxEndpointKeepsReadGate: a linked create nested in a
// transaction reads its endpoint on that transaction's view, so it sees a
// row the transaction created, and the read gate still refuses one the
// caller cannot read, as it refuses a missing one.
func TestCreateLinked_InTxEndpointKeepsReadGate(t *testing.T) {
	mgr, st := writeBlindManager(t, func(store.Store) acl.ACL { return hideOneACL{hidden: "REQ-9"} })
	link := func(ctx context.Context, to string) error {
		d := entity.New("", "decision")
		d.SetString("title", "d")
		_, err := entitymanager.CreateLinked(ctx, mgr, entitymanager.LinkedCreate{
			Entity: d, RelateType: "addresses", RelateTo: to,
		})
		return err
	}
	var visible, hidden, missing error
	err := st.Tx(writer(), func(view store.Store) error {
		ctx := store.ContextWithTxView(writer(), view)
		for _, id := range []string{"REQ-8", "REQ-9"} {
			r := entity.New(id, "requirement")
			r.SetString("title", id)
			if err := view.CreateEntity(ctx, r); err != nil {
				return err
			}
		}
		visible, hidden, missing = link(ctx, "REQ-8"), link(ctx, "REQ-9"), link(ctx, "REQ-404")
		return nil
	})
	if err != nil {
		t.Fatalf("Tx: %v", err)
	}
	if visible != nil {
		t.Fatalf("link to a readable row created in the transaction: %v", visible)
	}
	if hidden == nil {
		t.Fatal("link to a hidden row created in the transaction succeeded")
	}
	if got, want := strings.ReplaceAll(hidden.Error(), "REQ-9", "REQ-404"), missing.Error(); got != want {
		t.Fatalf("hidden row answers %q, missing row %q", got, want)
	}
}
