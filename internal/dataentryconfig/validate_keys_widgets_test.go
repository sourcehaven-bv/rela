package dataentryconfig

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateConfig_ReservedListKey pins the message for an action bound to
// a reserved list key: it names the key, the action and the list that offers
// it, and lists the keys to avoid.
func TestValidateConfig_ReservedListKey(t *testing.T) {
	for _, key := range reservedListKeyOrder {
		t.Run(key, func(t *testing.T) {
			cfg := fmt.Sprintf(`
version: "1.0"
actions:
  archive:
    label: Archive
    key: %s
    set: {status: closed}
lists:
  inbox:
    entity_type: ticket
    columns: [{property: title}]
    actions: [archive]
`, key)
			_, err := loadAndValidate(t, cfg)
			require.Error(t, err)
			msg := err.Error()
			assert.Contains(t, msg, fmt.Sprintf(`actions: "archive" declares key %q and offered by list "inbox"`, key))
			assert.Contains(t, msg, "reserved list shortcut ("+reservedListKeys[key]+")")
			assert.Contains(t, msg, "pick a key other than j, k, o, e, n, h, l")
		})
	}

	_, err := loadAndValidate(t, `
version: "1.0"
actions:
  archive: {label: Archive, key: a, set: {status: closed}}
lists:
  inbox: {entity_type: ticket, columns: [{property: title}], actions: [archive]}
`)
	require.NoError(t, err, "a key that is not reserved loads")
}

// TestReservedListKeysMatchSPA pins the reserved keys to the shortcuts the
// list keyboard handler binds, so a new SPA shortcut cannot be shadowed by an
// action key the server accepted.
func TestReservedListKeysMatchSPA(t *testing.T) {
	src, err := os.ReadFile("../../frontend/src/composables/useListKeyboard.ts")
	require.NoError(t, err)
	assert.Len(t, reservedListKeys, len(reservedListKeyOrder))
	for _, key := range reservedListKeyOrder {
		assert.Contains(t, string(src), "case '"+key+"':", "the SPA binds %q", key)
		assert.Contains(t, reservedListKeys, key)
	}
}

// TestValidateConfig_SelectWidgetDeprecated pins the message for an explicit
// select widget on a form property field: it names the form, the field and
// the line to remove, and says what the form shows instead.
func TestValidateConfig_SelectWidgetDeprecated(t *testing.T) {
	for _, tc := range []struct{ widget, property, where string }{
		{"select", "status", "field[1]"},
		{"multi-select", "tags", "field[1]"},
	} {
		t.Run(tc.widget, func(t *testing.T) {
			cfg := fmt.Sprintf(`
version: "1.0"
forms:
  new_ticket:
    entity_type: ticket
    title: New ticket
    fields:
      - {property: title}
      - {property: %s, widget: %s}
`, tc.property, tc.widget)
			_, err := loadAndValidate(t, cfg)
			require.Error(t, err)
			msg := err.Error()
			for _, want := range []string{
				fmt.Sprintf(`form "new_ticket": %s property %q`, tc.where, tc.property),
				fmt.Sprintf(`remove the "widget: %s" line from this field`, tc.widget),
				"a select for a property with values, a multi-select for a list property",
			} {
				assert.Contains(t, msg, want)
			}
		})
	}

	t.Run("wizard step", func(t *testing.T) {
		_, err := loadAndValidate(t, `
version: "1.0"
forms:
  new_ticket:
    entity_type: ticket
    title: New ticket
    steps:
      - title: One
        fields: [{property: title}, {property: status, widget: select}]
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `form "new_ticket": step[0] field[1] property "status"`)
	})

	t.Run("no widget loads", func(t *testing.T) {
		_, err := loadAndValidate(t, `
version: "1.0"
forms:
  new_ticket:
    entity_type: ticket
    title: New ticket
    fields: [{property: title}, {property: status}, {property: tags}]
`)
		require.NoError(t, err)
	})
}
