package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode"

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
		{"line\u2028para\u2029", "line\uFFFDpara\uFFFD"},
		{"marks \u200e\u200f\u061c", "marks \uFFFD\uFFFD\uFFFD"},
		{"c1 \u009b", "c1 \uFFFD"},
		{"bom\ufeff", "bom\uFFFD"},
		{"kept: soft\u00adhyphen, zw\u200cnj", "kept: soft\u00adhyphen, zw\u200cnj"},
		{"word\u2060joiner, tag\U000E0041", "word\uFFFDjoiner, tag\uFFFD"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, printable(tc.in), "%q", tc.in)
	}
}

// TestTerminalSafeJSON pins that -o json escapes every control, format and
// separator rune, including the C1 CSI and supplementary format runes, and
// still decodes to the same value.
func TestTerminalSafeJSON(t *testing.T) {
	in := map[string]string{"body": "a\u009b[31m \u202eevil\u2066 \u00ad \U000E0041 \u2028 <ok> \U0001F600"}
	data, err := json.Marshal(in)
	require.NoError(t, err)
	safe := terminalSafeJSON(data)
	for _, r := range string(safe) {
		assert.False(t, unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp), "raw %U in %s", r, safe)
	}
	assert.Contains(t, string(safe), `\u009b`)
	assert.Contains(t, string(safe), `\udb40\udc41`)
	assert.Contains(t, string(safe), "\U0001F600")
	var back map[string]string
	require.NoError(t, json.Unmarshal(safe, &back))
	assert.Equal(t, in, back)
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

// TestRenderRead_Session pins the session line in the channel header and
// the one-line form of a notice.
// TestRenderRead_SessionTimer pins TKT-INM3AZ AC 19: the session line
// names the timer that comes due next, and a timer's notice names it.
func TestRenderRead_SessionTimer(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 12, 3, 0, time.UTC)
	due := at.Add(24 * time.Hour)
	view := realtime.ReadView{
		Channel: realtime.ChannelView{
			Anchor: "DL-1", Type: "breach-response", State: realtime.StateOpen, LastSeq: 1, Rev: 1,
			Session: &realtime.SessionView{
				State: "detected", Generation: 0, EnteredAt: at, DueAt: &due,
				Timers: []realtime.SessionTimerView{
					{Name: "later", To: "x", Since: "state", DueAt: new(due.Add(time.Hour))},
					{Name: "no-containment", To: "overdue", Since: "state", DueAt: &due},
				},
			},
		},
		Messages: []realtime.MessageView{{
			Seq: 1, Type: "session", At: due, TriggeredBy: "session:breach-response",
			Author: &realtime.AuthorView{User: "system:realtime", System: true},
			Props: map[string]any{
				"from": "detected", "to": "overdue", "generation": int64(1), "timer": "no-containment",
			},
		}},
	}
	var buf bytes.Buffer
	require.NoError(t, renderRead(&buf, view))
	got := buf.String()
	assert.Contains(t, got, "session: detected, generation 0, next: no-containment to overdue at 2026-10-22T09:12:03Z\n")
	assert.Contains(t, got, "  moved from detected to overdue (timer no-containment)\n")
	assert.NotContains(t, got, propIndent+"timer")

	closed := realtime.SessionView{State: "detected", Timers: []realtime.SessionTimerView{{Name: "no-containment"}}}
	assert.Equal(t, "session: detected, generation 0", sessionText(&closed), "no due time, no next")
}

// TestRenderRead_SessionReminder pins TKT-36Z6X3 AC 21: the session line
// lists each reminder with its count, max and next time, and a reminder
// message carries the reminder note.
func TestRenderRead_SessionReminder(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 12, 3, 0, time.UTC)
	due := at.Add(70 * time.Minute)
	view := realtime.ReadView{
		Channel: realtime.ChannelView{
			Anchor: "DL-1", Type: "breach-response", State: realtime.StateOpen, LastSeq: 1, Rev: 1,
			Session: &realtime.SessionView{
				State: "detected", Generation: 0, EnteredAt: at,
				Reminders: []realtime.SessionReminderView{
					{Name: "status-update", Every: "30m", Count: 2, Max: 48, DueAt: &due},
				},
			},
		},
		Messages: []realtime.MessageView{{
			Seq: 1, Type: "reminder", At: at.Add(40 * time.Minute),
			TriggeredBy: "reminder:breach-response/status-update",
			Author:      &realtime.AuthorView{User: "system:realtime", System: true},
			Props:       map[string]any{"body": "Post a status update"},
		}},
	}
	var buf bytes.Buffer
	require.NoError(t, renderRead(&buf, view))
	got := buf.String()
	assert.Contains(t, got,
		"session: detected, generation 0, reminder status-update: 2 of 48, next at 2026-10-21T10:22:03Z\n")
	assert.Contains(t, got, "  2026-10-21T09:52:03Z  system:realtime  reminder  (reminder)\n")

	done := realtime.SessionView{State: "detected", Reminders: []realtime.SessionReminderView{{Name: "n", Count: 3}}}
	assert.Equal(t, "session: detected, generation 0, reminder n: 3", sessionText(&done))
}

func TestRenderRead_Session(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 12, 3, 0, time.UTC)
	view := realtime.ReadView{
		Channel: realtime.ChannelView{
			Anchor: "DL-1", Type: "breach-response", State: realtime.StateOpen, LastSeq: 2, Rev: 2,
			Session: &realtime.SessionView{State: "contained", Label: "Contained", Generation: 1, EnteredAt: at},
		},
		Messages: []realtime.MessageView{
			{Seq: 1, Type: "contained", At: at, Author: &realtime.AuthorView{User: "alice"}},
			{
				Seq: 2, Type: "session", At: at, TriggeredBy: "session:breach-response",
				Author: &realtime.AuthorView{User: "system:realtime", System: true},
				Props:  map[string]any{"from": "detected", "to": "contained", "generation": int64(1), "cause": int64(1)},
			},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, renderRead(&buf, view))
	got := buf.String()
	assert.Contains(t, got, "DL-1 breach-response (open, last #2, rev 2)\nsession: contained (Contained), generation 1\n")
	assert.Contains(t, got, "  session\n  moved from detected to contained\n")
	assert.NotContains(t, got, propIndent+"cause")

	for _, tc := range []struct {
		name string
		s    realtime.SessionView
		want string
	}{
		{"final", realtime.SessionView{State: "notified", Label: "AP notified", Generation: 3, Final: true},
			"session: notified (AP notified), generation 3, final"},
		{"stranded", realtime.SessionView{State: "old", Label: "old", Generation: 2, Stranded: true},
			"session: old, generation 2, stranded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sessionText(&tc.s))
		})
	}
}
