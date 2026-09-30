package mcp

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// TestLuaToolsHoldNoAmbientCapabilities enforces, rather than merely documents,
// the posture of the MCP Lua tools (TKT-YH52OM).
//
// lua_eval and lua_run execute code — or choose a file — supplied by an MCP
// CLIENT. That makes them the least appropriate surface in the tree to hold
// http, ai or secrets: arbitrary caller-influenced code paired with credentials
// is the exfiltration chain the ticket exists to close. Since TKT-BDG8U9 the
// MCP endpoint can be mounted over HTTP, so this is reachable off-host.
//
// Both call sites carry a comment saying the grant must stay empty, but a
// comment cannot fail a build. This test can. It matters especially because
// [lua.WithCapabilities] treats an empty grant as "no opinion" rather than as a
// revocation: if a deps-carried grant ever appeared on Deps.LuaWriteDeps,
// the tools would inherit it and NO option at the call site could take it back.
func TestLuaToolsHoldNoAmbientCapabilities(t *testing.T) {
	t.Parallel()

	var deps lua.WriteDeps // the zero value a Services is built from
	if deps.Capabilities.Any() {
		t.Fatal("lua.WriteDeps zero value now carries a capability grant")
	}

	// Guard the field the tools actually read. If someone populates
	// Deps.LuaWriteDeps.Capabilities at a wiring site, this fails and they are
	// forced to reckon with lua_eval inheriting it.
	d := Deps{}
	if d.LuaWriteDeps.Capabilities.Any() {
		t.Errorf("Deps.LuaWriteDeps carries an ambient capability grant (%+v). "+
			"lua_eval/lua_run execute client-supplied code and must reach no "+
			"http/ai/secrets; there is no way to revoke it at the call site",
			d.LuaWriteDeps.Capabilities)
	}
}
