---
id: RR-CAPMCP
type: review-response
title: 'MCP no-capability posture was a comment, not an enforced property'
finding: |-
    mcp/tools_lua.go states that Deps.LuaWriteDeps.Capabilities 'is the zero value here and must stay that way'. A comment cannot fail a build.

    The edge is sharpened by the RR-CAPSCH fix: because an empty WithCapabilities is now a no-op rather than a revocation, if a deps-carried grant ever appeared on Deps.LuaWriteDeps, lua_eval/lua_run would inherit it and NO option at the call site could take it back. Since TKT-BDG8U9 the MCP endpoint can also be mounted over HTTP, so these tools are reachable off-host.
resolution: |
    Added TestLuaToolsHoldNoAmbientCapabilities (internal/mcp), which asserts Deps.LuaWriteDeps carries no grant. Populating it at a wiring site now fails the build's test gate and forces the author to reckon with lua_eval inheriting it. The call-site comments were also updated to note the HTTP reachability.
severity: significant
status: addressed
---
