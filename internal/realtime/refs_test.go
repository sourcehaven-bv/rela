package realtime

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRefs_RestrictedAndUniform422 pins AC4: posting a ref the caller
// cannot read, one that does not exist and one of a type the property does
// not allow all get the same 422 body; a ref that becomes unreadable
// renders as restricted, with no id, type or title.
func TestRefs_RestrictedAndUniform422(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	bob := user("bob")
	h.refs.views["TKT-3"] = RefView{ID: "TKT-3", Type: "ticket", Title: "Hidden later"}

	var bodies []DenialError
	for _, about := range []string{"TKT-9", "DOC-1"} {
		_, err := h.svc.Post(as(bob), room, PostInput{Type: "note", Props: map[string]any{"level": 1, "about": about}})
		d := requireDenial(t, err, http.StatusUnprocessableEntity, TypeInvalidRef)
		bodies = append(bodies, *d)
	}
	require.Equal(t, bodies[0], bodies[1], "a missing and a disallowed ref must be refused alike")
	require.Zero(t, h.store.appends.Load())

	m := h.post(t, bob, PostInput{Type: "note", Props: map[string]any{"level": 1, "about": "TKT-3", "also": "DOC-1"}})
	require.Equal(t, &RefValue{ID: "TKT-3", Type: "ticket", Title: "Hidden later"}, m.Props["about"])

	delete(h.refs.views, "TKT-3")
	h.refs.calls.Store(0)
	page, err := h.svc.Read(as(bob), room, ReadRequest{})
	require.NoError(t, err)
	require.Equal(t, &RefValue{Restricted: true}, page.Messages[0].Props["about"])
	require.Equal(t, &RefValue{ID: "DOC-1", Type: "doc", Title: "A doc"}, page.Messages[0].Props["also"])
	require.EqualValues(t, 1, h.refs.calls.Load())
}

// TestRefs_OneResolvePerPage pins that a page of messages resolves every
// ref in one batch, whatever the number of messages.
func TestRefs_OneResolvePerPage(t *testing.T) {
	t.Parallel()
	for _, n := range []int{10, 25} {
		h := newHarness(t)
		bob := user("bob")
		for i := range n {
			about := "TKT-2"
			if i%2 == 0 {
				about = "DOC-1"
			}
			h.post(t, bob, PostInput{Type: "note", Props: map[string]any{"level": i + 1, "also": about}})
			h.clock.Add(3e9) // stay under the post rate
		}
		h.refs.calls.Store(0)
		h.store.heads.Store(0)
		page, err := h.svc.Read(as(bob), room, ReadRequest{})
		require.NoError(t, err)
		require.Len(t, page.Messages, n)
		require.EqualValues(t, 1, h.refs.calls.Load(), "n=%d", n)
		require.EqualValues(t, 1, h.store.heads.Load(), "n=%d", n)
		require.EqualValues(t, 1, h.anchors.calls.Load()-int64(n), "n=%d: one anchor read for the page", n)
	}
}
