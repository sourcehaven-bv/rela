---
id: PLAN-MWQHUZ
type: planning-checklist
title: 'Planning: History/diff views: put selected versions in the URL so a diff is shareable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The two history views keep the compared version pair in local refs (`baseSel` /
`targetSel`, `type Side = number | 'current'`) and never touch the URL. A user
looking at an interesting diff has no way to hand that diff to a colleague,
bookmark it, or get it back after a reload.

**User decisions (asked 2026-07-27):**

1. **Query params, not path params.** `?base=&target=` on the existing routes.
Path segments would need new route patterns plus an optional-segment scheme on
both routes, for no user-visible gain, and would sit awkwardly beside the
existing `return_to` / `from` query conventions.
2. **`current` stays live-relative.** A shared `?base=3&target=current` link
means "what changed since v3", evaluated against the entity as it is when the
recipient opens it — the same reading the sender had on screen. No
resolve-on-copy, no pinning, no copy-link button.

**Scope:**

IN:

- `HistoryView.vue` (entity) and `RelationHistoryView.vue` (relation): seed
`baseSel`/`targetSel` from `?base=` / `?target=`, write them back on every
selection change, and react to external navigation (back/forward, deep link).
- A shared composable for the parse/serialize/echo-guard logic, since the two
views would otherwise carry an identical copy.
- Validation of the incoming params against the loaded version list.
- `:data-version` on the entity view's timeline rows (currently only the
relation view has it) so an e2e page object can address rows.
- Unit tests for the composable; e2e assertions for deep-link + reload.

OUT:

- A "Copy link" affordance or any pinning of `current` to an ordinal (decision 2).
- Path-param routes (decision 1).
- Any backend change. This is entirely frontend; the `_history` endpoints
already accept a version ordinal and need nothing new.
- Making `goBack()` round-trip `return_to` (a real gap noted during research,
but pre-existing and unrelated — separate ticket if wanted).
- Unifying the two near-duplicate views. Tempting, but a much larger refactor
than this ticket, and doing it here would bury the URL change.

**Acceptance Criteria:**

1. **Deep link selects the pair.** Opening
`/history/<type>/<id>?base=2&target=5` renders the v2 → v5 diff, with both
dropdowns showing those versions and the timeline highlighting v2 — without the
user touching a control. *Test:* e2e navigates directly to the URL, asserts
`.compare-select` values and the `.compare-caption` text.
2. **Selection writes the URL.** Changing either dropdown, clicking a timeline
row, or hitting swap updates `?base=`/`?target=` to match. *Test:* unit test on
the composable asserts `router.replace` payload; e2e asserts `page.url()` after
a `selectOption`.
3. **Reload is stable.** Reloading any history URL reproduces the same diff
rather than resetting to defaults. *Test:* e2e picks a non-default pair,
reloads, asserts the pair survived.
4. **`current` round-trips.** `?target=current` is preserved verbatim as the
sentinel (not coerced to a number), and the diff is against the live entity.
*Test:* unit test for parse/serialize of the sentinel; e2e asserts the caption
reads `current` (entity view) / `latest` (relation view).
5. **No params = today's behaviour.** A bare `/history/<type>/<id>` still opens
on the existing defaults (entity: newest → current; relation: second-newest →
newest) and writes those defaults to the URL once loaded. *Test:* e2e opens the
bare URL, asserts the default caption.
6. **Bad params degrade to defaults, silently and safely.** `?base=999`,
`?base=abc`, `?base=-1`, `?base[]=1&base[]=2`, `?base=3` on an entity with 2
versions — all fall back to the default pair. No crash, no error toast, no
failed fetch. *Test:* unit tests, one case per malformed input.
7. **No history spam.** Changing selections uses `router.replace`, so the
browser Back button leaves the history view rather than stepping back through
every dropdown fiddle. *Test:* unit test asserts `replace` (not `push`) is
called.
8. **Restore still works.** After a restore the version list grows; the view
re-seeds to fresh defaults and the URL follows. *Test:* covered by extending the
existing relation-history e2e restore test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, well-bounded frontend change with three existing
in-repo precedents. A `/research` doc would restate what the codebase survey
below already settled.

**Existing Solutions:**

No library needed. `vue-router` already provides everything (`route.query` +
`router.replace`), and the repo has a documented house pattern for exactly this
— **seed / replace / echo-guard**:

- `frontend/src/composables/useUrlFilterSync.ts:1-120` — the canonical
implementation, used by `EntityList.vue:178`. Its header comment (`:1-22`)
states the three rules: seed synchronously at setup so the first fetch already
sees URL state; route all writes through one function that uses `router.replace`
and records a signature; a `watch(() => route.query)` that skips its own echo by
comparing that signature.
- `frontend/src/composables/useFormWizard.ts:182-224` — **the closest analogue,
and the one to copy.** It syncs a single scalar `?step=N`, with a scalar
`lastWritten` echo guard (`:205-212`), a `clamp()` for out-of-range values
(`:184-188`), and — critically — a re-runnable `seedFromUrl()` (`:197-203`)
because *the valid range isn't known until async data lands*. That is exactly
our situation: `?base=7` can't be validated until `listVersions` resolves. Its
own comment even says it mirrors `useUrlFilterSync`'s pattern.
- `frontend/src/components/entity/DocumentsPanel.vue:69-83` — lightest variant
(`?doc=`), seeds via `watch(..., {immediate:true})`, guards with a plain
equality check. Its `:79-81` comment documents the replace-over-push rationale.
- `frontend/src/views/SearchView.vue:130-142` — an **older hand-rolled variant
with no echo guard that also clobbers the whole query object**. Explicitly not
the model to follow; noted so a reviewer doesn't point at it as precedent.

Also relevant: `frontend/src/composables/useScopeNavigation.ts` and
`useBackTarget.ts:34-49` already read `from` / `q` / `return_to` off
`route.query` on entity pages. Our writes must **merge** into `route.query`
rather than replace it, or we'd break `return_to` round-tripping for anyone who
later wires it through to the history view.

Prior art in the graph: `FEAT-5UYW8F` (pgstore content versioning) and
`TKT-SCXHUL` (relation-history e2e) — this ticket is a UX follow-up on that
feature, not new capability.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add one composable, `useVersionSelectionSync`, and wire it into both views. It
owns the `Side` type's URL representation and nothing else — the views keep
owning fetching, diffing, and restore.

```ts
// frontend/src/composables/useVersionSelectionSync.ts
export type Side = number | 'current'

export interface UseVersionSelectionSyncOptions {
  // Ordinals that actually exist, supplied by the view once listVersions
  // resolves. Empty until then, which is why seeding must be re-runnable.
  validVersions: () => number[]
  // The view's default pair — the two views disagree (entity: newest→current,
  // relation: second-newest→newest), so the composable must not hardcode one.
  defaults: () => { base: Side; target: Side }
}
```

Returned surface: `base`, `target` (refs the views bind with `v-model`),
`seedFromUrl()`, `write(base, target)`, and `onExternalChange(cb)` — or, more
simply, the composable owns the refs and the view passes a `recompute` callback
to run whenever the pair changes from any source. I'll settle the exact shape
during implementation; the constraint is that **every** mutation path (dropdown,
timeline click, swap, external nav) converges on one write function and one
recompute, so no path can update the URL without recomputing the diff or vice
versa.

Parse rules (the fiddly part, and the source of most of the edge cases):

- `'current'` → the sentinel string, preserved as-is. Note the type asymmetry
this guards: the `<option value="current">` is a **string** while version
options bind `:value="m.version"` (a **number**). Parsing `?base=3` to the
string `"3"` would make `v-model` silently fail to match any option — the
dropdown would render blank. So a numeric param must be coerced with `Number()`,
not left as a string.
- A string of digits → `Number()`, then accepted **only if present in
`validVersions()`**. This is an allowlist against the real version list, not a
range check — it rejects `999`, `-1`, `0`, `1.5`, and `abc` in one step.
- An array (`?base=1&base=2`, which vue-router surfaces as `string[]`) → take
the last element, matching `useUrlFilterSync`'s `readQParam` (`:41-48`).
- Anything else, or a value not in the allowlist → fall back to that side's
default.

Seeding is **two-phase**, following `useFormWizard`:

1. Synchronously at setup, so a deep link is honoured before first paint where
possible. At this point `validVersions()` is empty, so this pass only captures
the raw intent.
2. Re-run inside `load()` immediately after `listVersions` resolves and the
defaults are computed, now with a populated allowlist. This is the pass that
actually decides the pair.

Then write the resolved pair back (`router.replace`), so a bare URL becomes an
explicit shareable one and AC5 holds.

The echo guard is a scalar signature (`` `${base}|${target}` ``) compared in the
`watch(() => [route.query.base, route.query.target]) ` handler, exactly as
`useFormWizard:205-223 ` does.

Two details that must not be missed:

- **`load() ` runs on mount and again after a restore** (`HistoryView.vue:166 `).
Re-seeding from the URL after a restore would resurrect a stale pair against a
changed version list, so the post-restore path must reset to fresh defaults
rather than re-read the URL (AC8). Cleanest way: give `load() ` a `seedFromUrl:
boolean ` parameter — `true ` from `onMounted `, `false ` from `restore `.
- **The relation view's `sideState ` maps `'current' ` to the newest snapshot**
(`RelationHistoryView.vue:88 `), and labels it `latest ` (`:45 `) where the
entity view says `current ` (`HistoryView.vue:44 `). The URL token stays
`current ` in both for consistency; only the display label differs. Worth a
code comment, since it looks like an inconsistency otherwise.

**Files to modify:**

| File | Change |
|---|---|
| `frontend/src/composables/useVersionSelectionSync.ts ` | **New.** Parse/serialize/seed/write/echo-guard. |
| `frontend/src/composables/useVersionSelectionSync.test.ts ` | **New.** Unit tests (model: `useUrlFilterSync.test.ts `). |
| `frontend/src/views/HistoryView.vue ` | Replace local `baseSel `/`targetSel ` with the composable; `load(seedFromUrl) `; route every mutation through the write path; add `:data-version ` to `.timeline-item ` (`:217-222 `). |
| `frontend/src/views/RelationHistoryView.vue ` | Same, minus `:data-version ` (already present at `:193 `). |
| `e2e/pages/history.page.ts ` | **New.** Entity-history page object (only the relation one exists). |
| `e2e/pages/relation-history.page.ts ` | Add URL/deep-link helpers. |
| `e2e/pages/index.ts ` | Export the new page object. |
| `e2e/tests/history-url-params.spec.ts ` | **New.** Deep-link + reload + URL-write, postgres-gated. |
| `docs/data-entry.md ` | Document the shareable-diff URL params. |

**Alternatives considered:**

- **Path params** — rejected by the user (decision 1); would need new route
patterns and an optional-segment scheme for zero gain.
- **A single combined param** (`?diff=3..current `, git-style) — compact and
rather elegant, but it invents a parse format where two plain params need none,
and it doesn't compose with `buildQueryWithFilters `-style merging. Rejected as
cleverness.
- **Inline the sync in both views** (no composable) — ~40 near-identical lines
duplicated across two files that are *already* near-duplicates of each other.
`npm run dupes ` (jscpd) would likely flag it, and the two copies would drift.
- **Resolve `current ` to an ordinal on load** so links are always frozen —
rejected by the user (decision 2); also changes what the existing dropdown
means.

**Dependencies:** none new. `vue `, `vue-router `, existing `@/api/history `.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

One new untrusted input: the `base ` / `target ` query params, fully
attacker-controlled since the whole point is that these URLs get shared.

| Input | Validation | On invalid |
|---|---|---|
| `?base= `, `?target= ` | Allowlist: the literal `current `, or a number present in the version list returned by the server. `string[] ` → last element. | Silent fallback to that side's default. No toast, no error state. |

The allowlist is the important property: the param is **never** interpolated
into a request path before being matched against a server-supplied ordinal. A
crafted `?base=../../foo ` or `?base=<script> ` fails the `validVersions() `
membership test and is discarded, so it never reaches `getVersion(type, id, s)
` and never reaches the DOM. Validating against the real list rather than a
numeric range also means a user can't probe for versions outside their lineage.

**Security-Sensitive Operations:**

- **No new authorization surface.** The params choose *among versions the server
already returned to this user* via `listVersions `. Read gating stays entirely
server-side: entity history through the visibility read path, relation history
gated on **both** endpoints (FROM ∧ TO) per the root `CLAUDE.md `. A shared
link is not a capability — a recipient without read access gets the same 404/403
they'd get without the link. Worth stating plainly because "shareable link" is
exactly the phrase that tends to smuggle in an authorization assumption; here it
does not.
- **No leakage through the URL itself.** `base `/`target ` are small integers or
the word `current ` — no content, no principal, no property values. A history
URL pasted into a chat reveals the entity id and type, which the existing
`/history/:type/:id ` route already did.
- **Error handling unchanged.** Bad params produce a default view, not a
message; nothing echoes the raw param back into the page (no reflected-XSS
vector, and Vue escapes text interpolation regardless).
- Unchanged: 501 → "not available for this deployment", which leaks only the
build flavour, as today.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Level | Test |
|---|---|---|
| 1 deep link | e2e + unit | Navigate to `?base=2&target=5 `; assert both `.compare-select ` values, the `.compare-caption `, and the highlighted timeline row. Unit: seeding with a populated allowlist yields the pair. |
| 2 writes URL | unit + e2e | Unit: assert `router.replace ` called with the merged query. e2e: `selectOption ` then assert `page.url() `. |
| 3 reload | e2e | Select a non-default pair, `page.reload() `, assert the pair survived. |
| 4 `current ` | unit + e2e | Unit: `'current' ` parses to the sentinel and serializes back unchanged (and is **not** coerced to a number). e2e: caption reads `current ` / `latest `. |
| 5 bare URL | e2e | Open bare URL; assert default caption; assert the URL gains explicit params after load. |
| 6 bad params | unit | One case per malformed input (table below). |
| 7 no spam | unit | Assert `replace ` called, `push ` not called. |
| 8 restore | e2e | Extend the existing relation-history restore test: after restore the URL reflects fresh defaults, not the pre-restore pair. |

Unit tests follow `useUrlFilterSync.test.ts:1-46 `: mock `vue-router ` with a
`reactive({query}) ` route and a `mockReplace ` that writes back into
`mockRoute.query ` (so the watcher genuinely fires), and run the composable in
a per-test `effectScope() ` so watchers are torn down.

E2E lives in `e2e/tests/ `, uses the `postgresTest ` fixture, and **skips
unless `RELA_E2E_DATABASE_URL ` is set** — history is pgstore-only, exactly as
`e2e/tests/relation-history.spec.ts:19-22 ` does.

**Edge Cases:**

| Case | Expected |
|---|---|
| `?base= ` (empty) | Default for that side. |
| `?base=abc ` | Default. |
| `?base=-1 `, `?base=0 ` | Default (not in allowlist; ordinals are 1-based). |
| `?base=1.5 ` | Default. |
| `?base=999 ` (beyond the list) | Default — **no fetch attempted**. |
| `?base=3 ` where the entity has 2 versions | Default. Same path as above; called out because it's the realistic case — a link shared before a lineage was purged or from a different entity. |
| `?base=1&base=2 ` (array) | Last wins (`2 `), matching `readQParam `. |
| Only one param supplied | That side seeds, the other takes its default. |
| `?base=2&target=2 ` | Allowed — an empty diff is a legitimate thing to link to. |
| Entity with **zero** versions | Existing "No versions recorded yet." state; params ignored; no write to the URL (nothing meaningful to write). |
| `?base=current&target=current ` | Allowed; empty diff. |
| Rapid dropdown changes | Existing `recomputeSeq ` guard (`HistoryView.vue:124-135 `) still drops stale results; the URL reflects the last selection. Must verify the guard survives the refactor. |
| Back/forward between two pairs | Watcher re-seeds and recomputes; echo guard prevents a write loop. |
| Restore while a non-default pair is selected | Fresh defaults (AC8), not a stale pair against a changed list. |
| Params present but the deployment returns 501 | Unsupported state, params ignored — no crash. |

**Negative Tests:**

Every malformed input above must fall back **silently**: no thrown error, no
`uiStore.showToast('error', ...) `, no request issued with an invalid ordinal.
A test asserts `getVersion ` is never called with an out-of-list value — that's
the assertion that actually pins the allowlist behaviour rather than just its
symptom.

Also negative-tested: the write path must not clobber unrelated query params
(assert an existing `?return_to=x ` survives a selection change).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Mitigation |
|---|---|---|
| **Infinite loop** — write → watcher → write. The classic failure of this pattern. | Medium | Scalar signature echo guard, copied from `useFormWizard:205-223 `; explicit unit test asserting `replace ` is called once per selection change. |
| **`v-model ` type mismatch** — parsing `"3" ` as a string leaves the dropdown blank because options bind numbers. Would look like "deep links just don't work". | Medium | Coerce with `Number() `; unit test asserts `typeof base === 'number' `; e2e asserts the visible select value, which is what would actually catch it. |
| **Re-seed after restore resurrects a stale pair.** | Medium | `load(seedFromUrl: boolean) `; AC8 e2e test. |
| **Regressing the `recomputeSeq ` stale-diff guard** while rerouting mutation paths. | Low-Medium | Keep the guard untouched; every path still funnels into the existing `recompute() `. |
| **Two views drift** — one gets the fix properly, the other approximately. | Medium | Shared composable rather than two copies; run both view's e2e. |
| Query-param collision with `return_to ` / `from `. | Low | Merge into `route.query ` (never replace it); explicit negative test. |
| Router file is excluded from v8 coverage (`router/index.ts:1 `) | Low | No router change needed under decision 1 — this risk is now moot, noted for the record. |
| Frontend coverage floor | Low | New composable ships with its own unit tests; `FEAT-wzwp ` ratchet applies. |

**Effort:** `s ` — one small composable, two view edits, a page object, and
tests. The estimate holds mainly because the pattern is already established
three times over in this codebase; the genuine work is the edge-case handling
around the async allowlist, not the sync itself.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md ` — document that history/diff views accept
`?base= ` / `?target= ` and that such links are shareable, including the
live-relative meaning of `current ` (decision 2) so nobody expects a frozen
diff.
- [ ] ~~`docs/metamodel.md `~~ (N/A: no metamodel change)
- [ ] ~~`docs/cli-reference.md `~~ (N/A: no CLI change)
- [ ] ~~`CLAUDE.md `~~ (N/A: uses the existing URL-sync pattern rather than
introducing one; `frontend/CLAUDE.md ` already covers composables)
- [ ] ~~`README.md `~~ (N/A: not project-level)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the plan was
      presented to the user and explicitly approved ("looks good; continue")
      before any code was written, and the two decisions that actually shaped
      the design — query-vs-path params, and whether `current` pins — were put
      to the user directly rather than inferred. For an `s`-sized change that
      reuses an established in-repo pattern with three precedents, a separate
      design-review pass would have re-litigated settled ground.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design
      review run — see above)

**Design Review Findings:** None — skipped, see above. The design was instead
scrutinized at code-review time by `cranky-code-reviewer`, which independently
re-implemented the control flow to attack the echo guard and found no critical
issues; its four findings are recorded as RR-XM367L, RR-L4M1WK, RR-1QWV9S and
RR-W7L33A, all addressed. Notably RR-XM367L was a genuine *design* defect (the
two views' default pairs diverging in meaning) that a design review might have
caught earlier — worth remembering next time a change touches two parallel
views.
