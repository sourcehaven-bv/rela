package entitymanager_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// fakeGuard is an [entitymanager.AnchorGuard] and its own
// [entitymanager.AnchorScope] over imaginary channels: every id has a
// channel unless noChannels is set; one blocking channel is anchored to the
// whole entity when blockWhole is set or the id is in blockIDs, and one on
// blockFace. Every detach returns an undo. It records every call.
//
// The zero value is a pass-through guard: it detaches everything and blocks
// nothing, so a manager over it runs every guard step.
type fakeGuard struct {
	mu         sync.Mutex
	blockWhole bool
	blockIDs   map[string]bool
	blockFace  entity.Face
	noChannels bool
	errIDs     map[string]bool // ids whose detach fails
	noDetach   bool            // BeforeDelete checks blocks but detaches nothing
	undoErr    error
	// onRestore runs at the end of each Restore, outside the lock: a write
	// that lands right after channels are put back.
	onRestore func()
	// onDelete runs at the end of each BeforeDelete, outside the lock: a
	// write that lands between the guard and the transaction.
	onDelete func(entitymanager.AnchorDelete)

	scopes      [][]string
	deletes     []entitymanager.AnchorDelete
	checks      []entitymanager.AnchorDelete
	undone      []string
	undoCtxErrs []error // ctx.Err() seen by each undo
	renames     []string
	detached    []string // ids BeforeRename cleared after free passed
}

func (g *fakeGuard) Active() bool { return true }

func (g *fakeGuard) Scope(_ context.Context, ids []string) (entitymanager.AnchorScope, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.scopes = append(g.scopes, ids)
	if g.noChannels {
		return nil, nil //nolint:nilnil // the interface's "no channels"
	}
	return g, nil
}

func (g *fakeGuard) blocks(d entitymanager.AnchorDelete) bool {
	if d.Purge {
		return false
	}
	whole := (g.blockWhole || g.blockIDs[d.ID]) && (d.Face == "" || d.Block)
	face := g.blockFace != "" && (d.Face == "" || d.Face == g.blockFace)
	return whole || face
}

func (g *fakeGuard) BeforeDelete(
	_ context.Context, d entitymanager.AnchorDelete,
) (entitymanager.AnchorUndo, error) {
	g.mu.Lock()
	g.deletes = append(g.deletes, d)
	blocked := g.blocks(d)
	failed := g.errIDs[d.ID]
	hook := g.onDelete
	g.mu.Unlock()
	if blocked {
		return nil, &entitymanager.AnchorBlockedError{ChannelType: "room"}
	}
	if failed {
		return nil, fmt.Errorf("channels: detach %s before delete: disk full", d.ID)
	}
	if hook != nil {
		hook(d)
	}
	if g.noDetach {
		return nil, nil //nolint:nilnil // the interface's "nothing detached"
	}
	return fakeUndo{g: g, ref: entity.FormatStateRef(d.ID, d.Face)}, nil
}

func (g *fakeGuard) CheckDelete(_ context.Context, d entitymanager.AnchorDelete) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.checks = append(g.checks, d)
	if g.blocks(d) {
		return &entitymanager.AnchorBlockedError{ChannelType: "room"}
	}
	return nil
}

func (g *fakeGuard) BeforeRename(
	ctx context.Context, newID string, free func(context.Context) error,
) (entitymanager.AnchorUndo, error) {
	g.mu.Lock()
	g.renames = append(g.renames, newID)
	g.mu.Unlock()
	if err := free(ctx); err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.detached = append(g.detached, newID)
	return fakeUndo{g: g, ref: "rename:" + newID}, nil
}

func (g *fakeGuard) undoneRefs() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.undone)
}

// fakeUndo records its restore in its guard.
type fakeUndo struct {
	g   *fakeGuard
	ref string
}

func (u fakeUndo) Restore(ctx context.Context) error {
	u.g.mu.Lock()
	u.g.undone = append(u.g.undone, u.ref)
	u.g.undoCtxErrs = append(u.g.undoCtxErrs, ctx.Err())
	err, hook := u.g.undoErr, u.g.onRestore
	u.g.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

func (u fakeUndo) String() string { return "tombstone:" + u.ref }

// failingTx fails every transaction, the way a backend whose commit fails
// would, without writing anything.
type failingTx struct{ store.Store }

var errTxFailed = errors.New("commit failed")

func (failingTx) Tx(context.Context, func(store.Store) error) error { return errTxFailed }

func newGuardedManager(t *testing.T, st store.Store, g entitymanager.AnchorGuard, sink audit.Audit) *entitymanager.Manager {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: parseMeta(t), Templater: nopTemplater{}, Audit: sink,
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{}, AnchorGuard: g,
	})
	require.NoError(t, err)
	return mgr
}

// seedAddressed stores DEC-1 addresses REQ-1.
func seedAddressed(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, st.CreateEntity(ctx, entity.New("DEC-1", "decision")))
	_, err := st.CreateRelation(ctx, entity.RelationKey{From: "DEC-1", Type: "addresses", To: "REQ-1"}, nil)
	require.NoError(t, err)
}

func deleteAuditCount(sink *audit.Memory) int {
	n := 0
	for _, r := range sink.Records() {
		if strings.HasPrefix(r.Op, "delete-") {
			n++
		}
	}
	return n
}

func TestNew_RejectsNilAnchorGuard(t *testing.T) {
	t.Parallel()
	_, err := entitymanager.New(entitymanager.Deps{
		Store: memstore.New(), Meta: parseMeta(t), Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	require.ErrorContains(t, err, "AnchorGuard")
}

// A block refuses the delete before anything is written, and the refusals
// the transaction would give come first, so the guard never acts for a
// delete that was not going to happen.
func TestAnchorGuard_BlockRefusesBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedAddressed(t, st)
	g := &fakeGuard{blockWhole: true}
	sink := audit.NewMemory()
	mgr := newGuardedManager(t, st, g, sink)
	ctx := context.Background()

	_, err := mgr.DeleteEntity(ctx, "REQ-1", false)
	require.ErrorIs(t, err, entitymanager.ErrHasRelations)
	assert.Empty(t, g.deletes, "the cascade refusal comes before the guard")

	_, err = mgr.DeleteEntity(ctx, "REQ-1", true)
	var blocked *entitymanager.AnchorBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, "room", blocked.ChannelType)
	_, gErr := st.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	require.NoError(t, gErr, "the entity survives")
	_, gErr = st.GetRelation(ctx, entity.RelationKey{From: "DEC-1", Type: "addresses", To: "REQ-1"})
	require.NoError(t, gErr, "its relation survives")
	assert.Zero(t, deleteAuditCount(sink), "nothing was written, so nothing is audited")
	assert.Equal(t, []entitymanager.AnchorDelete{{ID: "REQ-1", Block: true, Detach: true}}, g.deletes)
}

// A delete whose transaction fails puts back the channels the guard
// detached for it.
func TestAnchorGuard_TxFailureRestoresTheTombstone(t *testing.T) {
	t.Parallel()
	for _, undoErr := range []error{nil, errors.New("channel store down")} {
		mem := memstore.New()
		seedAddressed(t, mem)
		g := &fakeGuard{undoErr: undoErr}
		mgr := newGuardedManager(t, failingTx{mem}, g, audit.Nop{})

		_, err := mgr.DeleteEntity(context.Background(), "REQ-1", true)
		require.ErrorIs(t, err, errTxFailed, "a failed undo is logged; the delete's own error is reported")
		assert.Equal(t, []string{"REQ-1"}, g.undone)
	}
}

// A successful delete keeps the detach.
func TestAnchorGuard_SuccessKeepsTheDetach(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedAddressed(t, st)
	g := &fakeGuard{}
	mgr := newGuardedManager(t, st, g, audit.Nop{})
	_, err := mgr.DeleteEntity(context.Background(), "REQ-1", true)
	require.NoError(t, err)
	assert.Len(t, g.deletes, 1)
	assert.Empty(t, g.undone)
}

// A soft delete only checks: the rows stay restorable, so the channels stay.
// A purge decides nothing new, so a block detaches instead of refusing.
func TestAnchorGuard_SoftDeleteChecksAndPurgeDetaches(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedAddressed(t, st)
	g := &fakeGuard{blockWhole: true}
	mgr := newGuardedManager(t, st, g, audit.Nop{})
	ctx := aliceCtx()

	_, err := entitymanager.SoftDeleteEntity(ctx, mgr, "REQ-1")
	var blocked *entitymanager.AnchorBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Empty(t, g.deletes, "a soft delete never detaches")
	_, gErr := st.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	require.NoError(t, gErr)

	g.blockWhole = false
	_, err = entitymanager.SoftDeleteEntity(ctx, mgr, "REQ-1")
	require.NoError(t, err)
	assert.Empty(t, g.deletes)

	g.blockWhole = true
	n, err := entitymanager.PurgeSoftDeleted(context.Background(), mgr, time.Now().Add(time.Second))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, g.deletes, 1)
	assert.True(t, g.deletes[0].Purge)
}

// Renaming onto the id of a soft-deleted entity is refused, and that
// entity's channels stay where they are: the guard asks free first.
func TestAnchorGuard_RenameOntoSoftDeletedKeepsItsChannels(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	ctx := aliceCtx()
	for _, id := range []string{"auth", "billing", "live"} {
		require.NoError(t, st.CreateEntity(ctx, entity.New(id, "component")))
	}
	g := &fakeGuard{}
	mgr := newGuardedManager(t, st, g, audit.Nop{})
	_, err := entitymanager.SoftDeleteEntity(ctx, mgr, "billing")
	require.NoError(t, err)

	_, err = mgr.RenameEntity(ctx, "auth", "billing", entity.RenameOptions{})
	require.ErrorIs(t, err, entitymanager.ErrEntityAlreadyExists)
	assert.Empty(t, g.detached, "the soft-deleted entity keeps its channels")

	// Case folding matches the stores' rule.
	_, err = mgr.RenameEntity(ctx, "auth", "Billing", entity.RenameOptions{})
	require.ErrorIs(t, err, entitymanager.ErrEntityAlreadyExists)
	assert.Empty(t, g.detached)

	// A live occupant is refused the same way.
	_, err = mgr.RenameEntity(ctx, "auth", "live", entity.RenameOptions{})
	require.ErrorIs(t, err, entitymanager.ErrEntityAlreadyExists)
	assert.Empty(t, g.detached)

	// A dry run consults nothing; a free id is cleared and the rename runs.
	_, err = mgr.RenameEntity(ctx, "auth", "gateway", entity.RenameOptions{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"billing", "Billing", "live"}, g.renames)
	_, err = mgr.RenameEntity(ctx, "auth", "gateway", entity.RenameOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"gateway"}, g.detached)

	// A case-only rename of the entity itself is not refused by its own id.
	_, err = mgr.RenameEntity(ctx, "gateway", "Gateway", entity.RenameOptions{})
	require.NoError(t, err)

	// A malformed id never reaches the guard.
	_, err = mgr.RenameEntity(ctx, "Gateway", "bad id", entity.RenameOptions{})
	require.Error(t, err)
	assert.Len(t, g.renames, 5)
}

// A face delete decides a block on the faces the caller can read, so a
// hidden sibling face changes nothing about the answer. Whether the entity
// really goes is decided on the stored faces, and only that controls the
// detach.
func TestAnchorGuard_FaceDeleteIgnoresHiddenSibling(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			withSibling := &fakeGuard{blockWhole: true}
			f := newFaceEdgeFixtureWith(t, b, withSibling)
			_, errWith := f.mgr.DeleteEntityFace(asUser("blind-admin"), "POL-1", "draft", true)

			without := &fakeGuard{blockWhole: true}
			f2 := newFaceEdgeFixtureWith(t, b, without)
			_, err := f2.st.DeleteFace(ctx, entity.Ref{ID: "POL-1", Face: "published"})
			require.NoError(t, err)
			_, errWithout := f2.mgr.DeleteEntityFace(asUser("blind-admin"), "POL-1", "draft", true)

			var blocked *entitymanager.AnchorBlockedError
			require.ErrorAs(t, errWith, &blocked)
			require.ErrorAs(t, errWithout, &blocked)
			require.Len(t, withSibling.deletes, 1)
			require.Len(t, without.deletes, 1)
			assert.True(t, withSibling.deletes[0].Block)
			assert.False(t, withSibling.deletes[0].Detach, "the hidden face is still stored")
			assert.True(t, without.deletes[0].Detach)
			assert.Zero(t, f.deleteRecords())

			// A caller who reads both faces is not deleting its last one, so a
			// channel on the whole entity does not block.
			admin := &fakeGuard{blockWhole: true}
			f3 := newFaceEdgeFixtureWith(t, b, admin)
			_, err = f3.mgr.DeleteEntityFace(asUser("admin"), "POL-1", "draft", true)
			require.NoError(t, err)
			assert.Equal(t, []entitymanager.AnchorDelete{{ID: "POL-1", Face: "draft"}}, admin.deletes)

			// A channel on the face itself always blocks.
			faceBlock := &fakeGuard{blockFace: "draft"}
			f4 := newFaceEdgeFixtureWith(t, b, faceBlock)
			_, err = f4.mgr.DeleteEntityFace(asUser("admin"), "POL-1", "draft", true)
			require.ErrorAs(t, err, &blocked)
		})
	}
}

// A face delete whose transaction fails restores that face's channels.
func TestAnchorGuard_FaceDeleteTxFailureRestores(t *testing.T) {
	t.Parallel()
	mem := memstore.New()
	ctx := context.Background()
	require.NoError(t, mem.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
	require.NoError(t, mem.CreateEntity(ctx,
		&entity.Entity{ID: "REQ-1", Type: "requirement", Face: "published", Properties: map[string]any{}}))
	g := &fakeGuard{}
	mgr := newGuardedManager(t, failingTx{mem}, g, audit.Nop{})
	_, err := mgr.DeleteEntityFace(ctx, "REQ-1", "published", true)
	require.ErrorIs(t, err, errTxFailed)
	assert.Equal(t, []string{"REQ-1@published"}, g.undone)
}

// anchorGuardOf is the anchor guard a fixture over b wires: a pass-through
// fakeGuard when b.guarded, so the manager runs the guard's authorization
// pre-pass, and NopAnchorGuard otherwise.
func anchorGuardOf(b concBackend) entitymanager.AnchorGuard {
	if b.guarded {
		return &fakeGuard{}
	}
	return entitymanager.NopAnchorGuard{}
}

// withGuard is b with a pass-through anchor guard.
func withGuard(b concBackend) concBackend {
	b.name += "+guard"
	b.guarded = true
	return b
}

// deleteACLBackends is concBackends twice: as is, and with a pass-through
// anchor guard. The delete ACL tables run over it, so their refusals are
// pinned with and without the guard's pre-pass in front of the transaction.
func deleteACLBackends() []concBackend {
	out := slices.Clone(concBackends)
	for _, b := range concBackends {
		out = append(out, withGuard(b))
	}
	return out
}

// deleteGuards are the two guards the single-store delete ACL tests run
// under.
var deleteGuards = []struct {
	name  string
	guard func() entitymanager.AnchorGuard
}{
	{"nop", func() entitymanager.AnchorGuard { return entitymanager.NopAnchorGuard{} }},
	{"guard", func() entitymanager.AnchorGuard { return &fakeGuard{} }},
}
