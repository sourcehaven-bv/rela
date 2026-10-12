package appbuild

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const guardSchema = `version: "1.0"
entities:
  incident:
    label: Incident
    id_type: manual
    properties:
      title: {type: string}
      owner: {type: string}
    faces:
      draft: {}
      final: {}
`

const guardRealtime = `version: 1
channel_types:
  war-room:
    attach_to: [incident]
    on_entity_delete: block
    roles:
      owner: { from: entity.owner }
    message_types:
      note: { post: [owner] }
  chat:
    attach_to: [incident]
    roles:
      owner: { from: entity.owner }
    message_types:
      note: { post: [owner] }
  purge-me:
    attach_to: [incident]
    on_entity_delete: delete
    roles:
      owner: { from: entity.owner }
    message_types:
      note: { post: [owner] }
  notes:
    attach_to: [incident]
    anchor: face
    on_entity_delete: block
    roles:
      owner: { from: entity.owner }
    message_types:
      note: { post: [owner] }
`

type guardFixture struct {
	meta  *metamodel.Metamodel
	cfg   *realtimecfg.Compiled
	cs    channels.Store
	st    *memstore.MemStore
	guard *channelGuard
}

func newGuardFixture(t *testing.T) guardFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(guardSchema))
	require.NoError(t, err)
	cfg, err := realtimecfg.Parse([]byte(guardRealtime), meta, nil)
	require.NoError(t, err)
	_, cs, st := newChannelAliases(t)
	g := &channelGuard{store: cs, config: func() *realtimecfg.Compiled { return cfg }, now: time.Now}
	return guardFixture{meta: meta, cfg: cfg, cs: cs, st: st, guard: g}
}

// manager builds a manager over the fixture's store with the real guard and
// the real channel alias hook, as assemble wires them.
func (f guardFixture) manager(t *testing.T, g entitymanager.AnchorGuard) *entitymanager.Manager {
	t.Helper()
	aliases := channelAliasesFor(f.cs, f.st)
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: f.st, Meta: f.meta, Templater: gcNopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AliasRewriter: newAliasFanout(aliases), AnchorGuard: g,
	})
	require.NoError(t, err)
	return mgr
}

func (f guardFixture) seed(t *testing.T, id string, faces ...entity.Face) {
	t.Helper()
	for _, face := range faces {
		require.NoError(t, f.st.CreateEntity(context.Background(),
			&entity.Entity{ID: id, Type: "incident", Face: face, Properties: map[string]any{}}))
	}
}

func TestChannelGuard_BeforeDelete(t *testing.T) {
	whole := entitymanager.AnchorDelete{ID: "INC-1", Block: true, Detach: true}
	var blocked *entitymanager.AnchorBlockedError

	t.Run("block refuses and writes nothing", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		undo, err := f.guard.BeforeDelete(t.Context(), whole)
		require.ErrorAs(t, err, &blocked)
		assert.Equal(t, "war-room", blocked.ChannelType)
		assert.Nil(t, undo)
		assert.Equal(t, []string{"INC-1/war-room"}, entityKeys(t, f.cs, "INC-1"))
		require.ErrorAs(t, f.guard.CheckDelete(t.Context(), whole), &blocked)
	})

	t.Run("purge detaches a blocking channel", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		purge := whole
		purge.Purge = true
		undo, err := f.guard.BeforeDelete(t.Context(), purge)
		require.NoError(t, err)
		require.NotNil(t, undo)
		assert.Empty(t, entityKeys(t, f.cs, "INC-1"))
	})

	t.Run("detach, delete, unknown types and no config detach; undo restores", func(t *testing.T) {
		f := newGuardFixture(t)
		for _, typ := range []string{"chat", "purge-me", "gone-type"} {
			appendTo(t, f.cs, chanKey("INC-1", "", typ))
		}
		undo, err := f.guard.BeforeDelete(t.Context(), whole)
		require.NoError(t, err)
		assert.Empty(t, entityKeys(t, f.cs, "INC-1"))
		require.NoError(t, undo(t.Context()))
		assert.Len(t, entityKeys(t, f.cs, "INC-1"), 3)

		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		f.guard.config = func() *realtimecfg.Compiled { return nil }
		_, err = f.guard.BeforeDelete(t.Context(), whole)
		require.NoError(t, err, "without realtime.yaml nothing blocks")
		require.NoError(t, f.guard.CheckDelete(t.Context(), whole))
		assert.Empty(t, entityKeys(t, f.cs, "INC-1"))
	})

	t.Run("no channels", func(t *testing.T) {
		f := newGuardFixture(t)
		undo, err := f.guard.BeforeDelete(t.Context(), whole)
		require.NoError(t, err)
		assert.Nil(t, undo)
	})

	t.Run("face delete", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		appendTo(t, f.cs, chanKey("INC-1", "final", "notes"))

		// Not the caller's last face: the whole-entity block does not count,
		// and a face with no channels detaches nothing.
		undo, err := f.guard.BeforeDelete(t.Context(), entitymanager.AnchorDelete{ID: "INC-1", Face: "draft"})
		require.NoError(t, err)
		assert.Nil(t, undo)

		// The caller's last face: the whole-entity block counts.
		_, err = f.guard.BeforeDelete(t.Context(), entitymanager.AnchorDelete{ID: "INC-1", Face: "draft", Block: true})
		require.ErrorAs(t, err, &blocked)

		// A blocking channel on the face itself always counts.
		_, err = f.guard.BeforeDelete(t.Context(), entitymanager.AnchorDelete{ID: "INC-1", Face: "final"})
		require.ErrorAs(t, err, &blocked)
		assert.Equal(t, "notes", blocked.ChannelType)

		appendTo(t, f.cs, chanKey("INC-1", "draft", "chat"))
		undo, err = f.guard.BeforeDelete(t.Context(), entitymanager.AnchorDelete{ID: "INC-1", Face: "draft"})
		require.NoError(t, err)
		require.NotNil(t, undo)
		assert.Equal(t, []string{"INC-1/war-room", "INC-1@final/notes"}, entityKeys(t, f.cs, "INC-1"))
	})

	t.Run("a failing channel store fails the delete", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "chat"))
		f.guard.store = failingChannels{Store: f.cs}
		_, err := f.guard.BeforeDelete(t.Context(), whole)
		require.ErrorIs(t, err, errDiskFull)
	})
}

func TestChannelGuard_BeforeRename(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	calls := 0
	free := func(context.Context) error { calls++; return nil }
	require.NoError(t, f.guard.BeforeRename(ctx, "INC-2", free))
	assert.Zero(t, calls, "no channels on the new id: nothing to check")

	appendTo(t, f.cs, chanKey("INC-2", "", "chat"))
	taken := func(context.Context) error { return entitymanager.ErrEntityAlreadyExists }
	require.ErrorIs(t, f.guard.BeforeRename(ctx, "INC-2", taken), entitymanager.ErrEntityAlreadyExists)
	assert.Equal(t, []string{"INC-2/chat"}, entityKeys(t, f.cs, "INC-2"), "a taken id keeps its channels")

	require.NoError(t, f.guard.BeforeRename(ctx, "INC-2", free))
	assert.Equal(t, 1, calls)
	assert.Empty(t, entityKeys(t, f.cs, "INC-2"))

	require.Nil(t, channelAliasesFor(nil, f.st))
	_, nop := newChannelGuard(nil, nil).(entitymanager.NopAnchorGuard)
	assert.True(t, nop)
}

// A rename through the manager, with the real guard and alias hook, leaves
// the entity's channels reachable under the new id, and the channels an
// earlier entity left on the new id are not inherited.
func TestChannelGuard_RenameEndToEnd(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	mgr := f.manager(t, f.guard)
	f.seed(t, "inc-a", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "chat"))
	appendTo(t, f.cs, chanKey("inc-a", "draft", "notes"))
	appendTo(t, f.cs, chanKey("inc-b", "", "war-room"))

	_, err := mgr.RenameEntity(ctx, "inc-a", "inc-b", entity.RenameOptions{})
	require.NoError(t, err)
	assert.Empty(t, entityKeys(t, f.cs, "inc-a"))
	assert.Equal(t, []string{"inc-b/chat", "inc-b@draft/notes"}, entityKeys(t, f.cs, "inc-b"))
}

// postingGuard runs the real guard and then appends a channel, the way a
// first post landing between the guard and the commit would.
type postingGuard struct {
	*channelGuard
	t *testing.T
}

func (g postingGuard) BeforeDelete(
	ctx context.Context, d entitymanager.AnchorDelete,
) (func(context.Context) error, error) {
	undo, err := g.channelGuard.BeforeDelete(ctx, d)
	appendCtx(ctx, g.t, g.store, chanKey(d.ID, "", "war-room"))
	return undo, err
}

// A channel created after the guard ran is swept by the post-commit hook.
func TestChannelGuard_SweepAfterConcurrentFirstPost(t *testing.T) {
	f := newGuardFixture(t)
	mgr := f.manager(t, postingGuard{channelGuard: f.guard, t: t})
	f.seed(t, "inc-a", "draft")
	_, err := mgr.DeleteEntity(context.Background(), "inc-a", true)
	require.NoError(t, err)
	assert.Empty(t, entityKeys(t, f.cs, "inc-a"))
}

// A block refuses the delete through the manager and leaves the entity and
// its channels in place; a detach type lets it through and detaches.
func TestChannelGuard_ManagerDelete(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	mgr := f.manager(t, f.guard)
	f.seed(t, "inc-a", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "war-room"))

	_, err := mgr.DeleteEntity(ctx, "inc-a", true)
	var blocked *entitymanager.AnchorBlockedError
	require.ErrorAs(t, err, &blocked)
	_, gErr := f.st.GetEntity(ctx, entity.Ref{ID: "inc-a", Face: "draft"})
	require.NoError(t, gErr)
	assert.Equal(t, []string{"inc-a/war-room"}, entityKeys(t, f.cs, "inc-a"))

	f.seed(t, "inc-b", "draft")
	appendTo(t, f.cs, chanKey("inc-b", "", "chat"))
	_, err = mgr.DeleteEntity(ctx, "inc-b", true)
	require.NoError(t, err)
	assert.Empty(t, entityKeys(t, f.cs, "inc-b"))
}

func TestCheckChannelTypeRecords(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	require.NoError(t, checkChannelTypeRecords(ctx, nil, f.cfg))
	require.NoError(t, checkChannelTypeRecords(ctx, f.cs, nil))
	require.NoError(t, checkChannelTypeRecords(ctx, f.cs, f.cfg), "nothing recorded yet")

	matching := configuredTypeRecords(f.cfg)
	require.NoError(t, f.cs.RecordTypes(ctx, matching))
	require.NoError(t, checkChannelTypeRecords(ctx, f.cs, f.cfg))

	require.NoError(t, f.cs.RecordTypes(ctx, map[string]channels.TypeRecord{
		"chat":    {Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne},
		"notes":   {Anchor: channels.AnchorFace, Cardinality: channels.CardinalityMany},
		"retired": {Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne},
	}))
	before, err := f.cs.TypeRecords(ctx)
	require.NoError(t, err)
	err = checkChannelTypeRecords(ctx, f.cs, f.cfg)
	require.ErrorContains(t, err, "chat: anchor face")
	require.ErrorContains(t, err, "notes: anchor face and cardinality many")
	assert.NotContains(t, err.Error(), "retired", "a type the config dropped is not compared")
	after, err := f.cs.TypeRecords(ctx)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the check writes nothing")

	f.guard.store = failingRecords{Store: f.cs}
	require.ErrorIs(t, checkChannelTypeRecords(ctx, f.guard.store, f.cfg), errDiskFull)
}

type failingRecords struct{ channels.Store }

func (failingRecords) TypeRecords(context.Context) (map[string]channels.TypeRecord, error) {
	return nil, errDiskFull
}

// The reconciler detaches the channels of an entity removed outside the
// manager, and of a face removed while the family survives. A soft-deleted
// entity keeps its channels, and so does a family that is back.
func TestChannelReconciler(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	f.seed(t, "inc-a", "draft", "final")
	f.seed(t, "inc-b", "draft")
	f.seed(t, "inc-c", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "chat"))
	appendTo(t, f.cs, chanKey("inc-a", "final", "notes"))
	appendTo(t, f.cs, chanKey("inc-b", "", "chat"))
	appendTo(t, f.cs, chanKey("inc-c", "", "chat"))

	stop := startChannelReconciler(f.st, f.cs)
	t.Cleanup(stop)

	_, err := f.st.DeleteFace(ctx, entity.Ref{ID: "inc-a", Face: "final"})
	require.NoError(t, err)
	_, err = f.st.DeleteFamily(ctx, "inc-b", true)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return len(entityKeys(t, f.cs, "inc-b")) == 0 && len(entityKeys(t, f.cs, "inc-a")) == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"inc-a/chat"}, entityKeys(t, f.cs, "inc-a"))

	mgr := f.manager(t, entitymanager.NopAnchorGuard{})
	_, err = entitymanager.SoftDeleteEntity(ctx, mgr, "inc-c")
	require.NoError(t, err)
	r := channelReconciler{store: f.st, channels: f.cs, now: time.Now}
	require.NoError(t, r.reconcile(ctx, "inc-c", ""))
	assert.Equal(t, []string{"inc-c/chat"}, entityKeys(t, f.cs, "inc-c"), "a soft-deleted entity keeps its channels")

	require.NoError(t, r.reconcile(ctx, "inc-a", ""), "the family is still there")
	require.NoError(t, r.reconcile(ctx, "inc-a", "draft"), "the face is still there")
	assert.Equal(t, []string{"inc-a/chat"}, entityKeys(t, f.cs, "inc-a"))

	r.channels = failingChannels{Store: f.cs}
	_, err = f.st.DeleteFamily(ctx, "inc-a", true)
	require.NoError(t, err)
	require.ErrorIs(t, r.reconcile(ctx, "inc-a", ""), errDiskFull)
	r.handle(ctx, "inc-a", "") // logs

	stop()
	stop() // idempotent
	nilStop := startChannelReconciler(f.st, nil)
	nilStop()
}
