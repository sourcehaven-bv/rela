---
id: RR-77AMZO
type: review-response
title: 'Operator design: a visible type picker replaces inferred type matching entirely'
finding: 'The operator proposed a UI-led alternative that dissolves the problem RR-B0L1L0, RR-79QZA3 and RR-V9R8S9 were all circling. Rather than INFERRING an entity type from a query segment (which is ambiguous — in the tickets corpus ''re'' resolves to 3 types totalling 2467 of 4235 entities, and ''d'', ''doc'', ''test'', ''review'' are all ambiguous too), the menu SHOWS candidate types as selectable rows. The user disambiguates by picking rather than the system guessing, and the type constraint becomes explicit state rather than a hidden heuristic. Progressive disclosure by query length: on the first word show a type list; around 3 characters show a few best type options ALONGSIDE entity results; beyond that hide the type options or require increasing match quality. Combined with the operator''s other decision — a type match NARROWS rather than broadens — this eliminates the 1000-row eviction risk, the false-positive blowup, and the ranking perturbation in one move.'
severity: significant
resolution: 'Adopted as the design direction. Crucially this removes the need for ANY backend change: schemaStore.entityTypes is already loaded on app mount (frontend/src/stores/schema.ts:30, exposed as entityTypeList at :215), so the type list needs no API call. Type filtering then reuses the EXISTING searchEntities(query, type) parameter (frontend/src/api/entities.ts:207-217), which already forwards ?type= to handleV1Search (internal/dataentry/api_v1.go:1761-1770). So RR-B0L1L0''s finding that no backend indexes the type free-text becomes MOOT rather than blocking — the type never needs to be free-text searchable, because it is never searched as text. RR-79QZA3 and RR-V9R8S9 are superseded for the same reason: no pgstore migration, no backfill, no MatchText conformance change, no EXPLAIN test, no ranking shift.'
status: addressed
---

## Why the operator's design is better than either option offered

I offered a choice between exact-match and unambiguous-prefix type inference.
Both are guessing games whose behaviour depends on the schema and shifts
whenever a type is added. Measured against the real `tickets/` corpus (24 types,
4,235 entities):

| typed | prefix inference | rows pulled in |
|---|---|---|
| `re` | research, review-checklist, review-response | 2,467 (58% of corpus) |
| `review` | review-checklist, review-response | 2,437 |
| `d` | decision, doc-task, docs-checklist | 204 |
| `bug` | bug, bug-analysis-checklist | 152 |

Inference is ambiguous in the common case. A picker is not: the user resolves it
in one keystroke, and the resulting constraint is visible rather than guessed.

## What this removes from the ticket

Three findings collapse:

- **RR-B0L1L0** (type not free-text searchable on any backend) — moot. The type
is never matched as text.
- **RR-79QZA3** (pg `search_text` omits the type) — superseded. No change
needed.
- **RR-V9R8S9** (backfill, 5-site conformance change, false-positive blowup) —
superseded. None of it applies.

What remains is a **frontend-only** ticket again, which is where it started,
plus a genuinely better interaction.

## Existing machinery this reuses

- `schemaStore.entityTypes` — loaded on app mount, `entityTypeList` at
`stores/schema.ts:215`. No API call.
- `searchEntities(query, type)` — the `type` parameter already exists
(`api/entities.ts:207-217`).
- `handleV1Search` already applies `?type=` server-side
(`api_v1.go:1761-1770`).
- `MentionMenu.vue` is purely presentational with a `pick`/`hover` contract, so
a type section is an additive change.

There is also precedent: the **old backtick picker** made the user choose a type
prefix first, then an entity. `useMentionMenu.ts:4-8` records that `@` dropped
that step deliberately because "a backtick carries no information about what is
wanted". This design restores the *capability* without restoring the *mandatory*
step — types are offered, never required.

## Open design questions for the plan

1. **Thresholds.** The operator sketched: first word → type list; ~3 chars →
a few best types plus entity results; beyond → hide types or raise the quality
bar. The exact numbers need to be chosen and pinned by tests.
2. **How many type rows** to show (operator suggested 3 best).
3. **Ranking within the type list** — the fuzzy scorer this ticket introduces
can rank type names too, which is a natural reuse.
4. **How a picked type is displayed and cleared.** A chip in the menu? Does
Backspace at the start of the query clear it? This is the part most likely to
feel wrong if unspecified.
5. **Interaction with Enter.** Enter currently inserts the highlighted entity.
If a type row is highlighted, Enter must select the type instead — so the
highlight index now spans two sections, and `MilkdownEditor.vue`'s keydown
handling (`onKeydownCapture`, :439-481) needs to account for that.
6. Whether a picked type also **narrows** subsequent keystrokes (the operator
confirmed narrow semantics) and whether that survives a backspace.
