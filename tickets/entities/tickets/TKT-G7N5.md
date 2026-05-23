---
id: TKT-G7N5
type: ticket
title: _fields / _relations wire shape + SPA renderer (stub verdict source)
kind: enhancement
priority: medium
effort: l
status: done
---

## Goal

Land the **wire shape and SPA renderer** for field-level and relation-meta-level
affordances, driven by a **dev-only stub verdict source**. The eventual ACL
backing (role grants, predicates) is out of scope; this ticket exists to lock in
the wire contract and prove the SPA can render against it end-to-end.

Server emits `_fields` and `_relations` on per-entity GET, alongside the
existing `_actions`:

```json
{
  "id": "TKT-ABC",
  "type": "ticket",
  "properties": {...},
  "_actions": {"update": true, "delete": false, "rename": true},
  "_fields": {
    "description": {"writable": false},
    "status": {"writable": true, "options": {"ready": true, "in-progress": true, "done": false}}
  },
  "_relations": {
    "implements": {
      "creatable": true,
      "removable": true,
      "fields": {
        "note": {"writable": true},
        "weight": {"writable": false}
      }
    }
  }
}
```

## Wire semantics

- **Hidden** fields are **OMITTED** from `properties` entirely (and from `_fields`).
- **Read-only** fields appear in `properties` with `_fields[name].writable=false`.
- **Editable** fields appear in `properties` with `_fields[name].writable=true`.
- No masking; no `null` sentinel. `editable=false` is the same as read-only.
- **Relation meta fields** get per-relation-type affordances:
`_relations[type].fields[name].writable` mirrors `_fields[name].writable`.
Uniform across every link of that type. Per-link affordances are predicate
territory.

## Verdict source — stub

`internal/dataentry` exposes a `FieldVerdictResolver` interface. v1 ships one
implementation: a hardcoded fixture controlled by an env var.

- `RELA_AFFORDANCE_PROFILE=none` (default) — everything writable / visible /
creatable / removable. Equivalent to today's behavior.
- `RELA_AFFORDANCE_PROFILE=triager-demo` — applies a fixture profile against
the `ticket` type that exercises every affordance code path:
  - `description` read-only
  - `internal_notes` hidden (omitted)
  - `status.options.done` = false (filtered)
  - `implements` not creatable
  - `has-review-response` not removable
  - one relation meta-field non-writable

This profile is the manual-verification surface for the SPA work. No `acl.yaml`
schema changes.

## SPA consumption

`DynamicForm` consumes the new shape:
- Disabled inputs for read-only fields
- Filtered options on enum `<select>` (option absent when `_fields[name].options.<v>=false`)
- Hidden fields don't render at all (they're not in `properties`)
- Hidden `+ Add` button on relation panels when `_relations[type].creatable=false`
- Hidden per-link `x` button when `_relations[type].removable=false`
- `RelationCards` inline meta-field inputs disabled when
`_relations[type].fields[name].writable=false`

## In scope

- Wire-shape extension: `_fields` (name → `{writable, options?}`),
`_relations` (type → `{creatable, removable, fields?}`)
- `FieldVerdictResolver` interface + hardcoded `triager-demo` fixture
- `data-entry` handler emits new shape on per-entity GET
- SPA renderer (`DynamicForm`, `RelationCards`) consumes new shape
- Wire-vs-policy parity: stub also gates the corresponding write paths so a
stale client POSTing a hidden field gets a 403 with a rule identifier
- Per-entity granularity for v1

## Out of scope (follow-ups)

- **Real verdict source** — `acl.yaml` schema for field/option/relation-meta
grants. Predicate-engine ticket will own this and replace the stub directly. No
transitional `*_for` shorthand syntax.
- Type-default + per-entity-override compression of `_fields` for list views
- List-query field-level read enforcement (only per-entity GET emits the new fields)
- Cache invalidation beyond what entity ETags already cover
- Masked-value rendering ("****")
- Per-link affordances (different verdicts for different links of the same type)

## Why one combined ticket

Backend wire-shape changes can't be properly verified without a consumer
rendering them. Phase-1 affordances were split (TKT-Y72A backend / TKT-LFT2 SPA)
and the two PRs gate each other in review. This time one PR covers wire + server
+ SPA so the end-to-end works against the stub before the predicate ticket
lands.

## Dependencies

- ACL v0 (TKT-GN5LN, TKT-1XK1L, TKT-K0C83) — done. This ticket doesn't change
the ACL surface; it adds an *adjacent* affordance source for fields and
relation-meta.
- Affordances phase 1 (`_actions`, PR #779) — in review; this ticket builds on
the wire shape introduced there. Will rebase on develop once #779 merges.
