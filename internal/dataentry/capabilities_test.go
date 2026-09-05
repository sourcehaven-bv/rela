package dataentry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestLuaCapabilities_CarriesEveryGrant pins that the YAML→runtime translation
// is TOTAL: every field an operator can write in a `capabilities:` block
// arrives at the runtime.
//
// It asserts on the whole struct rather than field by field, deliberately. The
// defect this translation seam exists to prevent (TKT-YH52OM) was a capability
// silently dropped on one surface, and a field-by-field test cannot fail for a
// field nobody thought to add — which is exactly the case that matters. A whole
// -struct comparison against an explicit expectation does.
func TestLuaCapabilities_CarriesEveryGrant(t *testing.T) {
	t.Parallel()

	got := luaCapabilities(metamodel.Capabilities{
		HTTP:      true,
		AI:        true,
		Mail:      true,
		WriteFile: true,
		Secrets:   []string{"slack_webhook_url"},
	})

	require.Equal(t, lua.Capabilities{
		HTTP:      true,
		AI:        true,
		Mail:      true,
		WriteFile: true,
		Secrets:   []string{"slack_webhook_url"},
	}, got)
}

// TestLuaCapabilities_EmptyBlockGrantsNothing is the fail-closed half.
//
// AllSecrets gets its own assertion because it is the one field with no YAML
// spelling: it must stay false however the block is written, or an operator
// could hand a network-invoked script the whole of .rela/secrets.yaml.
func TestLuaCapabilities_EmptyBlockGrantsNothing(t *testing.T) {
	t.Parallel()

	got := luaCapabilities(metamodel.Capabilities{})

	require.False(t, got.Any(), "an omitted capabilities block must grant nothing")
	require.False(t, got.AllSecrets, "AllSecrets must never be reachable from config")
}
