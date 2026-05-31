---
id: RR-LH5W
type: review-response
title: 'Cranky #1: dead nil-guard on s.paths in Deps()'
finding: 'mcp_wiring.go Deps(): the `if s.paths != nil` guard cannot trigger — project.Discover never returns (nil, nil), so paths is always non-nil by the time any *mcpServices exists. Cargo-culted from the identical guard in luaReadDeps.'
severity: nit
reason: Left as-is for consistency with the sibling luaReadDeps guard; removing one of a matched pair adds asymmetry for a nit. Both guards dissolve when the composition-root unification follow-up collapses cli/mcp_wiring.
status: wont-fix
---
