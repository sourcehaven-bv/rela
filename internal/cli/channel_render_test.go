package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// TestChannelRender_ControlCharsAndIndent pins that message content cannot
// steer the terminal or forge output: control and bidi characters are
// replaced, and every line of a value carries the prefix, so a value that
// spells a message header still reads as content.
func TestChannelRender_ControlCharsAndIndent(t *testing.T) {
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	view := realtime.ReadView{
		Channel: realtime.ChannelView{Anchor: "ITEM-1", Type: "room", State: realtime.StateOpen, Title: "T\x1b[2Jitle"},
		Messages: []realtime.MessageView{{
			Seq: 1, Type: "text", At: at,
			Author: &realtime.AuthorView{User: "mal\u202elory", Tool: "c\rli"},
			Props: map[string]any{
				"body":  "line one\r\n#12  2026-10-19T09:00:00Z  admin (cli)  text\nesc \x1b[31mred\x1b[0m",
				"about": &realtime.RefValue{ID: "DOC-1", Type: "doc", Title: "Plan\u202e"},
				"count": float64(3),
			},
		}},
	}
	var buf bytes.Buffer
	require.NoError(t, renderRead(&buf, view))
	got := buf.String()

	for _, bad := range []string{"\x1b", "\r", "\u202e"} {
		assert.NotContains(t, got, bad)
	}
	assert.Contains(t, got, "Title: T\uFFFD[2Jitle\n")
	assert.Contains(t, got, "#1  2026-10-19T09:00:00Z  mal\uFFFDlory (c\uFFFDli)  text\n")
	assert.Contains(t, got, propIndent+"body: line one\n"+
		propIndent+"#12  2026-10-19T09:00:00Z  admin (cli)  text\n"+
		propIndent+"esc \uFFFD[31mred\uFFFD[0m\n")
	assert.Contains(t, got, propIndent+`about: DOC-1 "Plan`+"\uFFFD\"\n")
	assert.Contains(t, got, propIndent+"count: 3\n")
	for line := range strings.SplitSeq(strings.TrimSpace(got), "\n") {
		if strings.HasPrefix(line, "#") {
			assert.True(t, strings.HasPrefix(line, "#1  "), "only the real header starts a line with #: %q", line)
		}
	}
}

func TestPrintable(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"a\nb", "a\uFFFDb"},
		{"tab\there", "tab\uFFFDhere"},
		{"bidi \u202e\u2066x\u2069", "bidi \uFFFD\uFFFDx\uFFFD"},
		{"zero\u200bwidth", "zero\uFFFDwidth"},
		{"family \U0001F468\u200d\U0001F469", "family \U0001F468\u200d\U0001F469"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, printable(tc.in), "%q", tc.in)
	}
}

// TestChannelRender_Removed pins the placeholders of removed messages, with
// who and why only when the read carried them.
func TestChannelRender_Removed(t *testing.T) {
	view := realtime.ReadView{
		Channel: realtime.ChannelView{Anchor: "ITEM-1", Type: "room", State: realtime.StateOpen},
		Messages: []realtime.MessageView{
			{Seq: 1, Removed: &realtime.RemovedView{Kind: "author"}},
			{Seq: 2, Removed: &realtime.RemovedView{Kind: "moderator"}},
			{Seq: 3, Removed: &realtime.RemovedView{
				Kind: "moderator", By: &realtime.AuthorView{User: "mod", Tool: "web"}, Reason: "spam\nreally",
			}},
			{Seq: 4, At: time.Unix(0, 0), Type: "text", Props: map[string]any{"about": &realtime.RefValue{Restricted: true}}},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, renderRead(&buf, view))
	got := buf.String()
	assert.Contains(t, got, "#1  [removed by author]\n")
	assert.Contains(t, got, "#2  [removed]\n")
	assert.Contains(t, got, "#3  [removed] by mod (web)\n"+propIndent+"reason: spam\n"+propIndent+"really\n")
	assert.Contains(t, got, propIndent+"about: (restricted)\n")
}

func TestChannelRender_List(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, renderChannelList(&buf, realtime.AnchorChannels{}))
	assert.Equal(t, "No channel types attach to this entity.\n", buf.String())

	buf.Reset()
	require.NoError(t, renderChannelList(&buf, realtime.AnchorChannels{Types: []realtime.TypeChannels{
		{Type: "room"},
		{Type: "threads", Create: true, Channels: []realtime.ChannelView{
			{Name: "a\tb", State: realtime.StateClosed, Title: "x", LastSeq: 4},
		}},
	}}))
	got := buf.String()
	assert.Regexp(t, `(?m)^room\s+-\s+-\s+-\s+-$`, got)
	assert.Regexp(t, `(?m)^threads\s+a\x{FFFD}b\s+closed\s+x\s+4$`, got)
	assert.Contains(t, got, "You may create: threads\n")
}
