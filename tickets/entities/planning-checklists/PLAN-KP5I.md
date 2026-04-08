---
id: PLAN-KP5I
type: planning-checklist
title: 'Planning: Sync data-entry list filters with URL query params'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- New filter state shape: `Record<string, {value, op?}>` so operators round-trip
- Read filter state from URL query params on list mount and route change
- Write filter state to URL via `router.replace` (no history spam)
- Override semantics: URL params override `filter_controls` only; static `filters:` from config remain locked
- Format: `filter[prop]=value` (default `eq`) and `filter[prop][op]=value` for non-default operators
- Migrate the existing `filter_*` (underscore) consumers to the new bracket format: `EntityList.navigateToEntity`, `useScopeNavigation`, `SearchView`
- Extract a shared `useUrlFilterSync` composable so all three callers share the read/write logic
- Add `fromApiOperator()` and `parseFilterQueryParams()` to `filters.ts`
- Backend test for percent-encoded brackets (`%5B`/`%5D`)
- Backend: support repeated query params for multi-value (`filter[tags][in][]=a&filter[tags][in][]=b`) — fixes existing `values[0]` truncation
- Frontend: 250ms debounce on text widget filter changes
- Collision detection: when a static `filters:` entry shares a property with a URL filter, log a warning and skip the URL filter (so the user doesn't get a silent zero-result trap)

OUT of scope:
- Pagination URL sync (`?page=2`) — separate ticket
- Sort URL sync (`?sort=-due_date`) — separate ticket
- Filter URL params for kanbans/views — list only
- Per-list URL state in localStorage — separate ticket
- Range filters (`lt+gt` on same property) — document as v2 (RR-6P2C deferred)

**Acceptance Criteria:**

1. AC1: Navigating to `/v2/list/all_tasks?filter[status]=todo` shows the list pre-filtered with `status=todo`, FilterBar widget reflects it
2. AC2: Changing a filter control updates the URL via `router.replace` (no new history entry per keystroke)
3. AC3: Removing a filter from the FilterBar removes the param from the URL (delete, not merge)
4. AC4: Browser back/forward navigates filter history
5. AC5: A static `filters:` entry in `data-entry.yaml` cannot be overridden by URL filters; collision logs a warning and the URL filter is skipped
6. AC6: URL with operator like `filter[due_date][lte]=$today` is parsed correctly AND round-trips back to URL with the operator preserved
7. AC7: Multi-select values survive URL → state → URL round trip via repeated param form
8. AC8: Clearing all filters removes all `filter[*]` params from the URL while preserving non-filter params (`from`, `sort`, `page`, `scope`)
9. AC9: Text input filters debounced at 250ms — typing "todo" results in 1 backend request, not 4
10. AC10: Existing entity-detail back-navigation (via `useScopeNavigation`) reads the new bracket format and works
11. AC11: Backend correctly parses `%5Bstatus%5D` (percent-encoded brackets) — Vue Router emits this form

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing code:**

- `frontend/src/components/lists/EntityList.vue:26` — `filters` ref (Record<string,string>) — needs reshape
- `frontend/src/components/lists/EntityList.vue:91-136` — `queryParams` builds API request from filters
- `frontend/src/components/lists/EntityList.vue:240-245` — current `navigateToEntity` writes `filter_*` (underscore) — must migrate
- `frontend/src/components/lists/FilterBar.vue:91-110` — emits filter event, multi-select uses comma-join
- `frontend/src/composables/useScopeNavigation.ts:29-71` — reads `filter_*` query params — must migrate
- `frontend/src/views/SearchView.vue:403-449` — reads `filter_*` and `q`/`type` query params — must migrate filter parts
- `frontend/src/utils/filters.ts:8-32` — `OPERATOR_MAP`, `toApiOperator`, `buildFilterKey`
- `internal/dataentry/api_v1.go:1217+` — `applyV1Filters`, `values[0]` truncates multi-value (must fix for AC7)
- `internal/dataentry/api_v1_test.go:1693+` — existing filter tests use literal brackets, no `%5B` test (must add for AC11)

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical approach:**

### 1. New filter state shape

```typescript
// frontend/src/types/filters.ts (new)
export interface FilterValue {
  value: string
  op?: string  // UI operator symbol (=, !=, <, <=, >, >=, ~, in)
               // omitted means "="
}

export type FilterState = Record<string, FilterValue>
```

`EntityList.vue` `filters` ref changes from `Record<string, string>` to
`FilterState`. `FilterBar.vue` `localFilters` ref same change, plus it now needs
to know per-property defaults. `queryParams` writer uses `filter[prop]` when
`op` is undefined or `eq`, else `filter[prop][op]`.

### 2. Centralized URL ↔ state in a composable

```typescript
// frontend/src/composables/useUrlFilterSync.ts (new)
export function useUrlFilterSync(opts: {
  // Static filters from config — used for collision detection.
  staticFilterProperties: () => Set<string>
}) {
  const route = useRoute()
  const router = useRouter()
  const filters = ref<FilterState>({})

  // Synchronously seed from current query — must be called in setup, NOT onMounted
  function readFromQuery() {
    const fromUrl = parseFilterQueryParams(route.query)
    const blocked = staticFilterProperties()
    for (const prop of Object.keys(fromUrl)) {
      if (blocked.has(prop)) {
        console.warn(`URL filter for "${prop}" ignored (locked by static config filter)`)
        delete fromUrl[prop]
      }
    }
    filters.value = fromUrl
  }
  readFromQuery()

  // Write filters back to URL, preserving non-filter params
  let lastWrittenSig = ''
  function writeToQuery(newFilters: FilterState) {
    filters.value = newFilters
    const newQuery = buildQueryWithFilters(route.query, newFilters)
    lastWrittenSig = stringifyFilterQuery(newQuery)
    router.replace({ query: newQuery })
  }

  // React to back/forward navigation
  watch(() => route.query, (q) => {
    if (stringifyFilterQuery(q) === lastWrittenSig) return  // self-write, ignore
    readFromQuery()
  })

  return { filters, writeToQuery }
}
```

The `lastWrittenSig` comparison is self-healing (no stuck-gate problem from
RR-E3LY).

### 3. New `filters.ts` helpers

```typescript
// Inverse of OPERATOR_MAP
export const API_TO_UI_OPERATOR: Record<string, string> = Object.fromEntries(
  Object.entries(OPERATOR_MAP).map(([ui, api]) => [api, ui])
)

export function fromApiOperator(op: string | undefined): string {
  return API_TO_UI_OPERATOR[op || 'eq'] || '='
}

// Parse `route.query` into FilterState. Handles both filter[prop] and
// filter[prop][op] forms. Multi-value (filter[prop][in][]) collected as array.
export function parseFilterQueryParams(query: LocationQuery): FilterState

// Build the query object for router.replace, preserving non-filter params
// and serializing FilterState into bracket form.
export function buildQueryWithFilters(currentQuery: LocationQuery, filters: FilterState): LocationQuery

// Deterministic string for "did the URL change because of us" comparison
export function stringifyFilterQuery(query: LocationQuery): string
```

### 4. Migration of existing `filter_*` consumers

- **navigateToEntity in EntityList.vue**: stop writing `filter_<prop>=value`. Instead pass current `route.query` through (the bracket-format filters are already there). The entity detail page can read them without translation.
- **useScopeNavigation.ts**: replace the `filter_*` reading loop (lines 66-71) with a call to `parseFilterQueryParams`.
- **SearchView.vue**: same — migrate filter restoration to use `parseFilterQueryParams`. Keep `q` and `type` as-is since those aren't `filter_*`.

### 5. Backend changes for AC7 and AC11

`internal/dataentry/api_v1.go applyV1Filters`:

- Currently `value := values[0]`. Change to handle multi-value: when key ends in `[]` strip suffix and treat as `in` operator with `values` joined; otherwise still `values[0]`.
- Add tests with `filter%5Bstatus%5D=open`, `filter%5Bdue_date%5D%5Blte%5D=2026-04-07`, `filter%5Btags%5D%5Bin%5D%5B%5D=a&filter%5Btags%5D%5Bin%5D%5B%5D=b` to lock in the percent-encoded behavior.

### 6. Debouncing

Use `lodash.debounce` (already in deps if available, else add) or a small
`useDebouncedFn` composable. Wrap the FilterBar text input handler (currently
`@input="handleFilterChange"`) with a 250ms debounce. Select/multi-select stay
immediate.

### 7. Edge cases addressed

- **Mount race (RR-2I3H)**: `readFromQuery()` is called synchronously in composable setup, before `loadEntities()` runs. No second fetch.
- **Clear filters (RR-G78J)**: `buildQueryWithFilters` with empty `FilterState` strips all `filter[*]` keys before merging non-filter params.
- **filter_controls defaults vs URL (RR-ZHB6)**: FilterBar's `initializeFilters` reads from props; URL is loaded first into props, so URL wins.
- **Range filters (RR-6P2C)**: deferred. Document that the same property with two operators in one URL keeps last-value-wins for v1.
- **Null query values (RR-0RMV)**: `parseFilterQueryParams` accepts `LocationQuery` type and skips `null`/`undefined`/empty values.

### Files to modify

**Frontend:**
- `frontend/src/types/filters.ts` (new) — `FilterState` and `FilterValue` types
- `frontend/src/utils/filters.ts` — add `fromApiOperator`, `API_TO_UI_OPERATOR`, `parseFilterQueryParams`, `buildQueryWithFilters`, `stringifyFilterQuery`
- `frontend/src/utils/filters.test.ts` — tests for the new helpers
- `frontend/src/composables/useUrlFilterSync.ts` (new) — the shared composable
- `frontend/src/composables/useUrlFilterSync.test.ts` (new) — tests
- `frontend/src/composables/useDebouncedFn.ts` (new, small) — or just use lodash if available
- `frontend/src/components/lists/EntityList.vue` — switch to new shape, use composable, debounce text inputs, update navigateToEntity
- `frontend/src/components/lists/FilterBar.vue` — accept new shape from props, emit new shape, debounce text inputs
- `frontend/src/composables/useScopeNavigation.ts` — read bracket form via parseFilterQueryParams
- `frontend/src/composables/useScopeNavigation.test.ts` — update tests for new format
- `frontend/src/views/SearchView.vue` — migrate filter restoration

**Backend:**
- `internal/dataentry/api_v1.go` — add multi-value support in `applyV1Filters` (handle `filter[prop][in][]`)
- `internal/dataentry/api_v1_test.go` — add tests for `%5B`-encoded brackets and repeated multi-value params

**Docs:**
- `docs-project/entities/guide/GUIDE-data-entry.md` — document URL sync, format, override semantics, debouncing

**Alternatives considered:**

- **Computed-only filters from route (L1)**: rejected for v1 because text input would need a parallel local-state buffer for unflushed keystrokes. Composable approach is closer to existing patterns.
- **Push instead of replace**: rejected — debounce + replace gives the same UX without history pollution
- **Keep `filter_*` underscore format**: rejected — two formats on the same view is the bug we're fixing

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **URL query params** — fully user-controlled. Frontend translates to API; backend already validates per #327 (operator enum, type-mismatch errors).
- **No XSS**: filter values flow into v-model inputs and API params. No `v-html` / `eval`.
- **No infinite loops**: `lastWrittenSig` comparison prevents watcher → router.replace → watcher cycles even under errors.
- **No collision exploitation**: collision detection logs and skips, doesn't surface to backend at all.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. **filters.ts unit tests**:
   - `fromApiOperator('lte') === '<='`, `fromApiOperator(undefined) === '='`, `fromApiOperator('bogus') === '='`
   - `parseFilterQueryParams({'filter[status]': 'open'})` → `{status: {value: 'open'}}`
   - `parseFilterQueryParams({'filter[due_date][lte]': '$today'})` → `{due_date: {value: '$today', op: '<='}}`
   - `parseFilterQueryParams({'filter[tags][in][]': ['a','b']})` → `{tags: {value: 'a,b', op: 'in'}}`
   - `parseFilterQueryParams({})` → `{}`
   - Null/empty/undefined values skipped
   - `buildQueryWithFilters({page:'2'}, {status: {value:'open'}})` → `{page:'2', 'filter[status]':'open'}`
   - `buildQueryWithFilters({'filter[old]':'x', from:'list'}, {})` → `{from:'list'}` (drops old filter, keeps non-filter)
   - Round-trip: `parse(build({}, X)) === X`
   - `stringifyFilterQuery` is order-independent

2. **useUrlFilterSync composable tests**:
   - Initial setup reads from `route.query`
   - Static collision: filter for blocked property is skipped + warned
   - `writeToQuery` calls `router.replace`, sets `lastWrittenSig`, watcher ignores the next echo
   - Watcher fires on external `route.query` change → `filters.value` updates
   - Watcher does NOT fire-loop on self-write

3. **useScopeNavigation tests**: update to assert bracket-format reading, not underscore

4. **Backend tests** (api_v1_test.go):
   - `httptest.NewRequest("GET", "/api/v1/tickets?filter%5Bstatus%5D=open")` → returns matching entities
   - `httptest.NewRequest("GET", "/api/v1/tickets?filter%5Bdue_date%5D%5Blte%5D=2026-04-07")` → operator-with-percent-encoding
   - `httptest.NewRequest("GET", "/api/v1/tickets?filter%5Btags%5D%5Bin%5D%5B%5D=a&filter%5Btags%5D%5Bin%5D%5B%5D=b")` → multi-value via repeated params
   - Mixed encoded/unencoded

5. **Manual end-to-end (puppeteer)**:
   - Navigate to PIM `/v2/list/all_tasks?filter[status]=todo` → list pre-filtered, FilterBar reflects
   - Change FilterBar status → URL updates
   - Browser back → URL reverts, FilterBar reverts
   - Type rapidly in a text filter → only one backend request after 250ms
   - Click an entity → return to list → bracket-format URL preserved
   - Try `filter[status]=closed` on a list with `filters: status=open` static config → see warning in console, list shows static filter result

**Edge cases:**

- Empty values (cleared input) → key removed from URL
- URL filter on property without a control config → applied to API but no widget renders it (visible in static-filter chips? Out of scope for v1)
- Special chars in values → URL-encoded by Vue Router
- Missing op in URL → default `eq`
- Multi-select with comma-in-value → use repeated param form, not comma-split (RR-PSKQ)

**Negative tests:**

- Malformed `filter[]=` or `filter` keys → ignored
- Filter for non-existent property → passed to backend (which ignores it)
- Filter value with `<script>` → escaped by Vue (no XSS)
- Same property twice in URL → last wins (documented v1 behavior)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Watcher loop**: mitigated by `lastWrittenSig` comparison
- **Mount race**: mitigated by synchronous setup-time read
- **Multi-value comma data loss**: mitigated by repeated param form on both ends
- **Migration of `filter_*` consumers** introduces regression risk in entity scope navigation — mitigated by updating useScopeNavigation tests

Effort: **L** (was M before scope expansion)

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] User guide / reference docs (data-entry.md filter section: URL sync, format, override semantics, debouncing)
- [ ] ~~CLI help text~~ (N/A)
- [ ] ~~CLAUDE.md~~ (N/A)
- [ ] ~~README.md~~ (N/A)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Critical (addressed):
- RR-JJM4 — New filter state shape `Record<string, {value, op?}>` round-trips operators
- RR-7NAS — Migrate all `filter_*` consumers to bracket format; single source of truth

Significant (addressed):
- RR-E3LY — `lastWrittenSig` comparison instead of `syncingFromUrl` gate
- RR-T5RQ — Collision detection: skip URL filter for blocked properties, log warning
- RR-2I3H — Synchronous setup-time read before first `loadEntities`
- RR-M5LD — 250ms debounce on text widget filter changes
- RR-PSKQ — Repeated param form for multi-value (`filter[tags][in][]=a&filter[tags][in][]=b`); fixes backend `values[0]` truncation
- RR-8M2G — Backend tests for `%5B`/`%5D` encoded brackets
- RR-1JY8 — `useUrlFilterSync` composable shared by EntityList, useScopeNavigation, SearchView

Minor (addressed):
- RR-Y083 — AC8 explicitly mentions preserving non-filter params
- RR-0RMV — `parseFilterQueryParams` typed against `LocationQuery`, handles null
- RR-G78J — `buildQueryWithFilters` actively deletes filter keys, not merges
- RR-ZHB6 — FilterBar reads from props after URL load, URL wins by ordering

Deferred (with reason):
- RR-6P2C — Range filters (`lt`+`gt` on same property) — last-write-wins documented, follow-up ticket
- RR-JZKU — EntityList component test setup deferred; rely on Playwright e2e + composable unit tests instead
