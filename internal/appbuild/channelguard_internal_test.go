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
	"github.com/Sourcehaven-BV/rela/internal/store"
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

// guardDelete runs BeforeDelete in a scope of its own for d.ID, as the
// manager does for a delete of one entity.
func guardDelete(
	ctx context.Context, g *channelGuard, d entitymanager.AnchorDelete,
) (entitymanager.AnchorUndo, error) {
	s, err := g.Scope(ctx, []string{d.ID})
	if err != nil || s == nil {
		return nil, err
	}
	return s.BeforeDelete(ctx, d)
}

// guardCheck is guardDelete for CheckDelete.
func guardCheck(ctx context.Context, g *channelGuard, d entitymanager.AnchorDelete) error {
	s, err := g.Scope(ctx, []string{d.ID})
	if err != nil || s == nil {
		return err
	}
	return s.CheckDelete(ctx, d)
}

func TestChannelGuard_BeforeDelete(t *testing.T) {
	whole := entitymanager.AnchorDelete{ID: "INC-1", Block: true, Detach: true}
	var blocked *entitymanager.AnchorBlockedError

	t.Run("block refuses and writes nothing", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		undo, err := guardDelete(t.Context(), f.guard, whole)
		require.ErrorAs(t, err, &blocked)
		assert.Equal(t, "war-room", blocked.ChannelType)
		assert.Nil(t, undo)
		assert.Equal(t, []string{"INC-1/war-room"}, entityKeys(t, f.cs, "INC-1"))
		require.ErrorAs(t, guardCheck(t.Context(), f.guard, whole), &blocked)
	})

	t.Run("purge detaches a blocking channel", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		purge := whole
		purge.Purge = true
		undo, err := guardDelete(t.Context(), f.guard, purge)
		require.NoError(t, err)
		require.NotNil(t, undo)
		assert.Empty(t, entityKeys(t, f.cs, "INC-1"))
	})

	t.Run("detach, delete, unknown types and no config detach; undo restores", func(t *testing.T) {
		f := newGuardFixture(t)
		for _, typ := range []string{"chat", "purge-me", "gone-type"} {
			appendTo(t, f.cs, chanKey("INC-1", "", typ))
		}
		undo, err := guardDelete(t.Context(), f.guard, whole)
		require.NoError(t, err)
		assert.Empty(t, entityKeys(t, f.cs, "INC-1"))
		require.NoError(t, undo.Restore(t.Context()))
		assert.Len(t, entityKeys(t, f.cs, "INC-1"), 3)

		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		f.guard.config = func() *realtimecfg.Compiled { return nil }
		_, err = guardDelete(t.Context(), f.guard, whole)
		require.NoError(t, err, "without realtime.yaml nothing blocks")
		require.NoError(t, guardCheck(t.Context(), f.guard, whole))
		assert.Empty(t, entityKeys(t, f.cs, "INC-1"))
	})

	t.Run("no channels", func(t *testing.T) {
		f := newGuardFixture(t)
		undo, err := guardDelete(t.Context(), f.guard, whole)
		require.NoError(t, err)
		assert.Nil(t, undo)
	})

	t.Run("face delete", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
		appendTo(t, f.cs, chanKey("INC-1", "final", "notes"))

		// Not the caller's last face: the whole-entity block does not count,
		// and a face with no channels detaches nothing.
		undo, err := guardDelete(t.Context(), f.guard, entitymanager.AnchorDelete{ID: "INC-1", Face: "draft"})
		require.NoError(t, err)
		assert.Nil(t, undo)

		// The caller's last face: the whole-entity block counts.
		_, err = guardDelete(t.Context(), f.guard, entitymanager.AnchorDelete{ID: "INC-1", Face: "draft", Block: true})
		require.ErrorAs(t, err, &blocked)

		// A blocking channel on the face itself always counts.
		_, err = guardDelete(t.Context(), f.guard, entitymanager.AnchorDelete{ID: "INC-1", Face: "final"})
		require.ErrorAs(t, err, &blocked)
		assert.Equal(t, "notes", blocked.ChannelType)

		appendTo(t, f.cs, chanKey("INC-1", "draft", "chat"))
		undo, err = guardDelete(t.Context(), f.guard, entitymanager.AnchorDelete{ID: "INC-1", Face: "draft"})
		require.NoError(t, err)
		require.NotNil(t, undo)
		assert.Equal(t, []string{"INC-1/war-room", "INC-1@final/notes"}, entityKeys(t, f.cs, "INC-1"))
	})

	t.Run("a failing channel store fails the delete", func(t *testing.T) {
		f := newGuardFixture(t)
		appendTo(t, f.cs, chanKey("INC-1", "", "chat"))
		f.guard.store = failingChannels{Store: f.cs}
		_, err := guardDelete(t.Context(), f.guard, whole)
		require.ErrorIs(t, err, errDiskFull)
	})
}

func TestChannelGuard_BeforeRename(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	calls := 0
	free := func(context.Context) error { calls++; return nil }
	undo, err := f.guard.BeforeRename(ctx, "INC-2", free)
	require.NoError(t, err)
	assert.Nil(t, undo)
	assert.Zero(t, calls, "no channels on the new id: nothing to check")

	appendTo(t, f.cs, chanKey("INC-2", "", "chat"))
	taken := func(context.Context) error { return entitymanager.ErrEntityAlreadyExists }
	_, err = f.guard.BeforeRename(ctx, "INC-2", taken)
	require.ErrorIs(t, err, entitymanager.ErrEntityAlreadyExists)
	assert.Equal(t, []string{"INC-2/chat"}, entityKeys(t, f.cs, "INC-2"), "a taken id keeps its channels")

	undo, err = f.guard.BeforeRename(ctx, "INC-2", free)
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Empty(t, entityKeys(t, f.cs, "INC-2"))

	// A failed rename puts them back.
	require.NotNil(t, undo)
	_, pErr := channels.ParseTombstoneHandle(undo.String())
	require.NoError(t, pErr, "the undo names its batch")
	require.NoError(t, undo.Restore(ctx))
	assert.Equal(t, []string{"INC-2/chat"}, entityKeys(t, f.cs, "INC-2"))

	require.Nil(t, channelAliasesFor(nil, f.st))
	_, nop := newChannelGuard(nil, nil).(entitymanager.NopAnchorGuard)
	assert.True(t, nop)
	assert.True(t, f.guard.Active())
}

// A rename whose transaction fails, through the manager, leaves the
// channels an earlier entity left on the new id where they were.
func TestChannelGuard_RenameTxFailureRestores(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	f.seed(t, "inc-a", "draft")
	appendTo(t, f.cs, chanKey("inc-b", "", "chat"))
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: failingTxStore{f.st}, Meta: f.meta, Templater: gcNopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AnchorGuard: f.guard,
	})
	require.NoError(t, err)
	_, err = mgr.RenameEntity(ctx, "inc-a", "inc-b", entity.RenameOptions{})
	require.ErrorIs(t, err, errDiskFull)
	assert.Equal(t, []string{"inc-b/chat"}, entityKeys(t, f.cs, "inc-b"))
}

// failingTxStore fails every transaction without writing.
type failingTxStore struct{ store.Store }

func (failingTxStore) Tx(context.Context, func(store.Store) error) error { return errDiskFull }

// partialTombstone moves the batch and then fails, as the file backend does
// when it moved the channels but could not write the audit record.
type partialTombstone struct{ channels.Store }

func (p partialTombstone) Tombstone(
	ctx context.Context, id string, face entity.Face, at time.Time, f channels.Filing,
) (channels.TombstoneHandle, error) {
	h, err := p.Store.Tombstone(ctx, id, face, at, f)
	if err != nil {
		return h, err
	}
	return h, errDiskFull
}

// A detach that fails part way is put back before the error is returned, on
// delete and on rename alike, so a failed guard leaves nothing detached.
func TestChannelGuard_PartialTombstoneIsRestored(t *testing.T) {
	f := newGuardFixture(t)
	appendTo(t, f.cs, chanKey("INC-1", "", "chat"))
	f.guard.store = partialTombstone{Store: f.cs}

	_, err := guardDelete(t.Context(), f.guard, entitymanager.AnchorDelete{ID: "INC-1", Block: true, Detach: true})
	require.ErrorIs(t, err, errDiskFull)
	assert.Equal(t, []string{"INC-1/chat"}, entityKeys(t, f.cs, "INC-1"))

	_, err = f.guard.BeforeRename(t.Context(), "INC-1", func(context.Context) error { return nil })
	require.ErrorIs(t, err, errDiskFull)
	assert.Equal(t, []string{"INC-1/chat"}, entityKeys(t, f.cs, "INC-1"))
}

// One scope loads the config once, however many entities it judges, and
// its errors name no id.
func TestChannelGuard_ScopeLoadsConfigOnce(t *testing.T) {
	f := newGuardFixture(t)
	appendTo(t, f.cs, chanKey("INC-1", "", "chat"))
	appendTo(t, f.cs, chanKey("INC-2", "", "chat"))
	loads := 0
	cfg := f.cfg
	f.guard.config = func() *realtimecfg.Compiled { loads++; return cfg }

	s, err := f.guard.Scope(t.Context(), []string{"INC-1", "INC-2", "INC-3"})
	require.NoError(t, err)
	require.NotNil(t, s)
	for _, id := range []string{"INC-1", "INC-2", "INC-3"} {
		_, err = s.BeforeDelete(t.Context(), entitymanager.AnchorDelete{ID: id, Block: true, Detach: true})
		require.NoError(t, err)
	}
	assert.Equal(t, 1, loads)

	empty, err := f.guard.Scope(t.Context(), []string{"INC-1", "INC-3"})
	require.NoError(t, err)
	assert.Nil(t, empty, "no channels left: nothing to guard")

	f.guard.store = failingList{Store: f.cs}
	_, err = f.guard.Scope(t.Context(), []string{"INC-1", "SECRET-9"})
	require.ErrorIs(t, err, errDiskFull)
	assert.NotContains(t, err.Error(), "SECRET-9")
}

// failingList fails every channel listing.
type failingList struct{ channels.Store }

func (failingList) ListEntity(context.Context, string) ([]channels.Channel, error) {
	return nil, errDiskFull
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

func (g postingGuard) Scope(ctx context.Context, ids []string) (entitymanager.AnchorScope, error) {
	s, err := g.channelGuard.Scope(ctx, ids)
	if err != nil || s == nil {
		return s, err
	}
	return postingScope{AnchorScope: s, g: g}, nil
}

type postingScope struct {
	entitymanager.AnchorScope
	g postingGuard
}

func (s postingScope) BeforeDelete(
	ctx context.Context, d entitymanager.AnchorDelete,
) (entitymanager.AnchorUndo, error) {
	undo, err := s.AnchorScope.BeforeDelete(ctx, d)
	appendCtx(ctx, s.g.t, s.g.store, chanKey(d.ID, "", "war-room"))
	return undo, err
}

// A channel created after the guard ran is swept by the post-commit hook.
func TestChannelGuard_SweepAfterConcurrentFirstPost(t *testing.T) {
	f := newGuardFixture(t)
	mgr := f.manager(t, postingGuard{channelGuard: f.guard, t: t})
	f.seed(t, "inc-a", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "chat"))
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

	matching := map[string]channels.TypeRecord{}
	for _, ct := range f.cfg.ChannelTypes() {
		matching[ct.Name] = channels.TypeRecord{
			Anchor: channels.AnchorMode(ct.Anchor), Cardinality: channels.Cardinality(ct.Cardinality),
		}
	}
	require.NoError(t, f.cs.RecordTypes(ctx, matching))
	require.NoError(t, checkChannelTypeRecords(ctx, f.cs, f.cfg))

	// RecordTypes refuses to change a recorded shape, so the mismatched
	// records go into a fresh store.
	f = newGuardFixture(t)
	require.NoError(t, f.cs.RecordTypes(ctx, map[string]channels.TypeRecord{
		"chat":    {Anchor: channels.AnchorFace, Cardinality: channels.CardinalityOne},
		"notes":   {Anchor: channels.AnchorFace, Cardinality: channels.CardinalityMany},
		"retired": {Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne},
	}))
	before, err := f.cs.TypeRecords(ctx)
	require.NoError(t, err)
	err = checkChannelTypeRecords(ctx, f.cs, f.cfg)
	require.ErrorContains(t, err, "channel_types.chat.anchor: cannot change from face to family")
	require.ErrorContains(t, err, "channel_types.notes.cardinality: cannot change from many to one")
	assert.NotContains(t, err.Error(), "retired", "a type the config dropped is not compared")
	after, err := f.cs.TypeRecords(ctx)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the check writes nothing")

	// A record with an unknown cardinality (written before cardinality was
	// kept) matches the configured one; its anchor is still compared.
	err = checkChannelTypeRecords(ctx, legacyCardinality{Store: f.cs}, f.cfg)
	require.ErrorContains(t, err, "channel_types.chat.anchor: cannot change from face to family")
	assert.NotContains(t, err.Error(), "cardinality", "an unknown cardinality is not a change")
	ok := newGuardFixture(t)
	require.NoError(t, ok.cs.RecordTypes(ctx, matching))
	require.NoError(t, checkChannelTypeRecords(ctx, legacyCardinality{Store: ok.cs}, ok.cfg))

	f.guard.store = failingRecords{Store: f.cs}
	require.ErrorIs(t, checkChannelTypeRecords(ctx, f.guard.store, f.cfg), errDiskFull)
}

// legacyCardinality reads every record with an unknown cardinality.
type legacyCardinality struct{ channels.Store }

func (l legacyCardinality) TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error) {
	recs, err := l.Store.TypeRecords(ctx)
	for typ, r := range recs {
		r.Cardinality = channels.CardinalityUnknown
		recs[typ] = r
	}
	return recs, err
}

type failingRecords struct{ channels.Store }

func (failingRecords) TypeRecords(context.Context) (map[string]channels.TypeRecord, error) {
	return nil, errDiskFull
}

// The channels of an entity deleted outside the manager stay live: nothing
// detaches them, and a channel request finds no anchor. A create at the id
// through the manager tombstones them, so the new entity starts without
// them.
func TestChannelAliases_ManagerCreateClearsExternalLeftovers(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	f.seed(t, "inc-a", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "war-room"))
	appendTo(t, f.cs, chanKey("inc-a", "draft", "notes"))

	_, err := f.st.DeleteFamily(ctx, "inc-a", true)
	require.NoError(t, err)
	assert.Len(t, entityKeys(t, f.cs, "inc-a"), 2, "an external delete leaves the channels live")

	mgr := f.manager(t, f.guard)
	_, err = mgr.CreateEntity(ctx,
		&entity.Entity{Type: "incident", Properties: map[string]any{}},
		entity.CreateOptions{ID: "inc-a", Face: "draft"})
	require.NoError(t, err)
	assert.Empty(t, entityKeys(t, f.cs, "inc-a"), "the new entity does not inherit them")
}
