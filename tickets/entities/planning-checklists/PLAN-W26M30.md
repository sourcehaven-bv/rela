---
id: PLAN-W26M30
type: planning-checklist
title: 'Planning: Relation filter_controls render as target selector (select → typeahead), not free text'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: Relation `filter_controls` in the data-entry list `FilterBar` render as a
target selector instead of free text — plain `<select>` for a small option set
(≤ 10), typeahead combobox above. Value SENT to the backend is the target's
**bare display title** (`_title` via `entityDisplayTitle`, NOT `Title (ID)`),
matching the existing backend title-match. Options fetched client-side from the
relation's source-type list (`from[*]` for `incoming`, `to[*]` for `outgoing`),
reusing the same `entitiesStore.fetchList` the RelationPicker already uses.
Frontend unit + e2e tests.

OUT: Multi-select relation filter; a server-side `_filter_options` endpoint; "my
taken"/current-user pre-population; typeahead paging beyond the existing
`per_page: 100` per-type ceiling; widening `PROPERTY_NAME_RE` to allow
hyphenated relation names in deep links (see RR-3TJVQJ).

**Acceptance Criteria:**

1. A `filter_controls` entry with `relation:` renders as a `<select>` when the
resolved option count is ≤ 10, populated with the source-type entities' display
titles, plus an "All" (empty value = no filter) option. *Test:* mount FilterBar
with a relation control + stubbed schema/store that returns ≤ 10 candidates →
assert a `<select>` with N+1 `<option>`s whose values are titles.
2. Above 10 options, the control renders as a typeahead combobox (text input
   + filtered dropdown) reusing the RelationPicker search UX.
*Test:* same, with > 10 candidates → assert `role="combobox"` input, typing
narrows the visible options.
3. Selecting/committing an option filters the list; the emitted `filter` state
value is the **bare display title** (`_title`), NOT the entity id and NOT `Title
(ID)`. The wire param is `filter[<relation>]=<title>`. *Test:* select an option
→ assert `emit('filter', ...)` carries `{ [relation]: { value: '<bareTitle>' }
}`. e2e: pick a persoon → list narrows.
4. Empty selection (All / cleared input) removes the filter.
*Test:* select "All" → emitted state omits the key.
5. Property filters render exactly as today.
*Test:* existing property-filter render assertions unchanged/green.
6. `direction: outgoing` (or omitted) pulls options from the relation's `to[*]`
types; `incoming` from `from[*]`. *Test:* two mounts differing only in
`direction` → assert the fetched target type set matches `to` vs `from`.
7. Deep-link round-trip: `filter[<relation>]=<title>` in the URL on load shows
the matching option as selected. *Test:* seed props.filters with a title →
assert the select/typeahead reflects it as the current selection.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small, well-scoped UI change; approach is clear from existing components)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (effort `s`; reuse of an existing in-repo pattern)

**Existing Solutions:**

- No new library needed. Typeahead-over-entities already exists in-repo in three
places: `frontend/src/components/forms/RelationPicker.vue` (closest — resolves
candidates from a relation's from/to types), `EntityPickerModal.vue`, and
`ui/CommandPaletteModal.vue`.
- `RelationPicker.vue:114-145` — `filteredCandidates` (in-memory filter by `id`/
`_title`) + `loadCandidates()` (per-source-type `entitiesStore.fetchList(type, {
per_page: 100 })`, walking `relationType.from`/`.to`). Exact candidate-
resolution + search behavior needed, minus edit/save/affordance/inline-create.
- `FilterBar.vue:60-71` `resolveWidgetType` + template `<select>` at 226-240
already handle enum property filters — the relation branch slots in alongside.
- Backend match-by-title confirmed at `internal/dataentry/api_v1.go:476`
(`DisplayTitle(...) == want`). `_title` present on every v1 list row
(`entityserializer.toV1` always sets `Title: meta.DisplayTitle(...)`), so
`fetchList` rows carry it. `entity.ts:4` types it.
- `entityDisplayTitle` (bare, `entityDisplay.ts:25`) vs `entityDisplayTitleWithId`
("Title (ID)", :32) — the committed value MUST use the bare form.
- Wire format (VERIFIED): `filter[<key>]=<value>`. Backend `parseRelationFilterKey`
(api_v1.go:418) parses the bracket form only; SPA `filters.ts`
(`filterStateToApiParams:188`, `buildQueryWithFilters:161`) emits bracket form;
`relation_filter_test.go:139` proves `filter[verantwoordelijk_voor]={title}`.
- `RelationType` shape has `from: string[]` / `to: string[]`
(`schema.ts:34-38`); `schemaStore.getRelationType(name)` is the accessor.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Decisions locked (user-approved):**

- **Threshold = 10.** `≤ 10` candidates → plain `<select>`; `> 10` → typeahead.
- **New component lives in `frontend/src/components/common/`**, not `lists/`. The
long-term intent is that this IS the shared entity-target selector (future
RelationPicker consumes it too), so `common/` is the correct home.

**Design-review corrections folded in (RR-3MDVZD, RR-X4QWBF, RR-NH8B6D,
RR-A51QQ2, RR-3TJVQJ — all addressed):**

- **Wire format is `filter[<relation>]=<title>`** (RR-3MDVZD). The earlier
`filter_<rel>` wording was WRONG (reasoned from the dead-legacy
`QueryParamKey()`). The mechanism is already correct: FilterBar keys
`localFilters`/`buildState` by the bare relation name (`control.property ||
control.relation`, :98/:144), so a relation filter already emits the right
bracket form today. The selector just sets `localFilters[relation]=<title>` and
calls `handleFilterChange()`. NO wire-format change, NO backend change.
- **Committed value = `entityDisplayTitle(entity)` (bare `_title`)**, never
`entityDisplayTitleWithId` (RR-X4QWBF). Dropdown MAY display `Title (ID)` for
disambiguation; the VALUE written to `localFilters` is the bare title.
- **`searchQuery` is component-local, separate from committed value** (RR-NH8B6D).
Committed value changes only on option-select. External `props.filters` updates
the selected value, never the search box. Click-away discards the search string.
Emit via `handleFilterChange` (immediate), not `handleTextInput`.
- **Deep-link title→option resolution** (RR-A51QQ2): `<select>` option values are
bare titles so `v-model=localFilters[relation]` binds directly; typeahead
resolves the committed title against `entityDisplayTitle(candidate)` to mark the
selection, first-match on duplicate titles.
- **Hyphenated relation names** (RR-3TJVQJ): out of scope; `PROPERTY_NAME_RE`
rejects them on deep-link parse. atlas relations are underscore-safe. Implement
step must verify the in-scope atlas relations are identifier-safe.

**Technical Approach:**

1. **Extract a reusable candidate-search child** →
`frontend/src/components/common/EntityTargetSelect.vue`: presentational, takes
resolved candidates (`{ id, _title, type }[]`) + `mode` (`select` | `typeahead`)
+ the currently-committed title, emits a single chosen bare title. Owns
text-input + filtered-dropdown + click-outside lifted from RelationPicker
(`filteredCandidates`, `showDropdown`, `handleClickOutside`), with `searchQuery`
strictly local. Takes NONE of: `incoming-changed`, verdicts,
`InlineCreateModal`, `update:types`, `isIncoming`, multi-select. Display label =
`entityDisplayTitleWithId` (disambiguation); emitted value =
`entityDisplayTitle` (bare). RelationPicker keeps its own dropdown this ticket.
2. **In FilterBar**, extend `ResolvedFilter` with a relation branch: resolve
source types (`getRelationType(rel).from` for incoming / `.to` for outgoing),
fetch candidates once on mount (`fetchList` per type, `per_page: 100`,
`isCancelledFetch`-suppressed), cache in setup state keyed by control key.
Count-gate: `candidates.length <= 10` → `select`; else → typeahead child. The
relation control is NOT added to `textWidgetKeys`.
3. **Value plumbing**: chosen bare title → `localFilters[relation]` →
`buildState()` → `emit('filter')`, unchanged path. Wire param
`filter[<relation>]=<title>` (already what the existing plumbing produces).

**Alternatives considered:**

- *Plain `<select>` always* — rejected: 100-option native select is poor UX.
- *New `_filter_options` backend endpoint* — deferred (out of scope): match-by-
title means the client already has the exact strings.
- *Refactor RelationPicker to consume the child in this ticket* — rejected as
scope creep; the TKT-GFQK form save path is delicate. Extraction is additive.
- *Component in `lists/`* — rejected: reusable selector, `common/` is its home.

**Files to modify:**

- `frontend/src/components/lists/FilterBar.vue` — relation branch in
`resolveFilter` + candidate fetch/cache + widget gate + template.
- NEW `frontend/src/components/common/EntityTargetSelect.vue`.
- NEW `frontend/src/components/lists/FilterBar.test.ts` +
`frontend/src/components/common/EntityTargetSelect.test.ts`.
- `e2e/tests/…` — relation-filter narrows list.
- Possibly `frontend/src/types` — small shared candidate type if not reusing
`Entity`.

Backend: no change.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Config (`filter_controls[].relation` / `.direction`) — trusted project config;
resolved via `getRelationType`, unknown relation → render nothing / fall back.
- Candidate list — server data via the ACL-gated `GET /api/v1/{plural}`; the
filter offers only what the read gate returns to this user. The list filter is
re-applied server-side through the ACL-gated relation-filter pass
(`matchRelationFilter` gates neighbors, RR-HK1XNO), so the widget cannot widen
visibility. No new trust boundary.
- Chosen value — a display-title string as a query param. Rendered via `{{ }}`
interpolation only (NEVER `v-html`, per `vue/no-v-html`), so no XSS. Vue Router
encodes the value; `stringifyFilterQuery` (filters.ts:206) is collision-safe.
Backend does string equality, not query construction — no injection surface.

**Security-Sensitive Operations:** None new. No file access, auth, or crypto.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see per-criterion *Test:* notes under Acceptance Criteria.

**Edge Cases:**

- 0 candidates → render "All"-only select; no crash.
- Exactly 10 candidates → select; 11 → typeahead (boundary test).
- Multiple source types → candidates merged across types, sorted by title
case-insensitive.
- Duplicate display titles → collapse to distinct options; value is the shared
title; selected-state resolves to first match (documented).
- Candidate whose `_title` floors to id (type without display_property) → value
is that floored title, which the backend's `DisplayTitle` also floors to the
same string, so it still matches (load-bearing equivalence — same function both
sides; breaks if `Title (ID)` is ever sent).
- `fetchList` rejects (cancelled fetch on rapid nav) → `isCancelledFetch`
suppressed, widget stays empty, no console error spam.
- Property + relation filter on the same list → both render, no `localFilters`
cross-talk.
- External `props.filters` update while typeahead search box has text → search
box preserved (local), committed selection updates (RR-NH8B6D).

**Negative Tests:**

- Unknown relation name in config → `getRelationType` undefined → fall back,
no throw.
- Select then clear → emitted state omits the key (no stale `value: ''`).
- Hyphenated relation deep link → dropped on parse (documented limitation,
RR-3TJVQJ); assert atlas relations are identifier-safe.

**Integration:** e2e in `e2e/tests/` — load a list with a relation
`filter_controls`, open the selector, pick a target, assert the row set narrows;
clear → full list returns.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Extraction destabilizes RelationPicker* → additive extraction; RelationPicker
keeps its dropdown; RelationPicker.test.ts stays green as a guard.
- *`per_page: 100` truncation* → known limitation; fine for atlas.
- *Sending `Title (ID)` or id instead of bare title* → the top regression risk
(RR-X4QWBF); pinned by unit test asserting the emitted value equals
`entityDisplayTitle`, and e2e that the list actually narrows.
- *Search-vs-committed clobber* → mitigated by component-local `searchQuery`
(RR-NH8B6D); unit test for external-update-while-typing.

**Effort:** s.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md (or the filter_controls section) — note relation filter
controls now render as a target selector. Verify exact file at implement time.
- [x] N/A for metamodel/cli/README — no schema or command changes.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-3MDVZD (critical, addressed), RR-X4QWBF
(critical, addressed), RR-NH8B6D (significant, addressed), RR-A51QQ2
(significant, addressed), RR-3TJVQJ (significant, addressed — scoped out with
documented limitation). All folded into the Approach above.
