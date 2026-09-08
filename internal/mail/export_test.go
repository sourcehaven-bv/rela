package mail

import "github.com/Sourcehaven-BV/rela/internal/lua"

// ExportResolvePassword exposes credential resolution to the external test
// package. Kept in an _test.go file so it never reaches a production binary.
func ExportResolvePassword(c *Config) string { return c.resolvePassword() }

// ExportResolveAPIToken exposes the HTTP transport's credential resolution to
// the external test package. Kept in an _test.go file so it never reaches a
// production binary.
func ExportResolveAPIToken(c *Config) string { return c.resolveAPIToken() }

// ExportScriptCapsToLua exposes the send script's config→runtime capability
// translation to the external test package. Kept in an _test.go file so it
// never reaches a production binary.
func ExportScriptCapsToLua(c ScriptCapabilities) lua.Capabilities { return c.toLua() }
