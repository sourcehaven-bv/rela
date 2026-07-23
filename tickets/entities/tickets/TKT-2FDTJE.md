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

## Why this wasn't fixed in TKT-MJ02AO

Scoping the payload is a **behavior change to what existing commands receive**.
A script that today sees every entity of a type would start seeing a subset,
which can silently break working automations. That needs its own ticket, a
migration note, and a deliberate decision about the failure mode — not a
drive-by change inside a PR about authorization.

## Scope

**In scope:**

- Scope `buildEntityInput` through `PermitsRead` — deny or 404 when the caller
cannot read the requested entity
- Scope `buildListInput` through `ReadQuery` so the script sees only readable
rows
- Decide and document the semantics: does a partially-readable list yield a
filtered payload (quiet) or an error (loud)? Filtered matches how the rest of
the read path behaves; loud is safer for scripts that assume completeness
- Decide whether `relationsForEntity` should filter relations whose far endpoint
is unreadable (it currently returns both directions unconditionally)
- **Then reconsider TKT-MJ02AO's view deferral** — if the traversal can be
read-gate scoped by the same mechanism, `context: view` may become grantable and
`permission:` can be honored for it
- Update `docs/acl-security.md` (the table currently documents the *unscoped*
behavior) and `docs/data-entry.md`
- Migration note: scripts may receive fewer entities than before

**Out of scope:**

- Changing who may *execute* a command (TKT-MJ02AO, done)
- The launcher routes (separate ticket)

## Acceptance criteria

- An `entity`-context command invoked with an `entity_id` the principal cannot
read does not receive that entity (404 or deny, per the recorded decision)
- A `list`-context command receives only rows the principal may read
- Under `NopACL` payloads are byte-identical to today (regression test)
- The chosen partial-readability semantics are documented and pinned by a test
- The view deferral is either lifted (with `permission:` honored) or
re-justified in writing against the new scoping
- `docs/acl-security.md`'s "What a command permission actually confers" table
is updated to describe the scoped behavior
