---
id: RR-N9F6LB
type: review-response
title: Resolver (luaFail) errors reported the island's first line, not the resolver's actual line
finding: wrapLuaErr computed the correct frame-adjusted manual line but the pending branch overwrote it with seg.line (the island start). So a hand-written Lua error() reported the right line but every resolver error pointed at the top of the island — inconsistent fail-loud accuracy, undercutting the design goal.
severity: significant
resolution: The pending branch now uses the already-computed frame-adjusted `line` instead of seg.line. TestBuild_FailLoudLuaLineOffset asserts a Lua error on the 2nd body line of a multi-line island reports the correct manual line; TestBuild_FailLoudUnknownType asserts the resolver-error line + clean message + kind=resolve.
status: addressed
---
