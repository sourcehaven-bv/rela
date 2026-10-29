package realtime

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

func gen(n int64) *int64 { return new(n) }

// TestPost_ExpectGeneration pins AC 1 of TKT-T9G1AE: a post with
// ExpectGeneration is stored only when the session is at that generation;
// otherwise it is refused 409 session_moved, stores nothing and spends no
// rate token. Without it the post is unchanged. It is a 400 on a channel
// type without a session and on an ephemeral type.
func TestPost_ExpectGeneration(t *testing.T) {
	t.Parallel()
	nudge := strings.Replace(pocRealtime(""), "    lifecycle:\n",
		"      nudge: { persist: ephemeral, ttl: 30s, post: [owner] }\n    lifecycle:\n", 1)
	tests := []struct {
		name   string
		yaml   string
		addr   RawAddress
		state  string
		in     PostInput
		status int // 0: the post lands
		typ    string
		title  string
		moveTo string // the session state after a landed post; empty: not checked
	}{
		{name: "match stores and moves", state: "plan-review",
			in:     PostInput{Type: "approve", Props: map[string]any{"note": "ok"}, ExpectGeneration: gen(3)},
			moveTo: "working"},
		{name: "mismatch is refused", state: "plan-review",
			in:     PostInput{Type: "approve", Props: map[string]any{"note": "ok"}, ExpectGeneration: gen(2)},
			status: http.StatusConflict, typ: TypeSessionMoved,
			title: "The session moved on: it is at generation 3 (plan-review); this post expected generation 2."},
		{name: "absent is unchanged", state: "plan-review",
			in:     PostInput{Type: "approve", Props: map[string]any{"note": "ok"}},
			moveTo: "working"},
		{name: "zero on a never-moved session matches", state: "new",
			in: PostInput{Type: "go", Props: map[string]any{"note": "n"}, ExpectGeneration: gen(0)}, moveTo: "queued"},
		{name: "zero on an unstored channel matches", state: "unstored",
			in: PostInput{Type: "go", Props: map[string]any{"note": "n"}, ExpectGeneration: gen(0)}, moveTo: "queued"},
		{name: "a type that moves nothing at the generation lands", state: "plan-review",
			in: PostInput{Type: "text", Props: map[string]any{"body": "hi"}, ExpectGeneration: gen(3)}},
		{name: "negative is a 400", state: "plan-review",
			in:     PostInput{Type: "approve", Props: map[string]any{"note": "ok"}, ExpectGeneration: gen(-1)},
			status: http.StatusBadRequest, typ: TypeInvalid},
		{name: "no session is a 400", yaml: testRealtime, addr: room,
			in:     PostInput{Type: "text", Props: map[string]any{"body": "hi"}, ExpectGeneration: gen(0)},
			status: http.StatusBadRequest, typ: TypeInvalid},
		{name: "ephemeral is a 400", yaml: nudge, state: "plan-review",
			in:     PostInput{Type: "nudge", ExpectGeneration: gen(3)},
			status: http.StatusBadRequest, typ: TypeInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			y := tc.yaml
			if y == "" {
				y = pocRealtime("")
			}
			h := newHarness(t, withYAML(y))
			addr := tc.addr
			if addr == (RawAddress{}) {
				addr = pocRun
				h.pocDriveTo(t, tc.state)
			}
			o := pocOwner()
			var lastSeq int64
			if tc.state != "" && tc.state != "unstored" {
				head, err := h.store.Store.Head(context.Background(), pocKey())
				require.NoError(t, err)
				lastSeq = head.LastSeq
			}
			bucketBefore := h.postBucket(o, tc.in.Type)
			res, err := h.svc.Post(as(o), addr, tc.in)
			if tc.status == 0 {
				require.NoError(t, err)
				require.NotNil(t, res.Message)
				if tc.moveTo != "" {
					require.Equal(t, tc.moveTo, res.Session.State)
				}
				return
			}
			d := requireDenial(t, err, tc.status, tc.typ)
			if tc.title != "" {
				require.Equal(t, tc.title, d.Title)
			}
			if addr == pocRun {
				h.requireNothingAppended(t, lastSeq)
				require.Equal(t, bucketBefore, h.postBucket(o, tc.in.Type), "a refused pre-check spent a rate token")
			}
		})
	}
}

// TestPost_ExpectGenerationStoreRefuses pins that the store checks the
// generation under its lock: when the session moves between the post's
// read and its append, the post is refused 409 session_moved with the
// generation and state the store saw, and nothing is stored.
func TestPost_ExpectGenerationStoreRefuses(t *testing.T) {
	t.Parallel()
	h := newPocHarness(t)
	h.pocDriveTo(t, "plan-review")
	o := pocOwner()
	var lastSeq int64
	h.store.beforeAppend = func() {
		h.postTo(t, o, pocRun, "approve", map[string]any{"note": "go"})
		h.postTo(t, pocRunner(), pocRun, "permission", map[string]any{"note": "N+1"})
		head, err := h.store.Store.Head(context.Background(), pocKey())
		require.NoError(t, err)
		lastSeq = head.LastSeq
	}
	_, err := h.svc.Post(as(o), pocRun, PostInput{
		Type: "text", Props: map[string]any{"body": "late"}, ExpectGeneration: gen(3),
	})
	d := requireDenial(t, err, http.StatusConflict, TypeSessionMoved)
	require.Equal(t, "The session moved on: it is at generation 5 (approving); this post expected generation 3.", d.Title)
	h.requireNothingAppended(t, lastSeq)
	head, err := h.store.Store.Head(context.Background(), pocKey())
	require.NoError(t, err)
	require.Equal(t, channels.Session{State: "approving", Generation: 5, EnteredAt: head.Session.EnteredAt}, head.Session)
}

// TestPost_ButtonPreset pins TKT-S9TLJJ AC 8: a button's preset, posted
// as the caller with expect_generation, is a normal post. It moves the
// session along the button's entry and runs that entry's outputs; at an
// old generation it is refused session_moved.
func TestPost_ButtonPreset(t *testing.T) {
	t.Parallel()
	member := user("mo", "plan.member")
	w := &fakeOutputs{}
	h := newHarness(t, withYAML(buttonRealtime()), withOutputs(w))
	head, err := h.svc.Head(as(member), planCh)
	require.NoError(t, err)
	var preset map[string]any
	for _, r := range head.Channel.Session.Transitions {
		if r.MessageType == "decide" && r.Entry == 3 {
			require.NotNil(t, r.Button)
			preset = r.Button.Props
		}
	}
	require.Equal(t, map[string]any{"decision": "approve"}, preset)

	_, err = h.svc.Post(as(member), planCh, PostInput{Type: "decide", Props: preset, ExpectGeneration: gen(1)})
	requireDenial(t, err, http.StatusConflict, TypeSessionMoved)
	require.Empty(t, w.requests(), "a refused preset wrote an output")

	res, err := h.svc.Post(as(member), planCh, PostInput{
		Type: "decide", Props: preset, ExpectGeneration: gen(head.Channel.Session.Generation),
	})
	require.NoError(t, err)
	require.Equal(t, "working", res.Session.State)
	reqs := w.requests()
	require.Len(t, reqs, 1)
	require.Equal(t, map[string]any{"status": "in-progress"}, reqs[0].Props, "the button entry's outputs ran")
}
