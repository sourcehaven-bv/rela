package realtimecfg

import (
	"reflect"
	"testing"
)

// waitIndexType is a channel type on attach whose session starts in
// initial; open waits for wait. Roles lead (entity.owner) and opener
// (creator) are declared; member may post anything.
func waitIndexType(name, attach, anchor, initial, wait string) string {
	return `  ` + name + `:
    attach_to: [` + attach + `]
    anchor: ` + anchor + `
    cardinality: many
    create: { roles: [member] }
    roles:
      lead:   { from: entity.owner }
      opener: { creator: true }
      member: { from: attached }
    message_types:
      answer: { properties: { note: { type: string } }, post: [lead, opener, member] }
      start:  { properties: { note: { type: string } }, post: [member] }
    session:
      initial: ` + initial + `
      states:
        new:  { on: { start: { to: open } } }
        open:
          waiting_for: [` + wait + `]
          on: { answer: { to: done } }
        done: { final: true }
`
}

// TestCompiledWaiting pins AC 1 of TKT-ZZ4PNX.
func TestCompiledWaiting(t *testing.T) {
	cases := []struct {
		name  string
		types string
		want  WaitingIndex
	}{
		{
			name:  "named role",
			types: waitIndexType("br", "datalek", "family", "new", "lead"),
			want: WaitingIndex{
				Pairs:  []WaitingPair{{EntityType: "datalek", Property: "owner", Types: []string{"br"}}},
				States: map[string][]string{"br": {"open"}},
			},
		},
		{
			name:  "creator role",
			types: waitIndexType("br", "datalek", "family", "new", "opener"),
			want: WaitingIndex{
				Creator: []string{"br"},
				States:  map[string][]string{"br": {"open"}},
			},
		},
		{
			name: "two types sharing a property",
			types: waitIndexType("br", "datalek, incident", "family", "new", "lead") +
				waitIndexType("esc", "datalek", "family", "new", "lead, opener"),
			want: WaitingIndex{
				Pairs: []WaitingPair{
					{EntityType: "datalek", Property: "owner", Types: []string{"br", "esc"}},
					{EntityType: "incident", Property: "owner", Types: []string{"br"}},
				},
				Creator: []string{"esc"},
				States:  map[string][]string{"br": {"open"}, "esc": {"open"}},
			},
		},
		{
			name: "face-anchored type",
			types: waitIndexType("review", "page", "face", "new", "lead") +
				waitIndexType("chat", "page", "family", "new", "lead"),
			want: WaitingIndex{
				Pairs: []WaitingPair{
					{EntityType: "page", Property: "owner", Types: []string{"chat"}},
					{EntityType: "page", Property: "owner", Face: true, Types: []string{"review"}},
				},
				States: map[string][]string{"chat": {"open"}, "review": {"open"}},
			},
		},
		{
			name:  "initial state waits",
			types: waitIndexType("br", "datalek", "family", "open", "lead"),
			want: WaitingIndex{
				Pairs:  []WaitingPair{{EntityType: "datalek", Property: "owner", Types: []string{"br"}}},
				States: map[string][]string{"br": {"", "open"}},
			},
		},
		{
			name: "final only",
			types: `  br:
    attach_to: [datalek]
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    session:
      initial: new
      states:
        new:  { on: { text: { to: done } } }
        done: { final: true }
`,
			want: WaitingIndex{States: map[string][]string{}},
		},
		{
			name: "no session",
			types: `  br:
    attach_to: [datalek]
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
`,
			want: WaitingIndex{States: map[string][]string{}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte("version: 1\nchannel_types:\n"+tc.types), testMeta(t), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got := c.Waiting()
			if len(got.Pairs) == 0 {
				got.Pairs = nil
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Waiting() =\n%#v\nwant\n%#v", got, tc.want)
			}
		})
	}
	t.Run("copy", func(t *testing.T) {
		c, err := Parse([]byte("version: 1\nchannel_types:\n"+waitIndexType("br", "datalek", "family", "open", "lead")),
			testMeta(t), nil)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		w := c.Waiting()
		w.Pairs[0].Types[0] = "x"
		w.States["br"][0] = "x"
		if again := c.Waiting(); again.Pairs[0].Types[0] != "br" || again.States["br"][0] != "" {
			t.Errorf("Waiting() shares state with a caller: %#v", again)
		}
	})
	t.Run("off", func(t *testing.T) {
		var c *Compiled
		if w := c.Waiting(); len(w.Pairs) != 0 || len(w.Creator) != 0 || len(w.States) != 0 {
			t.Errorf("nil Compiled: %#v", w)
		}
	})
}
