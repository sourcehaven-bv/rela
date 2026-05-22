---
id: TKT-Y72A
type: ticket
title: 'Response-level action affordances: backend declares per-resource verbs to drive UI'
kind: enhancement
priority: medium
effort: xl
status: done
---

## Scope (phase 1 — this ticket)

Backend-driven per-resource `_actions: map[string]bool` map on every data-entry
HTTP response. The Vue SPA reads it to decide which write controls to render;
the server re-authorizes every write so the map is purely a UI hint.

**Phase 1 verb vocabulary** (matches `acl.Op` exactly):

| Verb | Scope | `acl.Op` |
|---|---|---|
| `create` | per-collection | `OpCreate` |
| `update` | per-item | `OpUpdate` |
| `delete` | per-item | `OpDelete` |
| `rename` | per-item | `OpRename` |

**Single source of truth.** `translateVerb` in
`internal/dataentry/affordances.go` is the only site in `internal/ dataentry`
that constructs `acl.WriteRequest{Op:...}`. A grep test enforces this. A
bidirectional contract test asserts the "`_actions[v]==false` ⇒ 403" invariant
end-to-end across NopACL and ReadOnlyACL.

**Anonymous fallback.** Anonymous requests get the `_actions` field omitted; the
SPA falls back to "render all + warn." Authenticated principals always get the
field (possibly `{}`).

## Why this design (vs. v1 of the planning doc)

`{verb: bool}` map was chosen over `[allowed_verb, ...]` list — the map is a
closed-world assertion (every verb the server defines is a key in the response)
that lets the SPA distinguish *denied* from *not evaluated* / *not defined* /
*old server*. List-of-allowed collapses all four into "absent."

Parallel-emit (`_actions_v2`) and a separate `/meta/actions` discovery endpoint
were both designed and then rejected during crit: rela's server and SPA ship
together (no backwards-compat needed), and collection-level `_actions.create`
covers menu population without a dedicated endpoint.

## Out of scope (deferred to follow-up tickets)

- **Frontend consumption + AWM6L payoff.** Vue components currently
ignore `_actions`; phase-2 ticket adds the button-omission + dev-mode warning +
E2E for read-only-mode-has-no-write-buttons.
- **`transition:*` and `relation:*` verbs.** ACL v0's
`WriteRequest{Op, EntityType, RelationType}` doesn't carry parameters (workflow
target state, relation type). Gated on ACL v0.5 (separate follow-up ticket).
- **List-endpoint profile / cache.** N+1 risk on big lists is
acknowledged in the design doc; profile gate lives in the phase-2 ticket. Cache
key is reserved as `(principal_id, entity_id, entity_updated_at)`; not
implemented in phase 1.
- **Per-property affordances, MCP affordance integration, SSE
policy-changed events** — all deferred to later tickets.

## References

- Supersedes: TKT-AWM6L (wont-fix)
- Builds on: TKT-GN5LN (ACL v0)
- Design: `.ignored/action-affordances-design.md`
- Research: `.ignored/action-affordances-research.md`
- Implementation PR: #779
