---
id: RR-9BU9GN
type: review-response
title: Stale dr.pending misattributed a later error after a pcall-swallowed luaFail
finding: The typed-BuildError stash (dr.pending) set by luaFail could be left set if an island wrapped a failing resolver in pcall (pcall/error are in the sandbox). A later, unrelated genuine error in the same island then surfaced the stale pending error — wrong message, kind, and line.
severity: significant
resolution: wrapLuaErr now only trusts dr.pending when the raised Lua error message still contains the stashed message (strings.Contains). A pcall-swallowed resolver failure followed by a real error() falls through to report the real failure. Also clears dr.pending unconditionally each call so it can't leak.
status: addressed
---
