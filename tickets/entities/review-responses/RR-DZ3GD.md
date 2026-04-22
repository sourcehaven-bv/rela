---
id: RR-DZ3GD
type: review-response
title: luaValueToGo called unconditionally in luaOutput even when muted
finding: internal/lua/runtime.go:737 computes goData before checking isAction/isDocument. In muted modes the value is discarded; a large nested Lua table pays for the conversion unnecessarily.
severity: nit
resolution: Moved luaValueToGo(data) past the isAction/isDocument guards in internal/lua/runtime.go:luaOutput. Muted modes no longer pay the conversion cost.
status: addressed
---

From post-impl cranky review.

Fix: move luaValueToGo after the mode guards. Small, local change.
