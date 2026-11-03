package realtime

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// literalParityTypes is a message type note with a required string under
// a max, an enum, a markdown and a when:, under a small message_bytes.
const literalParityTypes = `
    message_types:
      done: { post: [member] }
      note:
        post: [member]
        when: "message.body ~= 'nope'"
        properties:
          body: { type: string, required: true, max: 10 }
          sev:  { type: severity }
          text: { type: markdown }
`

// literalParityReminder is a file whose one reminder posts note with the
// literal props.
func literalParityReminder(props string) string {
	return `version: 1
limits: { message_bytes: 96 }
channel_types:
  br:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }` + literalParityTypes + `    session:
      initial: open
      states:
        open: { on: { done: { to: closed } } }
        closed: { final: true }
      reminders:
        r: { in: [open], every: 1h, post: { type: note, props: ` + props + ` } }
`
}

// literalParityTrigger is a file whose one trigger posts note with the
// literal props.
func literalParityTrigger(props string) string {
	return `version: 1
limits: { message_bytes: 96 }
channel_types:
  room:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }` + literalParityTypes + `triggers:
  opened:
    entity_type: ticket
    on: enter
    when: "entity.status == 'open'"
    channel: room
    post: { type: note, props: ` + props + ` }
`
}

// flowMap renders m as a YAML flow map of single-quoted strings.
func flowMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": '"+strings.ReplaceAll(m[k], "'", "''")+"'")
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// TestLiteralLoadPassesFireTime pins that the load check of a trigger's
// or reminder's literal props is the fire-time check: every literal
// realtimecfg.Parse accepts passes reminderProps for a reminder, and
// validateProps then whenHolds (the first steps of systemWriter.post) for
// a trigger. The loads column pins which cases load, so the test cannot
// pass by refusing everything (TKT-852C0M).
func TestLiteralLoadPassesFireTime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		lit   map[string]string
		loads bool
	}{
		{"valid", map[string]string{"body": "hi", "sev": "high", "text": "**x**"}, true},
		{"empty string on a required string", map[string]string{"body": ""}, true},
		{"enum in values", map[string]string{"body": "hi", "sev": "low"}, true},
		{"undeclared", map[string]string{"body": "hi", "x": "y"}, false},
		{"enum outside values", map[string]string{"body": "hi", "sev": "mid"}, false},
		{"over max", map[string]string{"body": "abcdefghijk"}, false},
		{"missing required", map[string]string{"sev": "high"}, false},
		{"over message_bytes", map[string]string{"body": "hi", "text": strings.Repeat("x", 100)}, false},
		{"when false", map[string]string{"body": "nope"}, false},
	}
	meta := testMeta(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			props := flowMap(tc.lit)

			cfg, err := realtimecfg.Parse([]byte(literalParityReminder(props)), meta, nil)
			require.Equal(t, tc.loads, err == nil, "reminder file: err = %v", err)
			if err == nil {
				requireReminderFires(t, cfg)
			}

			cfg, err = realtimecfg.Parse([]byte(literalParityTrigger(props)), meta, nil)
			require.Equal(t, tc.loads, err == nil, "trigger file: err = %v", err)
			if err == nil {
				requireTriggerFires(t, cfg)
			}
		})
	}
}

var literalParityNow = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// requireReminderFires requires cfg's one reminder to pass reminderProps.
func requireReminderFires(t *testing.T, cfg *realtimecfg.Compiled) {
	t.Helper()
	ct, ok := cfg.ChannelType("br")
	require.True(t, ok)
	rs := ct.Session().Reminders()
	require.Len(t, rs, 1)
	_, err := reminderProps(context.Background(), cfg, ct, rs[0], literalParityNow)
	require.NoError(t, err, "a reminder that loads fails at fire time")
}

// requireTriggerFires requires cfg's one trigger to pass validateProps
// and whenHolds, with its props copied as the trigger engine copies them.
func requireTriggerFires(t *testing.T, cfg *realtimecfg.Compiled) {
	t.Helper()
	trs := cfg.Triggers()
	require.Len(t, trs, 1)
	tr := trs[0]
	ct, ok := cfg.ChannelType(tr.Channel)
	require.True(t, ok)
	mt := ct.MessageTypes[tr.PostType]
	require.NotNil(t, mt)
	in := make(map[string]any, len(tr.Props))
	for k, v := range tr.Props {
		in[k] = v
	}
	vp, err := validateProps(mt, in, cfg.Limits().MessageBytes)
	require.NoError(t, err, "a trigger that loads fails validateProps")
	require.True(t, whenHolds(context.Background(), mt, vp.props, literalParityNow), "a trigger that loads fails when:")
}
