package metamodel_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const pilesBlock = `
piles:
  favorites:
    name: Favorites
    icon: star
  inbox:
    name: Notifications
    icon: bell
    on_repeat: bump
    read_state: true
    auto_read: on_open
    clear_read_after: 7d
    expire_after: 2w
    reasons:
      done: completed a task
`

func TestPiles_Parses(t *testing.T) {
	m := mustParse(t, pilesBlock)
	require.Equal(t, []string{"favorites", "inbox"}, m.PileKeys())
	fav, inbox := m.Piles["favorites"], m.Piles["inbox"]
	require.False(t, fav.Bump())
	require.False(t, fav.ReadsOnOpen())
	require.Zero(t, fav.ExpireDuration())
	require.True(t, inbox.Bump())
	require.True(t, inbox.ReadsOnOpen())
	require.Equal(t, 7*24*time.Hour, inbox.ClearReadDuration())
	require.Equal(t, 14*24*time.Hour, inbox.ExpireDuration())
	require.Equal(t, "completed a task", inbox.Reasons["done"])
}

func TestParsePileDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"": 0, "30m": 30 * time.Minute, "12h": 12 * time.Hour, "1h30m": 90 * time.Minute,
		"3d": 72 * time.Hour, "2w": 14 * 24 * time.Hour,
	} {
		got, err := metamodel.ParsePileDuration(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}
	for _, in := range []string{"0d", "-1h", "1.5d", "soon", "0s", "d"} {
		_, err := metamodel.ParsePileDuration(in)
		require.Error(t, err, in)
	}
}

func TestPiles_Validation(t *testing.T) {
	for _, tc := range []struct{ name, block, want string }{
		{"bad key", "piles:\n  Fav:\n    name: F\n", "piles.Fav: a pile key"},
		{"no name", "piles:\n  fav:\n    icon: star\n", "piles.fav: name is required"},
		{"bad on_repeat", "piles:\n  fav:\n    name: F\n    on_repeat: twice\n", "on_repeat must be"},
		{"bad auto_read", "piles:\n  fav:\n    name: F\n    read_state: true\n    auto_read: always\n", "auto_read must be"},
		{"auto_read needs state", "piles:\n  fav:\n    name: F\n    auto_read: on_open\n", "auto_read needs read_state"},
		{"clear needs state", "piles:\n  fav:\n    name: F\n    clear_read_after: 1d\n", "clear_read_after needs read_state"},
		{"bad duration", "piles:\n  fav:\n    name: F\n    expire_after: soon\n", "expire_after:"},
		{"bad reason key", "piles:\n  fav:\n    name: F\n    reasons:\n      Done: x\n", "reason key \"Done\""},
		{"empty reason label", "piles:\n  fav:\n    name: F\n    reasons:\n      done: \"\"\n", "reason \"done\" needs a label"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(t, tc.block)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestNotify_Parses(t *testing.T) {
	m := mustParse(t, pilesBlock+`
relations:
  owned-by:
    from: [ticket]
    to: [person]
automations:
  - name: tell owner
    on: {entity: ticket, property: title, becomes: done}
    do:
      - notify: {pile: inbox, reason: done, to: {via: owned-by, property: name}}
      - notify:
          pile: inbox
          to: {via: [owned-by], type: person, property: name}
`)
	do := m.Automations[0].Do
	require.Equal(t, "inbox", do[0].Notify.Pile)
	require.Equal(t, metamodel.StringOrSlice{"owned-by"}, do[0].Notify.To.Via)
	require.Equal(t, "name", do[0].Notify.To.Property)
	require.Equal(t, "person", do[1].Notify.To.Type)
}

func TestNotify_Validation(t *testing.T) {
	for _, tc := range []struct{ name, on, action, want string }{
		{"no pile", "{entity: ticket, created: true}", `{notify: {to: {property: title}}}`, "notify requires `pile:`"},
		{"undeclared pile", "{entity: ticket, created: true}", `{notify: {pile: nope, to: {property: title}}}`,
			`pile "nope" is not declared`},
		{"no read state", "{entity: ticket, created: true}", `{notify: {pile: favorites, to: {property: title}}}`,
			"needs `read_state: true`"},
		{"unknown reason", "{entity: ticket, created: true}",
			`{notify: {pile: inbox, reason: other, to: {property: title}}}`, `reason "other" is not one of`},
		{"no property", "{entity: ticket, created: true}", `{notify: {pile: inbox, to: {}}}`,
			"requires `to.property:`"},
		{"via without entity", "{created: true}", `{notify: {pile: inbox, to: {via: x, property: title}}}`,
			"needs the automation's `on.entity:`"},
		{"type without via", "{entity: ticket, created: true}",
			`{notify: {pile: inbox, to: {type: person, property: title}}}`, "applies only with `to.via:`"},
		{"shared with set", "{entity: ticket, created: true}",
			`{notify: {pile: inbox, to: {property: title}}, set: title, value: x}`, "cannot share a list entry with set"},
		{"shared with add_to_pile", "{entity: ticket, created: true}",
			`{notify: {pile: inbox, to: {property: title}}, add_to_pile: {pile: X}}`,
			"cannot share a list entry with add_to_pile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(t, pilesBlock+`
automations:
  - name: n
    on: `+tc.on+`
    do:
      - `+tc.action+`
`)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
