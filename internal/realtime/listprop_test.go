package realtime

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// listRealtime has a question type ask with a list of options and a
// `when:` over it, and the answer type it names in choices. There is no
// session.
const listRealtime = `
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
          options:  { type: string, list: true, max: 80, max_items: 4 }
        when: "len(message.options) <= 3"
        post: [member]
        choices: { from: options, answer: answer, into: body, other: true }
      answer:
        properties: { body: { type: string, required: true } }
        post: [member]
      must:
        properties: { picks: { type: string, list: true, required: true } }
        post: [member]
`

// listProp is the options property of listRealtime's ask.
func listProp() realtimecfg.Property {
	return realtimecfg.Property{Name: "options", Type: realtimecfg.PropertyString, List: true, Max: 80, MaxItems: 4}
}

// TestValidateProps_StringList pins AC 1 of TKT-03FQPD: a list value is
// trimmed and NFC-normalized item by item, and each bad value is refused
// with 400 invalid_message naming the property.
func TestValidateProps_StringList(t *testing.T) {
	t.Parallel()
	mt := &realtimecfg.MessageType{Name: "ask", Properties: map[string]realtimecfg.Property{"options": listProp()}}
	required := &realtimecfg.MessageType{Name: "must", Properties: map[string]realtimecfg.Property{
		"options": func() realtimecfg.Property { p := listProp(); p.Required = true; return p }(),
	}}
	ok := []struct {
		name string
		in   any
		want []any
	}{
		{"trimmed", []any{"Yes", " No "}, []any{"Yes", "No"}},
		{"a []string", []string{"A", "B"}, []any{"A", "B"}},
		{"NFC", []any{"e\u0301"}, []any{"\u00e9"}},
		{"ZWJ emoji", []any{"\U0001F468\u200D\U0001F469\u200D\U0001F467"}, []any{"\U0001F468\u200D\U0001F469\u200D\U0001F467"}},
		{"CJK and combining marks", []any{"中文", "n\u0303"}, []any{"中文", "\u00f1"}},
		{"80 characters", []any{strings.Repeat("\u00e9", 80)}, []any{strings.Repeat("\u00e9", 80)}},
		{"4 items", []any{"a", "b", "c", "d"}, []any{"a", "b", "c", "d"}},
		{"emoji style selector dropped", []any{"\u2714\uFE0F Done", "\u2764\uFE0E"}, []any{"\u2714 Done", "\u2764"}},
		{"ZWJ emoji with a style selector", []any{"\U0001F3F3\uFE0F\u200D\U0001F308"}, []any{"\U0001F3F3\u200D\U0001F308"}},
		{"ZWJ after a skin tone", []any{"\U0001F468\U0001F3FD\u200D\U0001F4BB"}, []any{"\U0001F468\U0001F3FD\u200D\U0001F4BB"}},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := validateProps(mt, map[string]any{"options": tc.in}, 1<<14)
			require.NoError(t, err)
			require.Equal(t, tc.want, got.props["options"])
		})
	}
	refused := []struct {
		name   string
		mt     *realtimecfg.MessageType
		in     any
		reason string
	}{
		{"not an array", mt, "Yes", "must be a list of strings"},
		{"an object", mt, map[string]any{"a": "b"}, "must be a list of strings"},
		{"an item that is not a string", mt, []any{1.0}, "item 1 must be a string"},
		{"an empty item", mt, []any{"a", "  "}, "item 2 is empty"},
		{"an item over max", mt, []any{strings.Repeat("x", 81)}, "item 1 must be at most 80 characters"},
		{"too many items", mt, []any{"a", "b", "c", "d", "e"}, "must have at most 4 items"},
		{"NFC duplicates", mt, []any{"\u00e9", "e\u0301"}, "item 2 repeats an earlier item"},
		{"duplicates after trim", mt, []any{"a", " a"}, "item 2 repeats an earlier item"},
		{"duplicates after a style selector", mt, []any{"\u2714 Done", "\u2714\uFE0F Done"}, "item 2 repeats an earlier item"},
		{"ZWJ inside a word", mt, []any{"Ye\u200Ds"}, "item 1 holds U+200D"},
		{"ZWJ before a word", mt, []any{"\u200DYes"}, "item 1 holds U+200D"},
		{"ZWJ after an emoji", mt, []any{"\U0001F44D\u200D"}, "item 1 holds U+200D"},
		{"other variation selector", mt, []any{"a\uFE00"}, "item 1 holds U+FE00"},
		{"line separator", mt, []any{"a\u2028b"}, "item 1 holds U+2028"},
		{"zero-width space", mt, []any{"a\u200bb"}, "item 1 holds U+200B"},
		{"tag character", mt, []any{"a\U000E0041"}, "item 1 holds U+E0041"},
		{"bidi override", mt, []any{"a\u202eb"}, "item 1 holds U+202E"},
		{"newline inside", mt, []any{"a\nb"}, "item 1 holds U+000A"},
		{"invalid UTF-8", mt, []any{"a\xffb"}, "item 1 must be valid UTF-8"},
		{"empty required list", required, []any{}, `property "options" is required`},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateProps(tc.mt, map[string]any{"options": tc.in}, 1<<14)
			d := requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
			require.Contains(t, d.Error(), tc.reason)
			if tc.mt == mt {
				require.Contains(t, d.Error(), `property "options"`)
			}
		})
	}
	t.Run("an empty list is absent", func(t *testing.T) {
		t.Parallel()
		got, err := validateProps(mt, map[string]any{"options": []any{}}, 1<<14)
		require.NoError(t, err)
		require.NotContains(t, got.props, "options")
	})
}

// TestPost_ListProp pins AC 1 and AC 3 of TKT-03FQPD through Service.Post:
// the list is stored normalized and read back as a list, a refusal names
// the property, and the type's `when:` reads the list with len.
func TestPost_ListProp(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(listRealtime))
	bob := user("bob")
	res, err := h.svc.Post(as(bob), room, PostInput{Type: "ask", Props: map[string]any{
		"question": "Which?", "options": []any{"Yes", " No "},
	}})
	require.NoError(t, err)
	require.Equal(t, []any{"Yes", "No"}, res.Message.Props["options"])

	msgs, err := h.store.Store.Messages(context.Background(), roomKey(), channels.Page{})
	require.NoError(t, err)
	require.Equal(t, []any{"Yes", "No"}, msgs[len(msgs)-1].Props["options"], "the store keeps the normalized list")

	read := h.readAll(t, bob, room)
	require.Equal(t, []any{"Yes", "No"}, read.Messages[len(read.Messages)-1].Props["options"])

	_, err = h.svc.Post(as(bob), room, PostInput{Type: "ask", Props: map[string]any{
		"question": "Which?", "options": "Yes",
	}})
	d := requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
	require.Contains(t, d.Error(), `property "options": must be a list of strings`)

	// The when: holds for three options and fails for four.
	_, err = h.svc.Post(as(bob), room, PostInput{Type: "ask", Props: map[string]any{
		"question": "Which?", "options": []any{"a", "b", "c"},
	}})
	require.NoError(t, err)
	_, err = h.svc.Post(as(bob), room, PostInput{Type: "ask", Props: map[string]any{
		"question": "Which?", "options": []any{"a", "b", "c", "d"},
	}})
	requireDenial(t, err, http.StatusForbidden, TypePostDenied)

	// A required list refuses an empty array as absent.
	_, err = h.svc.Post(as(bob), room, PostInput{Type: "must", Props: map[string]any{"picks": []any{}}})
	d = requireDenial(t, err, http.StatusBadRequest, TypeInvalid)
	require.Contains(t, d.Error(), `property "picks" is required`)
}

// TestWhenHolds_List pins AC 3: a `when:` over a list compiles and holds
// on the stored form, and an absent list binds as nil.
func TestWhenHolds_List(t *testing.T) {
	t.Parallel()
	c := testCompiled(t, listRealtime)
	ct, _ := c.ChannelType("room")
	mt := ct.MessageTypes["ask"]
	now := time.Now()
	require.True(t, whenHolds(context.Background(), mt, map[string]any{"options": []any{"a", "b"}}, now))
	require.False(t, whenHolds(context.Background(), mt, map[string]any{"options": []any{"a", "b", "c", "d"}}, now))
	// len of an absent list does not evaluate, so the when: fails closed,
	// as any comparison with an absent value does.
	require.False(t, whenHolds(context.Background(), mt, map[string]any{}, now))
	_, isNil := predicateValue(listProp(), "not a list").(predicate.Nil)
	require.True(t, isNil, "a stored value that is not a list binds as nil")
}

// TestChoices_SecondAnswerWithoutSession pins AC 13: outside a session,
// two answers to one question are both stored. The server does not record
// that a question was answered.
func TestChoices_SecondAnswerWithoutSession(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(listRealtime))
	bob, carol := user("bob"), user("carol")
	q := h.post(t, bob, PostInput{Type: "ask", Props: map[string]any{
		"question": "Which?", "options": []any{"A", "B"},
	}})
	first := h.post(t, carol, PostInput{Type: "answer", Props: map[string]any{"body": "A"}, ReplyTo: q.Seq})
	second := h.post(t, carol, PostInput{Type: "answer", Props: map[string]any{"body": "B"}, ReplyTo: q.Seq})
	require.Equal(t, q.Seq, first.ReplyTo)
	require.Equal(t, q.Seq, second.ReplyTo)
	require.NotEqual(t, first.Seq, second.Seq)
}
