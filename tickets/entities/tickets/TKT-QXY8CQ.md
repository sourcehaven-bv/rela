---
id: TKT-QXY8CQ
type: ticket
title: Direction inference for list columns / filter controls / kanban cards / CalDAV collections
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Follow-up to TKT-860BNJ, which removed the implicit `outgoing` default for FORM
relations only. The same magic remained on every other surface carrying a
`direction:`. This extends the rule to all of them, so the behaviour is uniform:

| Surface | Anchored to |
| --- | --- |
| form relations (incl. wizard steps) | form's `entity_type` (done in TKT-860BNJ) |
| list `columns` | list's `entity_type` |
| list `filter_controls` | list's `entity_type` |
| kanban `card.fields` | kanban's `entity_type` |
| kanban `filter_controls` | kanban's `entity_type` |
| `caldav.dynamic.<name>` | collection's `entity_type` (the MEMBER) |

Rule unchanged from TKT-860BNJ: entity type on exactly one side infers that
direction; on both sides is a hard error naming the binding; on neither side is
the existing wrong-side error.

## CalDAV is included deliberately

Its direction reads differently — it is the member→driver edge between two
declared types, not "which side is this view's entity on". Inference is still
well-posed (both types are declared, and `entity_type` is the member), and the
severity is higher than the display surfaces: `dynamicMembers` queries the
mirror side, so a wrong direction selects the wrong member set and a
client-created entry lands in the entity type but in NO collection, vanishing on
the next sync. That is form-relation severity, not display severity.

Both directions were already supported on CalDAV; only the *absent* case
defaulted silently.

## Approach

- `dataentryconfig.CheckAmbiguousDirection` — one shared guard, called from all
  six validation sites. `AmbiguousDirectionError` builds the message once
  instead of six copies (the new commentlint `duplication` rule is the standing
  argument against copy-paste prose, and the same logic applies to error text).
- `resolveConfigDirection` in `internal/dataentry` — one materializer, used by
  forms, lists, kanbans, view sections and the CalDAV backend. Every SPA
  consumer tests the literal `direction === 'incoming'` (RelationCards,
  RelationPicker, FilterBar, EntityList, KanbanView), so an inferred direction
  MUST be materialized server-side.
- The migration is renamed `form-relation-direction` → `relation-direction` and
  extended to the new surfaces. Its Detect/Apply were restructured to share one
  `bindings()` traversal, so they cannot disagree — the prior review flagged the
  apply-bool dual-use as provable-but-not-obvious.

## Scope note

Zero in-repo configs bind relations on these surfaces (all 34 relation uses in
`tickets/data-entry.yaml` are form relations), so the migration is a no-op here
and the verification burden sits entirely on unit tests plus a scratch project
exercising all five surfaces.

## Acceptance criteria

1. Unambiguous bindings on all surfaces validate without an explicit direction.
2. Self-referencing bindings on all surfaces are a validation error naming them.
3. An explicit direction is preserved verbatim everywhere.
4. Lists/kanbans are served to the SPA with directions materialized, without
   mutating the operator's in-memory config.
5. `rela migrate` fills unambiguous bindings on every surface and skips
   self-referencing ones.
6. CalDAV `dynamicMembers` resolves an absent direction from the member type.
