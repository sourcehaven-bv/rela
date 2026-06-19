---
id: TKT-XZEY
type: ticket
title: 'ACL v0.5: extend WriteRequest for parameterised verbs (transitions, relations)'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Status

**Reframed twice (2026-05-21), now likely to be split or descoped.** See
"Reframes" below for the trajectory. Current direction:

- Per-property and per-value gates probably belong in **Lua
write-veto hooks** (new ticket), not in the declarative ACL.
- The declarative `acl.yaml` stays entity-type-grain (no per-prop /
per-value primitives needed).
- This ticket's residual scope is therefore much smaller: at most,
`relation:<type>:add/remove` as a wire-level verb, and possibly not even that.

## Original goal

Extend ACL v0's `WriteRequest{Op, EntityType, RelationType}` so it can represent
**parameterised verbs** that ACL v0's enum-of-4 can't. The original sketch (from
phase 1) was `transition:<state>` and `relation:<type>:add/remove`.

## Reframes

### Reframe #1 — `transition:` is not the right primitive (2026-05-21)

**User feedback:** "transition as currently specced makes no sense, status is
nothing special; so this should be possible for all enum fields right?"

Correct. The status property has no special status in the metamodel. A generic
`set-prop:<prop>:<value>` would cover any enum.

But the follow-up exposed the limit: "How would set-prop work with enum-list
fields? Or fields of other types?" Per-value gating only works for properties
with a discrete finite value set. Strings, numbers, dates, refs, markdown
content have unbounded cardinality; enum-lists are combinatorial. The verb
taxonomy explodes or collapses.

### Reframe #2 — fine-grained ACL probably belongs in Lua (2026-05-21)

**User insight:** "it feels like the prop/rel level stuff might be better
handled via lua."

Right framing. rela already runs Lua at write time (the automation engine) and
CLAUDE.md explicitly bans Lua on the read path for performance reasons.
Fine-grained ACL maps onto this existing shape cleanly:

| Concern | Where |
|---|---|
| Coarse "can write entity type X" | `acl.yaml` (declarative, unchanged) |
| Field-level "can set status=done" | Lua veto hook on write |
| Field-level "can change priority" | Lua veto hook |
| "Only assignee can mark done" | Lua veto hook |
| Relation add/remove gating | Lua veto hook (or wire-level verb, see below) |

The split lets `acl.yaml` stay fast + declarative (the 90% case) and pushes
programmable predicates into Lua where they belong (read entity properties,
consult the graph, check principal attributes — the kind of context-aware logic
Lua is for).

**Implications for `_actions` (TKT-Y72A's wire shape):**

- `_actions` stays coarse-grained — the existing 4 verbs cover the
declarative cases. No per-property / per-value verb explosion.
- The SPA renders fine-grained controls (dropdowns, buttons)
optimistically; the server's Lua hook decides on write; on deny, the existing
403 error path shows a toast with the Lua-supplied reason. This is the **Stripe
attempt-and-recover pattern** (research §8) — selectively, where the alternative
is verb- cardinality hell.
- UX cost: no "grey out the dropdown option" for fine-grained
cases; only-allowed users click and succeed, others click and see a 403 toast.
Acceptable trade for the architectural cleanliness.

## What this ticket might still cover

Two viable scopes, both smaller than the original:

**(a) Wire-level relation verbs only.** Add `relation:<type>:add` /
`relation:<type>:remove` to the verb vocabulary, gated by an `acl.yaml`
extension that lists allowed relation types per role. The relation widgets
(RelationCards, RelationPicker) hide their add/remove buttons accordingly. Same
pattern as phase 1 for entity-CRUD; just one more verb family. Lua hooks
complement this for "which targets are allowed."

**(b) Drop entirely.** Lua write-veto hooks cover the whole space including
relations; the SPA renders relation buttons optimistically and surfaces 403
toasts from Lua-supplied reasons. TKT-Y72A's `_actions` stays at 4 verbs
forever. The verb taxonomy never grows.

Choice between (a) and (b) is the central design question. (a) is more work
(`WriteRequest` extension + policy schema + SPA gating); (b) is simpler but
accepts attempt-and-recover for relations.

## Prerequisite: Lua write-veto hook (new ticket)

This work depends on a Lua hook the automation engine doesn't yet have:

- Lua script returns `{allow=false, reason="..."}` (or analogous)
from a `pre-write` hook
- `entitymanager.Manager` invokes the hook before the write, audits
the deny, returns `*acl.ForbiddenError` with the Lua-supplied reason
- New `acl.yaml` field (or `metamodel.yaml`? — design choice)
registers per-type or per-relation Lua scripts as write vetos

A separate ticket will spec this. Without it, this ticket can't proceed.

## Open design questions

Per the reframe, most original questions dissolve. What remains:

1. **(a) vs (b)** — wire-level relation verbs or full Lua delegation?
2. **Lua hook ergonomics** — what's the API surface? `rela.acl.veto(...)`,
a return-value convention, a new hook type? Goes in the prerequisite ticket but
informs this one.
3. **Audit log shape for Lua vetos** — `denied-write` row carries
the Lua-supplied reason in the existing `reason` field, or a new `lua-veto`
op-kind?

## Out of scope (unchanged from earlier draft)

- ACL v1 per-row rules from `acl.yaml` (separate ticket; the Lua
hook may make this entire direction moot).
- Read-side filtering / property redaction.
- Snapshot threading through `AuthorizeWrite`.

## References

- Phase-1 implementation: TKT-Y72A, PR #779
- Phase-2 implementation: TKT-LFT2, PR #786
- Design: `.ignored/action-affordances-design.md`
- Research: `.ignored/action-affordances-research.md` §8 (Stripe
attempt-and-recover)
- ACL v0: TKT-GN5LN
- Reframes driven by user feedback, 2026-05-21 session
