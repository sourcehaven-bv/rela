---
id: TKT-LFT2
type: ticket
title: 'Action affordances phase 2: frontend consumption + AWM6L payoff'
kind: enhancement
priority: medium
effort: m
status: in-progress
---

## Goal

Make the Vue SPA actually consume the `_actions: map[string]bool` map shipped by
the backend in phase 1 (TKT-Y72A, PR #779). Read-only mode should produce a
button-less data-entry UI driven entirely by the backend's verdict — the
original deliverable TKT-AWM6L was chasing.

## Why now

Phase 1 landed the wire shape, the `translateVerb` source-of-truth, and the
bidirectional contract test. The SPA's type already declares `_actions?:
Record<string, boolean>` on `Entity` and `ListResponse`. What's missing is the
UI integration.

## Scope

- **Vue components consult `entity._actions[verb]`** at every write-
affordance call site (delete buttons, edit / update controls, create-new buttons
on list pages). Semantics:
  - `false` → omit the control.
  - `true` → render.
  - Absent → render unconditionally (defensive fallback for
pre-phase-1 servers or non-data-entry callers; the data-entry server always
emits the field).
- **Dev-mode warning.** When the SPA receives an authenticated
response without `_actions` in development builds, log a `console.warn` so a
future server-side regression (handler forgot to populate the field) is visible
at the edge. Suppressed in production builds.
- **Inventory the call sites first.** PLAN-B0CI's earlier 31-affordance
survey is a starting point; the actual phase-2 inventory may be smaller (phase-1
vocabulary covers `create`/`update`/`delete`/ `rename` only).
- **AC4 (list endpoint) — dedicated test.** Backend wiring already
exists (`computeCollectionActions`); add a handler test that asserts per-row
`_actions` differs across rows when the ACL gates by entity type / ID.
- **AC5 (frontend consumption) — component unit tests.** Fixture
responses with various `_actions` shapes; assert button render presence/absence.
- **AC6 (additive vocabulary) — synthetic verb test.** Backend emits
a synthetic verb `noop` from one handler; assert frontend doesn't crash or
console-warn on the unknown key.
- **AC7 (AWM6L payoff) — E2E.** Boot data-entry with `--read-only`;
navigate the SPA; assert no write controls render anywhere (delete button, edit
form fields, create button).

## Profile gate (decision)

Before merging, benchmark list response time at 100 / 1k / 10k entities × 3-verb
computation under `Declarative` ACL. If p95 > 200ms, land the per-row verdict
cache in this ticket; key shape is `(principal_id, entity_id,
entity_updated_at)`, TTL 60s, explicit invalidation on writes. Otherwise defer
the cache.

## Out of scope

- `transition:*` and `relation:*` verbs (gated on ACL v0.5 —
TKT-XZEY).
- MCP / Lua / scheduler write-path affordance integration.
- SSE policy-changed events.

## References

- Phase 1: TKT-Y72A, PR #779
- Phase 1 design: `.ignored/action-affordances-design.md`
- Predecessor (wont-fix): TKT-AWM6L
- Backend wire-shape demo: see TKT-Y72A done-status comment + curl
examples in `docs/data-entry/api-reference.md`
