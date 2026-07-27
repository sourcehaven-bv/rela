---
id: TKT-2FDTJE
type: ticket
title: Read-gate scope command payloads (entity/list), then reconsider the view-context deferral
kind: enhancement
priority: high
effort: m
status: backlog
---

## Description

Command stdin payloads are assembled straight from the store with **no
per-entity read-gate scoping**, in every context. TKT-MJ02AO gated *who may run*
a command; it did not scope *what the script receives*. So a `command:*` grant
is closer to "may read every entity of this shape" than "may run this script".

Raised as RR-37AYC0 during TKT-MJ02AO's code review. Documented accurately there
(`docs/acl-security.md` → "What a command permission actually confers"); this
ticket closes it in code.

## Evidence

`internal/dataentry/commands.go` — entity context, no `PermitsRead`:

```go
case "entity":
	entityID := r.URL.Query().Get("entity_id")
	svc := h.services()
	entityDomain, err := svc.Store.GetEntity(r.Context(), entityID)
	if err != nil { ... 404 ... }
	input = h.buildEntityInput(r.Context(), entityDomain)
```

`relationsForEntity` then loads **every** incident relation
(`store.DirectionBoth`). For lists, `listFromStoreByTypes` is a raw
`ListEntities` drain with no `ReadQuery` scoping.

Both `entity_id` and `list_id` come from the **request**, not from the page the
user was on — `available_on` is display scoping only.

| context | payload | scoped by |
|---|---|---|
| `entity` | entity at caller-supplied id + all incident relations | nothing |
| `list` | all entities in caller-supplied list, post-filter | nothing |
| `global` | project paths only | n/a |
| `view` | entry + full traversal closure | not grantable today |

Compare the read path, which *does* gate: `history_handler.go` uses
`gateReadOrNotFound` / `PermitsRead`.

## The seam to use (UPDATED 2026-07-25)

When this ticket was filed, the fix looked like hand-rolling `PermitsRead` /
`ReadQuery` calls at each payload builder. **Since then the read-side ACL has
been decomposed into `internal/visibility` decorators (DEC-ZBI39P, landed via
TKT-ZF2DTV / #1197), and `internal/dataentry` already exposes the exact seams
this ticket needs** — do NOT hand-roll gating:

- **entity context** → `a.visibleReader.getVisible(ctx, entityType, id)`
(see `api_v1.go:745`, `feed_handler.go:158`). Returns `(entity, found, err)`; a
hidden entity comes back `found=false`, i.e. an indistinguishable 404, which is
exactly the semantic this ticket wants.
- **list context** → `a.scopedSortedEntities(ctx, typeName, query)`
(`api_v1.go:282`) — the shared list pipeline that already applies ACL read
scope. The list handler at `api_v1.go:581` is the reference caller.
- **relations** → `visibleRelationIDs` (the neighbor-visibility gate the
transform/export path uses per CLAUDE.md) is the tool for filtering
`relationsForEntity` so hidden neighbors don't leak.

This **simplifies** the ticket: the payload builders in `commands.go` should
route through the same seam the GET handlers use, rather than the raw
`h.services().Store` / `listFromStoreByTypes`. It also means the fix is
consistent-by-construction with how every other read is gated, instead of a
parallel gating path that could drift.

Per CLAUDE.md's new rule ("Never redact a read that feeds a write"): command
payloads are read-**out** (the script consumes them), not write-prep, so the
visibility-wrapped read is correct here — there is no read-modify-write to
clobber.

## Why this wasn't fixed in TKT-MJ02AO

Scoping the payload is a **behavior change to what existing commands receive**.
A script that today sees every entity of a type would start seeing a subset,
which can silently break working automations. That needs its own ticket, a
migration note, and a deliberate decision about the failure mode — not a
drive-by change inside a PR about authorization.

## Scope

**In scope:**

- Route `buildEntityInput` through `a.visibleReader.getVisible` — a hidden
entity yields the same 404 as a nonexistent one
- Route `buildListInput` through `a.scopedSortedEntities` so the script sees
only readable rows
- Filter `relationsForEntity` via `visibleRelationIDs` so hidden neighbors
don't leak into the payload
- Decide and document the semantics: a partially-readable list yields a
filtered payload (quiet) — matches how the rest of the read path behaves and how
`scopedSortedEntities` already works; a loud error would fight the seam
- **Then reconsider TKT-MJ02AO's view deferral** — `executeView`'s traversal
would need the same visibility wrapping; if that composes cleanly, `context:
view` may become grantable and `permission:` honored for it
- Update `docs/acl-security.md` (the table currently documents the *unscoped*
behavior) and `docs/data-entry.md`
- Migration note: scripts may receive fewer entities than before

**Out of scope:**

- Changing who may *execute* a command (TKT-MJ02AO, done)
- The launcher routes (TKT-JRY8V5)
- Re-architecting the visibility seam — this ticket *consumes* it

## Acceptance criteria

- An `entity`-context command invoked with an `entity_id` the principal cannot
read receives the same 404 as a nonexistent id (via `getVisible`)
- A `list`-context command receives only rows the principal may read (via
`scopedSortedEntities`)
- Hidden neighbor relations do not appear in an entity command's payload
- Under `NopACL` payloads are byte-identical to today (regression test — the
visibility seam is a pass-through under NopACL)
- The filtered-not-errored semantics are documented and pinned by a test
- The view deferral is either lifted (with `permission:` honored) or
re-justified in writing against the visibility-wrapped traversal
- `docs/acl-security.md`'s "What a command permission actually confers" table
is updated to describe the scoped behavior
