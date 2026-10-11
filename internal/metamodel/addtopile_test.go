package metamodel_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddToPile_Parses(t *testing.T) {
	m := mustParse(t, `
automations:
  - name: inbox
    on: {entity: ticket, property: status, becomes: open}
    do:
      - add_to_pile: {pile: Inbox, owner: "{{new.assignee}}"}
      - add_to_pile: {pile: Later, create: false}
`)
	require.Len(t, m.Automations, 1)
	do := m.Automations[0].Do
	require.Len(t, do, 2)
	require.NotNil(t, do[0].AddToPile)
	require.Equal(t, "Inbox", do[0].AddToPile.Pile)
	require.Equal(t, "{{new.assignee}}", do[0].AddToPile.Owner)
	require.True(t, do[0].AddToPile.CreatePile(), "create defaults to true")
	require.False(t, do[1].AddToPile.CreatePile())
}

func TestAddToPile_Validation(t *testing.T) {
	for _, tc := range []struct {
		name, action, want string
	}{
		{"missing pile", `{add_to_pile: {owner: "{{new.assignee}}"}}`, "add_to_pile requires `pile:`"},
		{"blank pile", `{add_to_pile: {pile: "  "}}`, "add_to_pile requires `pile:`"},
		{"shared with set", `{add_to_pile: {pile: Inbox}, set: status, value: open}`, "cannot share a list entry with set"},
		{"shared with lua", `{add_to_pile: {pile: Inbox}, lua: "x = 1"}`, "cannot share a list entry with lua"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(t, `
automations:
  - name: inbox
    on: {entity: ticket, created: true}
    do:
      - `+tc.action+`
`)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
