package realtime

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// agentUser is the caller "bot" whose verified principal_type is typ.
func agentUser(typ string) *fakeAccess {
	a := user("bot")
	a.p = principal.VerifiedFrom("bot", principal.ToolMCP, principal.Claims{PrincipalType: typ})
	return a
}

// readOnlyClient makes a's client ceiling refuse every write.
func readOnlyClient(a *fakeAccess) *fakeAccess {
	a.ceiling = false
	return a
}

// TestAuthz_PostMatrix pins AC1 and AC16: a post succeeds exactly when the
// role source matches and, since a post is a write, the client ceiling
// admits update for every source.
func TestAuthz_PostMatrix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		access *fakeAccess
		typ    string
		ok     bool
	}{
		{"attached, ceiling admits", user("bob"), "text", true},
		{"attached, ceiling refuses", readOnlyClient(user("bob")), "text", false},
		{"permission held", user("bob", "chat.post"), "perm-text", true},
		{"permission held, ceiling refuses", readOnlyClient(user("bob", "chat.post")), "perm-text", false},
		{"permission missing", user("bob"), "perm-text", false},
		{"entity property names caller", user("alice"), "owner-text", true},
		{"entity property, ceiling refuses", readOnlyClient(user("alice")), "owner-text", false},
		{"entity property names someone else", user("bob"), "owner-text", false},
		{"entity list property names caller", user("carol"), "watcher-text", true},
		{"entity list property misses caller", user("bob"), "watcher-text", false},
		{"principal type matches", agentUser("agent"), "agent-text", true},
		{"principal type matches, ceiling refuses", readOnlyClient(agentUser("agent")), "agent-text", false},
		{"principal type differs", agentUser("app"), "agent-text", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			_, err := h.svc.Post(as(tc.access), room, PostInput{Type: tc.typ, Props: map[string]any{"body": "hi"}})
			if tc.ok {
				require.NoError(t, err)
				return
			}
			d := requireDenial(t, err, http.StatusForbidden, TypePostDenied)
			require.Contains(t, d.Rule, ".message_types."+tc.typ+".post")
			require.Zero(t, h.store.appends.Load(), "a denied post must not reach the store")
		})
	}
}

// TestAuthz_ReadNeedsNoCeiling pins that an attached read role is decided by
// the gated anchor read alone: a client whose ceiling refuses writes still
// reads.
func TestAuthz_ReadNeedsNoCeiling(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.post(t, user("bob"), text("hello"))
	head, err := h.svc.Head(as(readOnlyClient(user("bob"))), room)
	require.NoError(t, err)
	require.NotNil(t, head.Last)
	require.Empty(t, head.Actions.Post, "a client without update may post nothing")
}

// TestAuthz_Order_NameAfterRead pins AC3: the channel name of a many type is
// looked up only after the read role passed, so a caller without read
// cannot probe which names exist.
func TestAuthz_Order_NameAfterRead(t *testing.T) {
	// Sequential: the subtests share one harness and the code around them.
	h := newHarness(t)
	reader := user("alice", "chat.read")
	ch, err := h.svc.Create(as(reader), "ticket", "TKT-1", "topics", CreateInput{Title: "Launch"})
	require.NoError(t, err)
	h.store.heads.Store(0)

	tests := []struct {
		name   string
		access *fakeAccess
		chName string
		status int
		typ    string
		heads  int64
	}{
		{"no read, existing name", user("bob"), ch.Name, http.StatusForbidden, TypeReadDenied, 0},
		{"no read, missing name", user("bob"), "zzzzzzzzzzzzz", http.StatusForbidden, TypeReadDenied, 0},
		{"read, missing name", reader, "zzzzzzzzzzzzz", http.StatusNotFound, TypeChannelNotFound, 1},
		{"read, no name", reader, "", http.StatusNotFound, TypeChannelNotFound, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h.store.heads.Store(0)
			addr := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "topics", Name: tc.chName}
			_, err = h.svc.Head(as(tc.access), addr)
			d := requireDenial(t, err, tc.status, tc.typ)
			if tc.typ == TypeReadDenied {
				require.Equal(t, "channel_types.topics.read", d.Rule)
			}
			require.Equal(t, tc.heads, h.store.heads.Load())
		})
	}
	_, err = h.svc.Head(as(reader), RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "topics", Name: ch.Name})
	require.NoError(t, err)
}

// TestAuthz_HiddenAnchor pins the resolution order before the read role:
// an unknown type or a type that does not attach to the route's entity type
// is 404 unknown_channel_type without reading the anchor (after the read
// for a route naming no type); a face type needs
// a face before the read; every anchor miss is ErrAnchorNotFound.
func TestAuthz_HiddenAnchor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		addr    RawAddress
		status  int
		typ     string
		anchor  bool // want ErrAnchorNotFound
		readsAt int64
	}{
		{"unknown type", RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "nope"},
			http.StatusNotFound, TypeUnknownChannelType, false, 0},
		{"type not attached to route type", RawAddress{EntityType: "doc", Anchor: "DOC-1", Type: "room"},
			http.StatusNotFound, TypeUnknownChannelType, false, 0},
		{"missing anchor", RawAddress{EntityType: "ticket", Anchor: "TKT-9", Type: "room"}, 0, "", true, 1},
		// A route that names no entity type reads the anchor first; the
		// entity is readable, so its type may be named.
		{"untyped route, readable anchor outside attach_to", RawAddress{Anchor: "DOC-1", Type: "room"},
			http.StatusNotFound, TypeUnknownChannelType, false, 1},
		{"malformed anchor", RawAddress{Anchor: "not an id", Type: "room"}, 0, "", true, 0},
		{"face type with bare id", RawAddress{EntityType: "page", Anchor: "PAGE-1", Type: "facechat"},
			http.StatusUnprocessableEntity, TypeFaceRequired, false, 0},
		{"face type with missing face", RawAddress{EntityType: "page", Anchor: "PAGE-1@gone", Type: "facechat"},
			0, "", true, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			_, err := h.svc.Head(as(user("bob")), tc.addr)
			if tc.anchor {
				require.ErrorIs(t, err, ErrAnchorNotFound)
			} else {
				requireDenial(t, err, tc.status, tc.typ)
			}
			require.Equal(t, tc.readsAt, h.anchors.calls.Load())
		})
	}
}

// TestAuthz_FaceAnchor pins that a face type anchors to the admitted face
// and a family type to the whole entity, whichever face served it.
func TestAuthz_FaceAnchor(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	draft := RawAddress{EntityType: "page", Anchor: "PAGE-1@draft", Type: "facechat"}
	_, err := h.svc.Post(as(bob), draft, text("draft note"))
	require.NoError(t, err)
	head, err := h.svc.Head(as(bob), draft)
	require.NoError(t, err)
	require.Equal(t, "PAGE-1@draft", head.Channel.Anchor)
	require.EqualValues(t, 1, head.Channel.LastSeq)
	pub, err := h.svc.Head(as(bob), RawAddress{EntityType: "page", Anchor: "PAGE-1@published", Type: "facechat"})
	require.NoError(t, err)
	require.Zero(t, pub.Channel.LastSeq, "each face has its own channel")
}

// TestAuthz_RealtimeOff pins that every operation answers ErrRealtimeOff
// when the project has no realtime.yaml.
func TestAuthz_RealtimeOff(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.cfg.snap = Snapshot{}
	bob := as(user("bob"))
	_, err := h.svc.Head(bob, room)
	require.ErrorIs(t, err, ErrRealtimeOff)
	_, err = h.svc.Post(bob, room, text("x"))
	require.ErrorIs(t, err, ErrRealtimeOff)
	_, err = h.svc.ListAnchor(bob, "ticket", "TKT-1", "")
	require.ErrorIs(t, err, ErrRealtimeOff)
	_, err = h.svc.Create(bob, "ticket", "TKT-1", "topics", CreateInput{})
	require.ErrorIs(t, err, ErrRealtimeOff)
	require.Zero(t, h.anchors.calls.Load())
}

// TestAuthz_DegradedEntityProperty pins AC13: in degraded mode an
// entity-property role grants nobody, and the other sources still work.
func TestAuthz_DegradedEntityProperty(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice := user("alice", "chat.post")
	_, err := h.svc.Post(as(alice), room, PostInput{Type: "owner-text", Props: map[string]any{"body": "x"}})
	require.NoError(t, err)

	h.cfg.snap.Degraded = true
	_, err = h.svc.Post(as(alice), room, PostInput{Type: "owner-text", Props: map[string]any{"body": "x"}})
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
	_, err = h.svc.Post(as(alice), room, PostInput{Type: "perm-text", Props: map[string]any{"body": "x"}})
	require.NoError(t, err)
	_, err = h.svc.Post(as(alice), room, text("x"))
	require.NoError(t, err)
}

// TestAuthz_EntityPropertyUsesQueryIdentity pins that an entity-property
// role compares through the resolved query identity, not the raw user: a
// mapped user matches by its entity id, and an invalid identity matches
// nothing.
func TestAuthz_EntityPropertyUsesQueryIdentity(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	mapped := user("alice@example.com")
	mapped.q = predicatefns.QueryIdentity{EntityID: "alice", Raw: "alice@example.com"}
	_, err := h.svc.Post(as(mapped), room, PostInput{Type: "owner-text", Props: map[string]any{"body": "x"}})
	require.NoError(t, err)

	anonymous := user("alice")
	anonymous.q = predicatefns.QueryIdentity{}
	_, err = h.svc.Post(as(anonymous), room, PostInput{Type: "owner-text", Props: map[string]any{"body": "x"}})
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
}

// TestAuthz_PrincipalTypeVerified pins that the principal-type source reads
// only the verified claim: a user named like the type gets nothing.
func TestAuthz_PrincipalTypeVerified(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_, err := h.svc.Post(as(user("agent")), room, PostInput{Type: "agent-text", Props: map[string]any{"body": "x"}})
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
	_, err = h.svc.Post(as(agentUser("agent")), room,
		PostInput{Type: "agent-text", Props: map[string]any{"body": "x"}})
	require.NoError(t, err)
}

// TestAuthz_Creator pins the creator source: the creator reads their
// channel; anyone else is refused with the read denial, which is also the
// answer for a name that does not exist. Creator identity is the user and
// the organization.
func TestAuthz_Creator(t *testing.T) {
	// Sequential: the subtests share one harness and the code around them.
	h := newHarness(t)
	alice := user("alice")
	ch, err := h.svc.Create(as(alice), "ticket", "TKT-1", "threads", CreateInput{Title: "Mine"})
	require.NoError(t, err)
	addr := RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "threads", Name: ch.Name}

	_, err = h.svc.Post(as(alice), addr, PostInput{Type: "text", Props: map[string]any{"body": "x"}})
	require.NoError(t, err)
	_, err = h.svc.Head(as(alice), addr)
	require.NoError(t, err)

	tests := []struct {
		name   string
		access *fakeAccess
		addr   RawAddress
	}{
		{"another user", user("bob"), addr},
		{"same user, another organization", &fakeAccess{
			p:       principal.VerifiedFrom("alice", principal.ToolDataEntry, principal.Claims{OrgID: "other"}),
			ceiling: true,
		}, addr},
		{"missing name", user("bob"), RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "threads",
			Name: "zzzzzzzzzzzzz"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err = h.svc.Head(as(tc.access), tc.addr)
			requireDenial(t, err, http.StatusForbidden, TypeReadDenied)
		})
	}

	mine, err := h.svc.ListAnchor(as(alice), "ticket", "TKT-1", "")
	require.NoError(t, err)
	theirs, err := h.svc.ListAnchor(as(user("bob")), "ticket", "TKT-1", "")
	require.NoError(t, err)
	require.Len(t, channelsOf(t, mine, "threads"), 1)
	require.Empty(t, channelsOf(t, theirs, "threads"), "another user must not see the creator's channel")
}

func channelsOf(t *testing.T, ac AnchorChannels, typ string) []ChannelView {
	t.Helper()
	for _, tc := range ac.Types {
		if tc.Type == typ {
			return tc.Channels
		}
	}
	t.Fatalf("type %s not listed", typ)
	return nil
}

// TestAuthz_ACLModes pins AC16: without a policy every permission is held
// (NopACL); under ReadOnlyACL every write is refused and reads work. A
// caller with no identity may not write, and one the ACL cannot evaluate
// may do nothing.
func TestAuthz_ACLModes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	nop := user("bob", "chat.post", "chat.moderate", "chat.read")
	readOnly := readOnlyClient(user("ro", "chat.post", "chat.moderate", "chat.read"))

	m := h.post(t, nop, PostInput{Type: "perm-text", Props: map[string]any{"body": "x"}})
	_, err := h.svc.Delete(as(nop), room, m.Seq, "spam")
	require.NoError(t, err, "nop may remove")

	_, err = h.svc.Head(as(readOnly), room)
	require.NoError(t, err)
	_, err = h.svc.Read(as(readOnly), room, ReadRequest{})
	require.NoError(t, err)
	_, err = h.svc.Post(as(readOnly), room, text("x"))
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
	_, err = h.svc.Create(as(readOnly), "ticket", "TKT-1", "topics", CreateInput{})
	requireDenial(t, err, http.StatusForbidden, TypeCreateDenied)

	unknown := user(principal.Unknown)
	_, err = h.svc.Head(as(unknown), room)
	require.NoError(t, err, "an anonymous caller may read what the ACL lets them")
	_, err = h.svc.Post(as(unknown), room, text("x"))
	requireDenial(t, err, http.StatusForbidden, TypeNoPrincipal)

	_, err = h.svc.Head(context.Background(), room)
	requireDenial(t, err, http.StatusForbidden, TypeNoPrincipal)
}

// TestAuthz_CreateGate pins the create gate: the operator's denied message
// is the refusal title, a one type is not creatable, and the per-anchor
// channel cap refuses with 409.
func TestAuthz_CreateGate(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_, err := h.svc.Create(as(readOnlyClient(user("bob", "chat.read"))), "ticket", "TKT-1", "topics", CreateInput{})
	d := requireDenial(t, err, http.StatusForbidden, TypeCreateDenied)
	require.Equal(t, "channel_types.topics.create", d.Rule)

	_, err = h.svc.Create(as(user("bob")), "ticket", "TKT-1", "room", CreateInput{})
	requireDenial(t, err, http.StatusConflict, TypeNotCreatable)

	_, err = h.svc.Create(as(user("bob")), "ticket", "TKT-1", "topics", CreateInput{})
	requireDenial(t, err, http.StatusForbidden, TypeReadDenied)

	_, err = h.svc.Create(as(user("bob", "chat.read")), "ticket", "TKT-9", "topics", CreateInput{})
	require.ErrorIs(t, err, ErrAnchorNotFound)
}
