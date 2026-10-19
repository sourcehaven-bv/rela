//go:build !postgres && !memorybackend && !sqlite

package appbuild_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const serviceRealtimeYAML = `version: 1
channel_types:
  room:
    attach_to: [doc, memo]
    roles:
      owner: { from: entity.owner }
    message_types:
      note:
        properties:
          body: { type: string }
          about: { type: ref, to: [doc] }
        post: [owner]
`

// serviceSchema declares a required title, so a ref renders it, and a memo
// type, so a policy can let a caller read one type and not the other.
const serviceSchema = `version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      owner: {type: string}
  memo:
    label: Memo
    id_prefix: "MEMO-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      owner: {type: string}
`

// serviceFixture assembles a project with realtime.yaml over a memstore
// holding DOC-1 (owner alice), DOC-2 (owner bob) and MEMO-1 (owner bob). files may
// name its own realtime.yaml.
func serviceFixture(t *testing.T, files map[string]string, opts ...appbuild.Option) *appbuild.Services {
	t.Helper()
	if _, ok := files["realtime.yaml"]; !ok {
		files["realtime.yaml"] = serviceRealtimeYAML
	}
	files["schema.yaml"] = serviceSchema
	base, err := realtimeBase(t, realtimeProject(t, files), opts...)
	require.NoError(t, err)
	st := memstore.New()
	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "DOC-1", Type: "doc",
		Properties: map[string]any{"title": "One", "owner": "alice"}}))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "DOC-2", Type: "doc",
		Properties: map[string]any{"title": "Two", "owner": "bob"}}))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "MEMO-1", Type: "memo",
		Properties: map[string]any{"title": "Memo", "owner": "bob"}}))
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func asUser(name string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: name, Tool: principal.ToolDataEntry})
}

var doc1Room = realtime.RawAddress{EntityType: "doc", Anchor: "DOC-1", Type: "room"}

func note(body string) realtime.PostInput {
	return realtime.PostInput{Type: "note", Props: map[string]any{"body": body}}
}

func requireStatus(t *testing.T, err error, status int) {
	t.Helper()
	d, ok := errors.AsType[*realtime.DenialError](err)
	require.Truef(t, ok, "want a *DenialError, got %v", err)
	require.Equal(t, status, d.Status, d.Error())
}

// TestChannelService_Declarative pins the wiring under acl.yaml: the
// anchor read is row-gated (a caller who cannot read the entity gets
// ErrAnchorNotFound), entity-property roles read the gated anchor, and a
// ref the caller cannot read is a 422.
func TestChannelService_Declarative(t *testing.T) {
	t.Parallel()
	svc := serviceFixture(t, map[string]string{"acl.yaml": `roles:
  reader:
    read: ["*"]
  memos:
    read: [memo]
assignments:
  alice: reader
  bob: memos
`})
	cs := appbuild.ChannelService(svc)
	require.NotNil(t, cs)
	as := func(name string) context.Context { return bindAt(asUser(name), t, svc) }

	// A context the edge did not bind fails closed.
	_, err := cs.Head(asUser("alice"), doc1Room)
	requireStatus(t, err, http.StatusForbidden)

	res, err := cs.Post(as("alice"), doc1Room, note("hi"))
	require.NoError(t, err)
	require.EqualValues(t, 1, res.Message.Seq)

	_, err = cs.Head(as("bob"), doc1Room)
	require.ErrorIs(t, err, realtime.ErrAnchorNotFound)
	_, err = cs.Head(as("carol"), doc1Room)
	require.ErrorIs(t, err, realtime.ErrAnchorNotFound)

	memo := realtime.RawAddress{EntityType: "memo", Anchor: "MEMO-1", Type: "room"}
	_, err = cs.Post(as("bob"), memo, note("mine"))
	require.NoError(t, err)
	_, err = cs.Post(as("bob"), memo, realtime.PostInput{Type: "note", Props: map[string]any{"about": "DOC-1"}})
	requireStatus(t, err, http.StatusUnprocessableEntity)

	got, err := cs.Post(as("alice"), doc1Room,
		realtime.PostInput{Type: "note", Props: map[string]any{"about": "DOC-2"}})
	require.NoError(t, err)
	require.Equal(t, &realtime.RefValue{ID: "DOC-2", Type: "doc", Title: "Two"}, got.Message.Props["about"])

	_, err = cs.Post(as("alice"), realtime.RawAddress{EntityType: "doc", Anchor: "DOC-2", Type: "room"}, note("x"))
	requireStatus(t, err, http.StatusForbidden)
}

// conditionRealtimeYAML offers close on a doc room when the doc's title is
// One.
const conditionRealtimeYAML = `version: 1
channel_types:
  room:
    attach_to: [doc]
    roles:
      member: { from: attached }
    message_types:
      note: { properties: { body: { type: string } }, post: [member] }
    lifecycle:
      close: { roles: [member], condition: "entity.title == 'One'", denied: "Only One closes" }
`

// TestChannelService_LifecycleConditionReadsAnchor pins the wiring of a
// lifecycle condition: the anchor reader binds the condition's record from
// the gated read, so the condition decides per entity.
func TestChannelService_LifecycleConditionReadsAnchor(t *testing.T) {
	t.Parallel()
	svc := serviceFixture(t, map[string]string{"realtime.yaml": conditionRealtimeYAML}, appbuild.WithACL(acl.NopACL{}))
	cs := appbuild.ChannelService(svc)
	doc2Room := realtime.RawAddress{EntityType: "doc", Anchor: "DOC-2", Type: "room"}
	for _, addr := range []realtime.RawAddress{doc1Room, doc2Room} {
		_, err := cs.Post(asUser("alice"), addr, note("x"))
		require.NoError(t, err)
	}

	head, err := cs.Head(asUser("alice"), doc1Room)
	require.NoError(t, err)
	require.Equal(t, []string{"close"}, head.Actions.Lifecycle)
	got, err := cs.Lifecycle(asUser("alice"), doc1Room, "close")
	require.NoError(t, err)
	require.Equal(t, channels.StateClosed, got.State)

	head, err = cs.Head(asUser("alice"), doc2Room)
	require.NoError(t, err)
	require.Empty(t, head.Actions.Lifecycle)
	_, err = cs.Lifecycle(asUser("alice"), doc2Room, "close")
	requireStatus(t, err, http.StatusForbidden)
}

// TestChannelService_DegradedRefusesConditions pins AC4 through the real
// wiring: an acl.yaml reload that hides the field a lifecycle condition
// reads makes realtime.yaml invalid, the successor keeps the previous
// config in degraded mode, and the operation is then neither offered nor
// allowed, although the condition would hold on the stored entity.
func TestChannelService_DegradedRefusesConditions(t *testing.T) {
	t.Parallel()
	open := `roles:
  reader:
    read: ["*"]
assignments:
  alice: reader
`
	root := realtimeProject(t, map[string]string{
		"schema.yaml": serviceSchema, "realtime.yaml": conditionRealtimeYAML, "acl.yaml": open,
	})
	st := memstore.New()
	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "DOC-1", Type: "doc",
		Properties: map[string]any{"title": "One", "owner": "alice"}}))
	assemble := func(base *appbuild.SharedBase) *appbuild.Services {
		t.Helper()
		svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })
		return svc
	}

	base, err := realtimeBase(t, root)
	require.NoError(t, err)
	origin := assemble(base)
	cs := appbuild.ChannelService(origin)
	_, err = cs.Post(bindAt(asUser("alice"), t, origin), doc1Room, note("x"))
	require.NoError(t, err)
	head, err := cs.Head(bindAt(asUser("alice"), t, origin), doc1Room)
	require.NoError(t, err)
	require.Equal(t, []string{"close"}, head.Actions.Lifecycle, "the condition holds before the reload")

	// alice now sees only the owner of a doc: the title is hidden from her.
	writeProjectFile(t, root, "acl.yaml", `roles:
  reader:
    read: ["*"]
    visible:
      doc: [{field: owner}]
assignments:
  alice: reader
`)
	next, err := realtimeBase(t, root, appbuild.WithRealtimeFallback(origin))
	require.NoError(t, err)
	svc := assemble(next)
	snap := appbuild.Realtime(svc).Load()
	require.True(t, snap.Degraded)
	require.ErrorContains(t, snap.Problem, "visible: grant in acl.yaml restricts")

	cs = appbuild.ChannelService(svc)
	head, err = cs.Head(bindAt(asUser("alice"), t, svc), doc1Room)
	require.NoError(t, err)
	require.Empty(t, head.Actions.Lifecycle)
	_, err = cs.Lifecycle(bindAt(asUser("alice"), t, svc), doc1Room, "close")
	requireStatus(t, err, http.StatusForbidden)
	d, _ := errors.AsType[*realtime.DenialError](err)
	require.Equal(t, realtime.TypeLifecycleDenied, d.Type)
	require.Contains(t, d.Title, "degraded")
}

// TestChannelService_InjectedACL pins the two policy-free modes: under
// NopACL every reader may read and the owner may post; under ReadOnlyACL
// reads work and every post is refused.
func TestChannelService_InjectedACL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		acl      acl.ACL
		postable bool
	}{
		{"nop", acl.NopACL{}, true},
		{"read only", acl.ReadOnlyACL{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cs := appbuild.ChannelService(serviceFixture(t, map[string]string{}, appbuild.WithACL(tc.acl)))
			_, err := cs.Head(asUser("carol"), doc1Room)
			require.NoError(t, err)
			_, err = cs.Post(asUser("alice"), doc1Room, note("x"))
			if tc.postable {
				require.NoError(t, err)
			} else {
				requireStatus(t, err, http.StatusForbidden)
			}
			_, err = cs.Post(asUser("carol"), doc1Room, note("x"))
			requireStatus(t, err, http.StatusForbidden)
		})
	}
}

// TestChannelService_ReloadRefusesRecordedShapeChange pins that a hot
// reload is held against the type records the channel store has, as boot
// is. The config at boot does not declare room, so the reload's own
// history check knows nothing of it; only the store's records do. A post
// records a type's shape, and closing the assembly stops the check.
func TestChannelService_ReloadRefusesRecordedShapeChange(t *testing.T) {
	t.Parallel()
	lobby := strings.Replace(serviceRealtimeYAML, "  room:\n", "  lobby:\n", 1)
	root := realtimeProject(t, map[string]string{"realtime.yaml": lobby, "schema.yaml": serviceSchema})
	base, err := realtimeBase(t, root, appbuild.WithACL(acl.NopACL{}))
	require.NoError(t, err)
	st := memstore.New()
	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "DOC-1", Type: "doc",
		Properties: map[string]any{"title": "One", "owner": "alice"}}))
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	lobbyRoom := realtime.RawAddress{EntityType: "doc", Anchor: "DOC-1", Type: "lobby"}
	_, err = appbuild.ChannelService(svc).Post(asUser("alice"), lobbyRoom, note("x"))
	require.NoError(t, err)
	recs, err := appbuild.Channels(svc).TypeRecords(ctx)
	require.NoError(t, err)
	require.Equal(t, channels.TypeRecord{
		Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityOne, Refs: []string{"note.about"},
	}, recs["lobby"], "a post records the type's shape")

	require.NoError(t, appbuild.Channels(svc).RecordTypes(ctx, map[string]channels.TypeRecord{
		"room": {Anchor: channels.AnchorFamily, Cardinality: channels.CardinalityMany},
	}))
	holder := appbuild.Realtime(svc)
	prev := holder.Load().Config
	both := lobby + strings.TrimPrefix(serviceRealtimeYAML, "version: 1\nchannel_types:\n")
	writeProjectFile(t, root, "realtime.yaml", both)
	require.ErrorContains(t, holder.Reload(ctx),
		"channel_types.room.cardinality: cannot change from many to one")
	require.Same(t, prev, holder.Load().Config, "a refused reload keeps the previous config")

	require.NoError(t, svc.Close())
	require.NoError(t, holder.Reload(ctx), "a closed assembly's store is no longer checked")
}

// TestChannelService_RefTurnedPlainIsRefused pins that once a post has
// recorded a ref property, neither a hot reload nor a boot accepts a
// realtime.yaml that declares that property as another type: the stored
// entity ids would render to every reader as plain text.
func TestChannelService_RefTurnedPlainIsRefused(t *testing.T) {
	t.Parallel()
	root := realtimeProject(t, map[string]string{"realtime.yaml": serviceRealtimeYAML, "schema.yaml": serviceSchema})
	base, err := realtimeBase(t, root, appbuild.WithACL(acl.NopACL{}))
	require.NoError(t, err)
	st := memstore.New()
	ctx := context.Background()
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{ID: "DOC-1", Type: "doc",
		Properties: map[string]any{"title": "One", "owner": "alice"}}))
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	_, err = appbuild.ChannelService(svc).Post(asUser("alice"), doc1Room, note("x"))
	require.NoError(t, err)

	plain := strings.Replace(serviceRealtimeYAML, "about: { type: ref, to: [doc] }", "about: { type: string }", 1)
	require.NotEqual(t, serviceRealtimeYAML, plain)
	writeProjectFile(t, root, "realtime.yaml", plain)
	holder := appbuild.Realtime(svc)
	require.ErrorContains(t, holder.Reload(ctx), "cannot change from ref")

	again, err := realtimeBase(t, root, appbuild.WithACL(acl.NopACL{}))
	require.NoError(t, err)
	st2 := memstore.New()
	_, err = again.Assemble(st2, search.New(st2, search.NewLinearSearch()), nil, nil)
	require.ErrorContains(t, err, "cannot change from ref")
}

// TestChannelService_Nil pins the accessor's nil contract.
func TestChannelService_Nil(t *testing.T) {
	t.Parallel()
	require.Nil(t, appbuild.ChannelService(nil))
}
