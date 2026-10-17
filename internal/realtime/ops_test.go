package realtime

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"strings"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func pageAll() channels.Page { return channels.Page{} }

// TestOps_RemovePlaceholder pins AC6: a removed message renders only its
// seq, rev, thread link and removal; props, author, type and time are gone.
func TestOps_RemovePlaceholder(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob, mod := user("bob"), user("mo", "chat.moderate")
	parent := h.post(t, bob, text("parent"))
	h.post(t, bob, PostInput{Type: "text", Props: map[string]any{"body": "reply"}, ReplyTo: parent.Seq})

	got, err := h.svc.Delete(as(mod), room, parent.Seq, "off topic")
	require.NoError(t, err)
	require.NotNil(t, got.Removed)

	page, err := h.svc.Messages(as(bob), room, pageAll())
	require.NoError(t, err)
	require.Len(t, page.Messages, 2)
	removed := page.Messages[0]
	raw, err := json.Marshal(removed)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.ElementsMatch(t, []string{"seq", "rev", "reply_count", "removed"}, keys(fields))
	require.Equal(t, 1, removed.ReplyCount)
	require.Equal(t, channels.RemovalModerator, removed.Removed.Kind)
	require.Nil(t, removed.Removed.By, "a reader without the remove gate does not see who removed it")
	require.Empty(t, removed.Removed.Reason)
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestOps_ReasonRequired pins that remove refuses an empty reason when the
// type requires one, and that the refusal does not remove.
func TestOps_ReasonRequired(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.post(t, user("bob"), text("x"))
	_, err := h.svc.Delete(as(user("mo", "chat.moderate")), room, m.Seq, "")
	requireDenial(t, err, http.StatusBadRequest, TypeReasonRequired)
	head, err := h.svc.Head(as(user("bob")), room)
	require.NoError(t, err)
	require.Nil(t, head.Last.Removed)
}

// TestOps_ReasonVisibility pins that who removed a message and why are
// shown only to readers who pass the remove gate, and that an author's own
// delete reads as kind author.
func TestOps_ReasonVisibility(t *testing.T) {
	// Sequential: the subtests share one harness and the code around them.
	h := newHarness(t)
	bob, mod := user("bob"), user("mo", "chat.moderate")
	first := h.post(t, bob, text("one"))
	second := h.post(t, bob, text("two"))
	_, err := h.svc.Delete(as(mod), room, first.Seq, "spam")
	require.NoError(t, err)
	own, err := h.svc.Delete(as(bob), room, second.Seq, "ignored")
	require.NoError(t, err)
	require.Equal(t, channels.RemovalAuthor, own.Removed.Kind)

	tests := []struct {
		name   string
		reader *fakeAccess
		by     bool
	}{
		{"moderator", mod, true},
		{"author", bob, false},
		{"moderator whose ceiling refuses writes", readOnlyClient(user("mo2", "chat.moderate")), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := h.svc.Messages(as(tc.reader), room, pageAll())
			require.NoError(t, err)
			require.Len(t, page.Messages, 2)
			mr, ar := page.Messages[0].Removed, page.Messages[1].Removed
			require.Equal(t, channels.RemovalModerator, mr.Kind)
			require.Equal(t, channels.RemovalAuthor, ar.Kind)
			if tc.by {
				require.Equal(t, "spam", mr.Reason)
				require.Equal(t, "mo", mr.By.User)
				require.Empty(t, ar.Reason, "an author's own delete records no reason")
			} else {
				require.Empty(t, mr.Reason)
				require.Nil(t, mr.By)
			}
		})
	}
}

// TestOps_DeleteRules pins how DELETE picks the operation: the author's
// delete_own first, then the moderator's remove, else a refusal naming
// both. A removed message cannot be removed again.
func TestOps_DeleteRules(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	m := h.post(t, bob, text("x"))

	_, err := h.svc.Delete(as(user("eve")), room, m.Seq, "")
	d := requireDenial(t, err, http.StatusForbidden, TypeDeleteDenied)
	require.Contains(t, d.Title, "the author with")
	require.Contains(t, d.Title, "moderator")

	_, err = h.svc.Delete(as(bob), room, 99, "")
	requireDenial(t, err, http.StatusNotFound, TypeMessageNotFound)

	_, err = h.svc.Delete(as(bob), room, m.Seq, "")
	require.NoError(t, err)
	_, err = h.svc.Delete(as(bob), room, m.Seq, "")
	requireDenial(t, err, http.StatusConflict, TypeRemoved)
}

// TestOps_EditWithin pins AC7: edit_own works inside `within:` of posting
// and is refused after it.
func TestOps_EditWithin(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	m := h.post(t, bob, text("draft"))

	h.clock.Add(14 * time.Minute)
	got, err := h.svc.Edit(as(bob), room, m.Seq, map[string]any{"body": "final"})
	require.NoError(t, err)
	require.Equal(t, "final", got.Props["body"])
	require.Greater(t, got.Rev, m.Rev)
	require.False(t, got.EditedAt.IsZero())

	h.clock.Add(2 * time.Minute)
	_, err = h.svc.Edit(as(bob), room, m.Seq, map[string]any{"body": "late"})
	requireDenial(t, err, http.StatusForbidden, TypeEditWindowClosed)

	_, err = h.svc.Edit(as(user("eve")), room, m.Seq, map[string]any{"body": "mine now"})
	requireDenial(t, err, http.StatusForbidden, TypeEditDenied)
}

// TestOps_AuthorIsUserAndOrg pins that authorship compares the user and
// the organization, not the tool.
func TestOps_AuthorIsUserAndOrg(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	acme := &fakeAccess{
		p:       principal.VerifiedFrom("bob", principal.ToolDataEntry, principal.Claims{OrgID: "acme"}),
		ceiling: true,
	}
	m := h.post(t, acme, text("x"))
	sameUserOtherTool := &fakeAccess{
		p:       principal.VerifiedFrom("bob", principal.ToolMCP, principal.Claims{OrgID: "acme"}),
		ceiling: true,
	}
	_, err := h.svc.Edit(as(sameUserOtherTool), room, m.Seq, map[string]any{"body": "y"})
	require.NoError(t, err)
	otherOrg := &fakeAccess{
		p:       principal.VerifiedFrom("bob", principal.ToolDataEntry, principal.Claims{OrgID: "evil"}),
		ceiling: true,
	}
	_, err = h.svc.Edit(as(otherOrg), room, m.Seq, map[string]any{"body": "z"})
	requireDenial(t, err, http.StatusForbidden, TypeEditDenied)
}

// TestOps_DeleteOwnFrom pins AC7: delete_own applies only while the channel
// is in its `from:` state; a closed channel refuses it naming the state.
func TestOps_DeleteOwnFrom(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	m := h.post(t, bob, text("x"))
	_, err := h.store.SetLifecycle(as(bob), roomKey(), channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: channels.Author{User: "bob"}, At: h.clock.Now()})
	require.NoError(t, err)

	_, err = h.svc.Delete(as(bob), room, m.Seq, "")
	d := requireDenial(t, err, http.StatusForbidden, TypeDeleteDenied)
	require.Contains(t, d.Rule, "delete_own.from")

	_, err = h.svc.Delete(as(user("mo", "chat.moderate")), room, m.Seq, "closed anyway")
	require.NoError(t, err, "a moderator may still remove")
}

// TestOps_EditRevalidates pins that an edit passes every check a post
// does: validation, the type's when:, the refs, a current post role and an
// open channel.
func TestOps_EditRevalidates(t *testing.T) {
	// Sequential: the subtests share one harness and the code around them.
	h := newHarness(t)
	bob := user("bob")
	m := h.post(t, bob, PostInput{Type: "note", Props: map[string]any{"level": 1}})

	tests := []struct {
		name   string
		access *fakeAccess
		props  map[string]any
		status int
		typ    string
	}{
		{"undeclared property", bob, map[string]any{"level": 1, "x": "y"}, http.StatusBadRequest, TypeInvalid},
		{"missing required", bob, map[string]any{"about": "TKT-2"}, http.StatusBadRequest, TypeInvalid},
		{"when fails", bob, map[string]any{"level": 0}, http.StatusForbidden, TypePostDenied},
		{"unreadable ref", bob, map[string]any{"level": 1, "about": "TKT-9"},
			http.StatusUnprocessableEntity, TypeInvalidRef},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.svc.Edit(as(tc.access), room, m.Seq, tc.props)
			requireDenial(t, err, tc.status, tc.typ)
		})
	}
	got, err := h.svc.Edit(as(bob), room, m.Seq, map[string]any{"level": 3, "about": "TKT-2"})
	require.NoError(t, err)
	require.Equal(t, &RefValue{ID: "TKT-2", Type: "ticket", Title: "Second"}, got.Props["about"])

	_, err = h.store.SetLifecycle(as(bob), roomKey(), channels.StateOpen,
		channels.Transition{To: channels.StateClosed, By: channels.Author{User: "bob"}, At: h.clock.Now()})
	require.NoError(t, err)
	_, err = h.svc.Edit(as(bob), room, m.Seq, map[string]any{"level": 4})
	requireDenial(t, err, http.StatusConflict, TypeChannelNotOpen)
}

// TestOps_EditNeedsCurrentPostRole pins that an author who lost the post
// role of the message type may not edit, though the edit_own gate passes.
func TestOps_EditNeedsCurrentPostRole(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	alice := user("alice")
	m := h.post(t, alice, PostInput{Type: "owner-text", Props: map[string]any{"body": "x"}})
	h.cfg.snap.Degraded = true
	_, err := h.svc.Edit(as(alice), room, m.Seq, map[string]any{"body": "y"})
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)
}

// TestOps_MessageTypeGone pins that editing a message whose type the
// configuration no longer declares is refused, and that it renders without
// properties.
func TestOps_MessageTypeGone(t *testing.T) {
	t.Parallel()
	h2 := newHarness(t)
	held := user("bob", "chat.post")
	msg := h2.post(t, held, PostInput{Type: "perm-text", Props: map[string]any{"body": "x"}})
	h2.cfg.snap = Snapshot{Config: testCompiled(t, withoutType(testRealtime, "perm-text"))}
	_, err := h2.svc.Edit(as(held), room, msg.Seq, map[string]any{"body": "y"})
	requireDenial(t, err, http.StatusConflict, TypeMessageTypeGone)
	page, err := h2.svc.Messages(as(held), room, pageAll())
	require.NoError(t, err)
	require.Nil(t, page.Messages[0].Props, "a message of an undeclared type renders no properties")
}

// withoutType removes the perm-text message type from the test config.
func withoutType(yaml, name string) string {
	block := "      " + name + ":\n        properties: { body: { type: string } }\n        post: [poster]\n"
	return strings.Replace(yaml, block, "", 1)
}
