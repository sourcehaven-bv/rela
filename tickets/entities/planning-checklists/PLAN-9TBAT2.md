---
id: PLAN-9TBAT2
type: planning-checklist
title: 'Planning: Fuzzy multi-field ranking for the editor''s @ mention completion menu'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Revision 2.** The first plan was rewritten after a design review produced ten
> findings (four critical) and the operator replaced its type-matching mechanism
> with a type-picker UI. Findings RR-B0L1L0, RR-79QZA3 and RR-V9R8S9 are
> superseded by that decision (recorded in RR-77AMZO); the remaining seven are
> addressed in the approach below. The ticket is **frontend-only** again.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope — two changes to the `@` mention menu, both client-side:

1. **A type picker.** The menu gains a types section listing entity types.
Selecting one scopes the search to that type via the **existing**
`searchEntities(query, type)` parameter. The type is never matched as free text
and never inferred from the query.
2. **A fuzzy scorer** replacing `rankByIdMatch`, ranking the returned entities
across id and title, and ranking the type names in the types section.

OUT of scope:

- **Any backend change.** No pgstore migration, no bleve indexing change, no
`MatchText` conformance change. See RR-77AMZO for why this became possible.
- **Splitting the typed query before sending it.** The first plan's "half 1".
Dropped: it reproduces BUG-O09QUC's regression on bleve (RR-88RUF8), returns
nothing on LinearSearch and postgres (RR-DLL40G), and lets filter syntax be
injected mid-query (RR-UE3YVJ). The query goes to the server **as typed**,
exactly as today. Cross-field reach comes from the type picker instead.
- **Allowing spaces in the query.** Operator decision; `mentionQuery.ts` and the
"space closes" e2e test are untouched.
- The backtick picker in `src/app-editor/` (stays on EasyMDE).
- Free-text type matching in `/_search` generally — a real gap (RR-B0L1L0) worth
its own backend ticket.
- Two pre-existing issues surfaced by the review, both worth follow-up tickets:
no query-count budget test covers the free-text search branch, and
`MIN_SEARCH_LEN = 2` sits below pg_trgm's 3-character threshold so the first
search a user triggers cannot use the trigram index.

**The interaction, as decided with the operator:**

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

**Acceptance Criteria:**

1. **Type picker appears and scopes.** Typing `@` then `ticket` and pressing
Enter on the type row sets a `ticket` chip; the following search sends
`?type=ticket` and returns only tickets.
2. **Progressive disclosure.** Below `MIN_SEARCH_LEN` the types section shows
with no search fired. Between 2 and 6 characters, up to 3 best-matching types
show above the entity results. Above 6 characters the types section is hidden.
3. **Backspace clears the chip.** With a chip set and an empty query, one more
Backspace removes the chip and the next search is unscoped. Backspace with a
non-empty query edits text only.
4. **Enter is unambiguous.** Enter on a highlighted type row selects the type;
Enter on a highlighted entity row inserts the entity ref. Arrow keys traverse
both sections in one sequence.
5. **Cross-field reach.** The motivating case works via the picker: `@ticket` →
Enter → `ranking` finds the ranking ticket. The original
`@fancy-some-word-in-title` single-token form is **not** an AC — it is
unreachable without a backend change, and the picker replaces it.
6. **Fuzzy ranking of entities.** Title-only matches are ordered by match
quality rather than dumped in one tier, and an exact/prefix ID still ranks
first.
7. **Prefix typing never blanks the menu.** Every prefix of a query that will
eventually match must keep the target visible while typing (RR-NNFWGP).
8. **No regression for non-Latin queries.** A CJK or Cyrillic query still
returns the server's rows (RR-66JTAL).
9. **`@---` shows "No matches"**, never "Search failed" (RR-9B2QSS, RR-66JTAL).
10. **The ranker never invents rows.** Output is always a subset of the server
response; an empty subset over a non-empty response is its own tested state.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-DJEDGX

**Existing Solutions:** `@leeoniya/ufuzzy` 1.0.19, MIT, 4.2 KB min+gzip. Full
six-library comparison, fzf's scoring constants and measured benchmarks in
RES-DJEDGX. The choice survives the review: it is the only candidate that
accepts a hyphenated query as typed, and it returns match positions for
highlighting. `fuzzysort` is the fallback if per-field weighting is ever needed.

The review corrected one premise: there is **no separate editor chunk**.
`vite.config.ts` has no `manualChunks`, and `vite.editor.config.ts` builds the
EasyMDE app editor, which is out of scope. The 4.2 KB lands in the normal SPA
graph with no build-config change.

**Prior art in-tree.** The old backtick picker made the user choose a type
first, then an entity. `useMentionMenu.ts:4-8` records that `@` dropped that
step because "a backtick carries no information about what is wanted". This
design restores the *capability* without the *mandatory* step: types are
offered, never required.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*The query goes to the server unchanged.* This is the single most important
correction from revision 1. No splitting, no segment sanitisation, no ID-shape
detection — because nothing is rewritten, RR-88RUF8, RR-DLL40G and RR-UE3YVJ
cannot occur.

*1. Type source and filtering — no new API surface.*

`schemaStore.entityTypes` is already loaded on app mount (`stores/schema.ts:30`,
exposed as `entityTypeList` at `:215`). The types section renders from it.
Selecting a type sets menu state; `runSearch` then passes it as the existing
second argument:

```ts
searchEntities(query, selectedType, abort.signal)   // api/entities.ts:207
```

`handleV1Search` already applies `?type=` server-side (`api_v1.go:1761-1770`).
Nothing new is built.

*2. Menu state gains a type dimension.*

`MentionMenuState` grows `selectedType: string | null` and `typeItems:
string[]`. `MentionMenuController` gains `selectType(name)` and `clearType()`.
`highlightedIndex` must now address a **combined** list — types first, then
entities — so `moveHighlight` wraps across both sections and `current()` returns
a discriminated result:

```ts
type MentionChoice =
  | { kind: 'type'; name: string }
  | { kind: 'entity'; entity: Entity }
```

`MilkdownEditor.vue`'s `onKeydownCapture` (:439-481) dispatches on that kind:
`entity` → `insertRef` as today; `type` → `selectType` and keep the menu open.
This is AC 4 and is the part most likely to regress the existing e2e tests.

*3. Type visibility thresholds (AC 2).* Named constants beside the existing
`MIN_SEARCH_LEN` / `SEARCH_DEBOUNCE_MS` / `MAX_RESULTS`:

```ts
const MAX_TYPE_SUGGESTIONS = 3   // shown alongside entity results
const TYPE_SECTION_MAX_QUERY = 6 // above this, types are hidden
```

Below `MIN_SEARCH_LEN`, show all types and fire no search — this replaces
today's bare "Type to search entities" note with something useful.

*4. Backspace clears the chip (AC 3).* `parseMentionQuery` is untouched; the
clear is handled where the keystroke is already seen, in `onKeydownCapture`:
Backspace with an empty query and a chip set → `clearType()` and swallow the
event. Note the chip is menu state, not document text, so nothing is written to
the editor and the write-back guard is unaffected.

*5. Entity ranking with uFuzzy.* Haystack per candidate:

```ts
const haystack = `${entityDisplayTitle(e) || ''} ${e.id}`
```

The **type is deliberately absent** from the haystack now — it is a filter, not
a scoring dimension, so including it would reintroduce the false-positive
surface for no benefit. This also removes the need for the start-offset boost
entirely, so **RR-ZFYZCJ is designed out** rather than fixed.

Two guards the review proved necessary:

```ts
const terms = uf.split(query)
if (terms.length === 0) return items        // RR-66JTAL, RR-9B2QSS:
                                            // non-Latin or all-separator →
                                            // pass the server's order through
const idxs = uf.filter(haystacks, query)
if (idxs === null || idxs.length === 0) return []   // filter() returns null
```

*6. The quality floor — replaced (RR-NNFWGP).* `terms >= need` is wrong:
measured, it rejects every prefix match (`fanc` → terms=0) and drops `TKT-ABCD`
for query `TKT-AB`. Since a picked type already bounds the candidate set, an
aggressive floor is no longer load-bearing. **Rank, do not filter**, except for
what uFuzzy's own `filter` already excludes. If a floor proves necessary in
manual testing, it must tolerate a prefix on the final term and be pinned by a
matrix over *every* prefix of a target query — not by complete-word fixtures,
which is exactly how the original mistake survived.

*Alternatives rejected:* inferring the type from a query segment (ambiguous — in
the real tickets corpus `re` resolves to 3 types covering 58% of entities; see
RR-77AMZO); adding the type to the searchable text (backfill, five-site
conformance change, false-positive blowup — RR-V9R8S9); broadening rather than
narrowing on a type match (operator chose narrow: it keeps the result set small
and cannot evict the target via the 1000-row relevance cap).

**Files to modify:**

- `frontend/package.json` — add `@leeoniya/ufuzzy`.
- `frontend/src/components/forms/milkdown/useMentionMenu.ts` — type state,
`selectType`/`clearType`, combined highlight, pass `type` to `searchEntities`.
- New `frontend/src/components/forms/milkdown/mentionRanking.ts` — pure scorer
for entities and for type names, with the two guards above.
- `frontend/src/components/forms/milkdown/MentionMenu.vue` — types section,
chip, section headers, combined highlight rendering.
- `frontend/src/components/forms/milkdown/MilkdownEditor.vue` — `onKeydownCapture`
dispatch on choice kind; Backspace-clears-chip.
- Tests: `useMentionMenu.test.ts`, new `mentionRanking.test.ts`,
`e2e/tests/markdown-editor-mention-autocomplete.spec.ts`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *The typed query* — untrusted, bounded by `MAX_QUERY_LENGTH = 64`
(`mentionQuery.ts:17`), terminated on whitespace and backticks, and now sent
**unmodified**. Because nothing is rewritten, RR-UE3YVJ's filter-syntax
injection cannot arise: the single-token invariant that makes `@status:open`
harmless today is preserved.
- *The selected type* — chosen from `schemaStore.entityTypes`, so it is an
**allowlist by construction**; a free-typed string never becomes a `?type=`
value.
- *Candidate fields* — `type`, `_title`, `id` from `/_search`, already
ACL-filtered server-side.

**Security-Sensitive Operations:**

**This must not become an ACL bypass.**

- **Recall stays server-side and gated.** `/_search` runs through the ACL-scoped
`search.VisibleSearcher`. The client filters and reorders that set and must
never widen it — no local cache of candidates, no client-side title lookup.
- **Entity-reference titles are ACL output, never derived.** Verified satisfied:
the haystack uses the `_title` already on the response, which
`stripHiddenProperties` (`affordances.go:990-994`) has already rewritten to the
ID when the display property is hidden. `entityDisplayTitle` is also exactly
what `MentionMenu.vue:29` renders, so haystack and display cannot diverge.
- **Type names are not confidential** (root CLAUDE.md: "The configuration is not
a secret" — list and view names, entity and property names are explicitly named
as non-secret). Showing the full type list to every principal is therefore
correct and needs no per-principal filtering. Counts, not names, are the gated
thing. This mirrors the settled decision in `docs/acl-security.md` § "Sidebar
menu structure is principal-independent" — do not reintroduce per-principal menu
filtering here as a security measure.

No file access, auth or crypto. The `filter() === null` guard (RR-66JTAL)
prevents a `TypeError` surfacing through the existing catch as a misleading
"Search failed".

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Level |
|---|---|---|
| 1 picker scopes | assert `searchEntities` is called with the type as 2nd arg after `selectType` | unit |
| 1 (integration) | e2e: `@` → pick `ticket` → type → only tickets listed → Enter inserts | e2e |
| 2 thresholds | table-driven over query lengths 0,1,2,5,6,7: assert which sections render | unit |
| 3 backspace | chip set + empty query + Backspace → chip cleared, next search unscoped; non-empty query → text edit only | unit |
| 4 Enter/arrows | highlight traverses types then entities; Enter dispatches on kind | unit + e2e |
| 5 cross-field | e2e: the motivating flow end to end | e2e |
| 6 fuzzy ranking | title-quality ordering; exact ID first; ID prefix beats loose title | unit |
| 7 **prefix matrix** | for target "FancyReport ranking": every prefix `f`,`fa`,`fan`,`fanc`,`fancy`,`fancy-`,… keeps the target visible. **This is the test that catches RR-NNFWGP** | unit |
| 7 ID prefix | `TKT-AB` must still return **and rank** `TKT-ABCD` — the existing `useMentionMenu.test.ts:24-29` case | unit |
| 8 non-Latin | a CJK title and a Cyrillic title survive; assert the server order is passed through unchanged | unit |
| 9 `@---` | "No matches", not "Search failed"; assert no throw | unit |
| 10 subset | output ⊆ input always; empty-subset-over-non-empty-response is its own case | unit + `fast-check` property |

**Edge Cases:**

- `uf.filter()` returns `null`, not `[]` — every call site null-checks.
- `uf.split()` returns `[]` for all-separator and non-Latin input.
- Missing `_title` (redacted or untitled): haystack must not contain
`undefined`; the entity stays rankable by ID.
- A type is selected, then the user backspaces the whole query: chip persists
until one further Backspace.
- Duplicate IDs across faces are possible and `MentionMenu.vue:46` keys on
`item.id`; any haystack de-duplication would break the index mapping.
- A schema with zero entity types, and one with a single type.
- Selected type no longer exists after a schema reload.
- 64-character query at the `MAX_QUERY_LENGTH` bound.
- Late/superseded responses and AbortError — existing tests must keep passing
since `runSearch` is edited.

**Negative Tests:** a query matching nothing → "No matches", never a stale list;
`searchEntities` rejecting → `'Search failed'` with items cleared; the ranker
never introduces a candidate absent from the response.

Do **not** add an O(n²) assertion — measured at ~1.0 ms for 1000 fully-matching
candidates against a 150 ms debounce, it guards a non-problem.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **The combined highlight index is the likeliest bug source.** Two sections,
one index, wrap-around, and an Enter that means different things per section.
*Mitigation:* model the choice as a discriminated union so the compiler forces
every call site to handle both kinds; unit-test traversal across the boundary
explicitly.
- **Eight existing e2e tests** cover the current menu, including "space closes"
and "Escape without insert". *Mitigation:* run the whole spec, not just new
tests.
- **A quality floor added later by feel** could silently reintroduce RR-NNFWGP.
*Mitigation:* the prefix matrix (AC 7) is a standing guard — any floor must pass
it.
- **Discoverability.** A types section that only appears under 6 characters may
go unnoticed. *Mitigation:* showing all types on a bare `@` (replacing a dead
"Type to search entities" note) is the discovery path.
- **Performance is not a risk here:** 2 database round trips per search on
postgres, ACL membership walk memoized once per request, ~1.0 ms client-side
ranking. A picked type makes the query *cheaper* (indexed equality).

**Effort:** **m → l.** More UI surface than revision 1 (a second section, a
chip, combined keyboard handling), but no backend work and no migration.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — the `@` menu: type picker, how to scope and clear,
that a space still closes the menu.
- [x] `frontend/CLAUDE.md` — mention ranking is client-side over an ACL-filtered
response; no title may be looked up locally to enrich it; the query is sent to
`/_search` **unmodified** and why (BUG-O09QUC's ID ranking depends on it).
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Ten findings. Every one was verified by executing code — the installed uFuzzy
package, a throwaway bleve index, the real `searchparser`, and the live
`tickets/` corpus — except RR-79QZA3, which is code-read (no test database).

| ID | Severity | Status | How it is addressed |
|---|---|---|---|
| RR-88RUF8 | critical | addressed | Query is no longer split; sent as typed |
| RR-DLL40G | critical | addressed | Same — no rewrite, so no backend divergence |
| RR-NNFWGP | critical | addressed | `terms` floor dropped; rank-don't-filter; prefix matrix (AC 7) |
| RR-B0L1L0 | critical | wont-fix | Superseded by RR-77AMZO — type is filtered, never text-matched |
| RR-ZFYZCJ | significant | addressed | Designed out — type left out of the haystack entirely |
| RR-9B2QSS | significant | addressed | Empty-split guard returns server order (AC 9) |
| RR-66JTAL | significant | addressed | Same guard + `filter() === null` check (AC 8, 9) |
| RR-UE3YVJ | significant | addressed | Cannot arise — single-token invariant preserved |
| RR-79QZA3 | significant | wont-fix | Superseded by RR-77AMZO |
| RR-V9R8S9 | significant | wont-fix | Superseded; two follow-up recommendations carried to Out of Scope |
| RR-77AMZO | significant | addressed | The operator's type-picker design, now the plan |

**Operator decisions taken, all confirmed:**

1. Space keeps closing the menu — `mentionQuery.ts` untouched.
2. A type match **narrows** the search rather than broadening it.
3. Type picker with progressive disclosure: all types on bare `@`, up to 3 best
between 2 and ~6 characters, hidden above that.
4. Backspace at the start of the query clears the type chip.

**Ready for implementation.**
