package metamodel

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestCapabilities_DecodesEveryGrant pins the YAML spellings an operator
// writes. A renamed or mistyped key would otherwise decode to the zero value
// and silently deny a capability the operator granted — a failure that looks
// like a bug in the script, not in the config.
func TestCapabilities_DecodesEveryGrant(t *testing.T) {
	t.Parallel()

	var got Capabilities
	require.NoError(t, yaml.Unmarshal([]byte(`
http: true
ai: true
mail: true
write_file: true
secrets: [slack_webhook_url]
`), &got))

	require.Equal(t, Capabilities{
		HTTP: true, AI: true, Mail: true, WriteFile: true,
		Secrets: []string{"slack_webhook_url"},
	}, got)
}

// TestCapabilities_OmittedBlockGrantsNothing is the fail-closed rule: a
// capability is present only where an operator wrote it down.
func TestCapabilities_OmittedBlockGrantsNothing(t *testing.T) {
	t.Parallel()

	var got Capabilities
	require.NoError(t, yaml.Unmarshal([]byte("{}"), &got))
	require.False(t, got.Any())
}

// TestCapabilities_FieldsCoversEveryField is the guard that makes this type's
// central design property real rather than aspirational.
//
// [Capabilities.Fields] is documented as the SINGLE translation seam, and the
// reason it exists is that adding a capability should break the build at every
// consumer. That only holds if Fields actually returns every field — a
// capability added to the struct but not to Fields would compile fine
// everywhere and be silently dropped on every surface, which is precisely the
// defect the seam was introduced to prevent (TKT-YH52OM) and the one that let
// mail.send stay ungated (TKT-JVHSOZ).
//
// Reflection rather than an enumerated list, because a hand-written list is
// one more place to forget the new field.
func TestCapabilities_FieldsCoversEveryField(t *testing.T) {
	t.Parallel()

	// Every bool field set, so a dropped one shows as a false in the results.
	all := Capabilities{HTTP: true, AI: true, Mail: true, WriteFile: true, Secrets: []string{"k"}}

	structFields := reflect.TypeOf(all).NumField()
	http, ai, mail, writeFile, secrets := all.Fields()

	returned := []any{http, ai, mail, writeFile, secrets}
	require.Len(t, returned, structFields,
		"Capabilities.Fields must return one value per struct field — a capability "+
			"missing from the seam is dropped silently on every surface")

	for i, v := range returned {
		require.NotZero(t, v, "Fields() result %d is zero although the input granted it", i)
	}
}

// TestCapabilities_RefusesBareBoolean pins that `capabilities: true` is an
// error rather than a grant-everything shorthand. For a privilege field, the
// broadest reading is the wrong default — and here "everything" would include
// the whole secrets file.
func TestCapabilities_RefusesBareBoolean(t *testing.T) {
	t.Parallel()

	var got Capabilities
	err := yaml.Unmarshal([]byte("true"), &got)
	require.Error(t, err)
	require.ErrorContains(t, err, "must be a mapping")
	require.False(t, got.Any(), "a refused block must not have granted anything")
}
