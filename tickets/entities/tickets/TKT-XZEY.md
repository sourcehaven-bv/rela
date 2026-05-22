---
id: TKT-XZEY
type: ticket
title: 'ACL v0.5: extend WriteRequest for parameterised verbs (transitions, relations)'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Goal

Extend ACL v0's `WriteRequest{Op, EntityType, RelationType}` so it can represent
**parameterised verbs** — workflow transitions (`transition:done`,
`transition:cancel`) and relation operations (`relation:depends-on:add`,
`relation:depends-on:remove`) — without collapsing them under `OpUpdate`.
Required before TKT-Y72A's phase 1 verb vocabulary can grow to cover those
operations.

## Why this is its own ticket

The action-affordances design audit (cranky #1, architect C1) surfaced that ACL
v0's `Op` enum is exactly `{create, update, delete, rename}`. The phase-1
`_actions` map was deliberately scoped to that closed set so the cardinal rule
("`_actions[v]==false` ⇒ 403 on write") would be structurally honest. Mapping
`transition:done` to `OpUpdate` would have collapsed every workflow transition's
verdict to a single `update` boolean — not the per-transition gating the UI
needs.

## Options to evaluate during design

1. **Add `Op` variants** — `OpTransition`, `OpRelationAdd`,
`OpRelationRemove`. Easy to enumerate; clean per-op switch in `Declarative`.
Adds combinatorial Op constants if more parameter shapes follow.
2. **Add an extension field** — `WriteRequest.Args
map[string]string` (or similar). One Op shape, arbitrary parameters. Looser
typing; `Declarative` evaluates args per Op.
3. **Subject identity** — `WriteRequest.EntityID string` so the
policy can gate per-row (assignee-only transitions, etc.). Orthogonal to
(1)/(2); needed if ACL v1 lands per-row rules.

Pick during the design phase. Each option has consequences for how
`translateVerb` (in `internal/dataentry/affordances.go`) extends.

## Out of scope

- ACL v1 per-row rules (separate ticket).
- Read-side filtering / property redaction.
- Snapshot threading through `AuthorizeWrite` (flagged as a v1
concern in the design doc Q4; still relevant when row context matters, but not
blocking on this ticket).

## References

- Phase-1 implementation: TKT-Y72A, PR #779
- Design: `.ignored/action-affordances-design.md` §"Phase 1 verb set"
- Research: `.ignored/action-affordances-research.md`
- ACL v0: TKT-GN5LN
