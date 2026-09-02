---
id: RR-GE0DM1
type: review-response
title: appEntityWriter was threaded through App's 40-line doc comment
finding: The declaration landed far from its godoc and nearly detached App's plimsoll directive
severity: minor
resolution: Fixed during implementation - the block was moved above App's doc comment and directive/struct adjacency re-verified with just plimsoll.
status: addressed
---

The first placement of `appEntityWriter` inserted its godoc *inside* `App`'s
multi-paragraph doc comment, between the decomposition history and the
`//plimsoll:max-methods=86` directive — which detached the directive from `type
App struct` and failed `just plimsoll` (*"type App has 86 methods, over the load
line of 40"*).

Fixed by moving the whole block above `App`'s doc comment, and re-verified:
directive and struct are adjacent, plimsoll passes. Recorded because the failure
mode is silent in review — a directive one line out of place turns a
grandfathered type into a CI failure or silently un-pins it.
