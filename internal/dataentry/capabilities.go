package dataentry

import (
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// luaCapabilities translates an operator-authored `capabilities:` block into
// the runtime's capability grant (TKT-YH52OM).
//
// The mapping is deliberately total and mechanical: every field is carried
// across and nothing is inferred. In particular lua.Capabilities.AllSecrets is
// NEVER set here — it is the operator-shell-only grant (lua.TrustedCapabilities)
// and must not be reachable from a config file, or an operator could hand a
// network-invoked script the whole of .rela/secrets.yaml with one key.
func luaCapabilities(c metamodel.Capabilities) lua.Capabilities {
	http, ai, writeFile, secrets := c.Fields()
	return lua.Capabilities{HTTP: http, AI: ai, WriteFile: writeFile, Secrets: secrets}
}
