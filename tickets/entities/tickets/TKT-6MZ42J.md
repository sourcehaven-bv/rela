---
id: TKT-6MZ42J
type: ticket
title: Type picker and fuzzy ranking for the editor's @ mention completion menu
kind: enhancement
priority: medium
effort: l
status: done
---

## Problem

The `@` completion menu works "ok-ish". Two things are wrong with it.

**1. Ranking is a coarse tier sort over the ID alone**
(`frontend/src/components/forms/milkdown/useMentionMenu.ts:62`):

```ts
export function rankByIdMatch(items: Entity[], query: string): Entity[] {
  if (!query) return items
  const q = query.toUpperCase()
  const tier = (e: Entity): number => {
    const id = (e.id ?? '').toUpperCase()
    if (id === q) return 0
    if (id.startsWith(q)) return 1
    if (id.includes(q)) return 2
    return 3
  }
  // stable sort by tier; backend relevance survives as the tie-break
}
```

An entity's title is displayed in the menu but never scored, so every title-only
match lands in one undifferentiated tier 3 — behind any entity whose ID merely
*contains* the query — ordered only by whatever the backend returned. And
`includes` needs a contiguous substring, so there is no subsequence matching at
all.

**2. There is no way to say what kind of thing you are looking for.** The menu
searches every type at once. A query cannot express "a ticket called ranking",
so a common word returns a mixed list dominated by whichever type happens to be
numerous.

## What this ticket does

Two client-side changes to the menu.

**A type picker.** The menu gains a types section. Selecting a type scopes the
search to it through the **existing** `searchEntities(query, type)` parameter
(`api/entities.ts:207`), which `handleV1Search` already honours
(`api_v1.go:1761-1770`). Progressive disclosure, as agreed:

```
@                 types section: all types, scrollable
@re               TYPES     research / review-checklist / review-response
                  ─────────────────────────────────────
                  ENTITIES  (search results, as today)
@ranking-fix      ENTITIES only — over ~6 chars, types are hidden
@ticket <Enter>   [ticket] chip set; search now scoped to tickets
[ticket] rank     scoped search for "rank" within tickets
backspace to empty, then once more → chip cleared, search unscoped
```

**A fuzzy scorer** (`@leeoniya/ufuzzy`, 4.2 KB, MIT) replacing `rankByIdMatch`,
ranking entities across title and ID and ranking the type names in the types
section.

## Why a picker rather than inferring the type from the query

The original request was for `fancy-some-word-in-title` to find a `FancyReport`
entity. The design review established that inference cannot work:

- **The entity type is not free-text searchable on any backend.** Bleve stores a
`Type` field but never searches it — `boostedFields` covers only
`primary`/`properties`/`content`/`all`, and the `all` composite omits the type.
Measured: an entity of type `FancyReport` returns nothing for `fancyreport`.
Postgres does not index the type either. See RR-B0L1L0.
- **Making it searchable is expensive**: a full-table backfill under an advisory
lock, a five-site change including the `MatchText` conformance ground truth, and
a false-positive blowup where `@pro` matches every `project` *and* every
`policy` row. See RR-V9R8S9.
- **Prefixes are ambiguous anyway.** In this repo's own `tickets/` corpus (24
types, 4,235 entities) `re` resolves to 3 types covering **58% of all
entities**; `d`, `doc`, `test` and `review` are all ambiguous too.

A picker makes the user disambiguate in one keystroke instead of the system
guessing, turns the constraint into visible state, and needs **no backend change
at all**. `schemaStore.entityTypes` is already loaded on app mount.

## The query is sent unmodified — this is load-bearing

An earlier draft split the typed query on separators before sending it. That is
abandoned. Measured, it reproduces BUG-O09QUC's exact symptom: `TKT-6MZ42J`
finds the ticket, but split into `TKT 6MZ42J` it drops out of the top 5
entirely, buried under entities whose titles merely contain "tkt" — because
bleve builds its boosted ID queries from the *whole* query string
(`bleveindex.go:455-470`). It also returns nothing on the LinearSearch and
postgres backends, which match the whole query as one substring, and it would
let search-filter syntax be injected mid-query. See RR-88RUF8, RR-DLL40G,
RR-UE3YVJ.

## Prior art

The old backtick picker made the user choose a type first, then an entity.
`useMentionMenu.ts:4-8` records that `@` dropped that step deliberately because
"a backtick carries no information about what is wanted". This restores the
*capability* without the *mandatory* step: types are offered, never required.

## Out of scope

- Any backend change. No migration, no reindex, no conformance change.
- Allowing spaces in the mention query — a space still closes the menu.
- The backtick picker in `src/app-editor/`, which stays on EasyMDE.
- Free-text type matching in `/_search` generally (RR-B0L1L0) — a genuine gap,
worth its own backend ticket.
- Two pre-existing issues the review surfaced, both worth follow-ups: no
query-count budget test covers the free-text search branch, and `MIN_SEARCH_LEN
= 2` sits below pg_trgm's 3-character threshold, so the first search a user
triggers cannot use the trigram index.
