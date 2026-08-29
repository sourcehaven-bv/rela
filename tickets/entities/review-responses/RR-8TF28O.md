---
id: RR-8TF28O
type: review-response
title: handlerSet godoc documented only the gocritic reason, not the anti-partial-wiring property it protects
finding: The handlerSet godoc said the grouping exists 'so Deps.handlers has one return value rather than one per group' — the gocritic reason. The load-bearing reason is that whole-struct assignment makes a partially-wired Server unconstructible (the hazard RR-OSJQWC flagged on the parent PR). As written it reads like a style concession a later refactor could reverse without realising what it gives up.
severity: minor
resolution: 'Rewrote the godoc to lead with the safety property: single whole-struct assignment at every construction site, adding a seventh group is picked up automatically, and per-field assignment would compile then nil-panic at request time. Ends with an explicit ''keep it one value; do not spread these back out into separate returns.'''
status: addressed
---

Nit from the TKT-MGNE5L code review (cranky-code-reviewer, PR #1468). Reviewer
verified: pure move after normalization; get_metamodel/get_schema aliasing
preserved AND now defended by dispatch_test.go's inventory test rather than
prose alone; no principal in any handler struct; all handlers hold the narrow
GraphReader (resources/prompts narrowed their reach, same gate); handlerSet
promotes zero methods so the plimsoll reduction is real, not accounting; no
field-name collisions; TestLuaToolsHoldNoAmbientCapabilities did not get
orphaned by the move; directive 25 matches; zero assertion/golden changes.
