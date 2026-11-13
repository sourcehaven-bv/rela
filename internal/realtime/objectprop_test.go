package realtime

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// objectRealtime has a question type whose options are an object list,
// with choices that read each button's text from the label field.
const objectRealtime = `
version: 1
channel_types:
  room:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      ask:
        properties:
          question: { type: string, required: true }
          options:
            type: object
            list: true
            max_items: 4
            properties:
              label: { type: string, required: true, max: 40 }
              recommended: { type: boolean }
              pros: { type: string, list: true }
        post: [member]
        choices: { from: options, label: label, answer: answer, into: body }
      answer:
        properties: { body: { type: string, required: true } }
        post: [member]
`

// TestPost_ObjectListProp pins that an object list posts through
// Service.Post, is stored in its normalized form and reads back whole,
// and that a repeated label or an undeclared field is refused with 400.
func TestPost_ObjectListProp(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(objectRealtime))
	bob := user("bob")
	res, err := h.svc.Post(as(bob), room, PostInput{Type: "ask", Props: map[string]any{
		"question": "Which store?",
		"options": []any{
			map[string]any{"label": " Postgres ", "recommended": true, "pros": []any{"durable"}},
			map[string]any{"label": "SQLite", "pros": []any{}},
		},
	}})
	require.NoError(t, err)
	want := []any{
		map[string]any{"label": "Postgres", "recommended": true, "pros": []any{"durable"}},
		map[string]any{"label": "SQLite"},
	}
	require.Equal(t, want, res.Message.Props["options"])

	msgs, err := h.store.Store.Messages(context.Background(), roomKey(), channels.Page{})
	require.NoError(t, err)
	require.Equal(t, want, msgs[len(msgs)-1].Props["options"], "the store keeps the normalized list")

	read := h.readAll(t, bob, room)
	require.Equal(t, want, read.Messages[len(read.Messages)-1].Props["options"])

	for _, tc := range []struct {
		name    string
		options []any
		want    string
	}{
		{"repeated label", []any{map[string]any{"label": "A"}, map[string]any{"label": "A"}},
			"item 2 repeats the label of an earlier item"},
		{"undeclared field", []any{map[string]any{"label": "A", "cons": "x"}},
			`item 1 has field "cons", which options does not declare`},
		{"missing label", []any{map[string]any{"recommended": true}}, `item 1 needs field "label"`},
		{"not an object", []any{"A"}, "item 1 must be an object"},
	} {
		// One harness for every case, so the cases run in turn.
		_, err := h.svc.Post(as(bob), room, PostInput{Type: "ask", Props: map[string]any{
			"question": "Which?", "options": tc.options,
		}})
		d := requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
		require.Contains(t, d.Error(), tc.want, tc.name)
	}
}
