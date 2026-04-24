---
id: PLAN-W7N0R
type: planning-checklist
title: 'Planning: Honor return_to as a back affordance on non-form screens'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** The document link rewriter (TKT-4MFUK) appends `?return_to=` only
on links that target form routes. Users who click through a document into e.g.
an entity detail, list, kanban, or custom view have no visible way back to the
document except the browser back button. The `rela.url.*` helpers announced
those routes as first-class targets, but the user experience on clicking one is
worse than the old `edit://` / `create://` schemes they replaced, because at
least those always led to a form with a cancel button.

**Scope — In.**
1. Extend the rewriter so internal (`/…`) hrefs other than form routes also
get `?return_to=` appended (path + stripped fragment).
2. **Fix server-side `isSafeReturnPath` case asymmetry** (RR-IFA9K): the Go
guard today rejects only uppercase percent-encoded separators; the TS guard
rejects both cases. Bringing this into scope because widening the rewriter to
every internal path expands the server guard's blast radius.
3. Define a single Back-button precedence rule: `?return_to=` wins, then
`?from=<list-id>` falls back, then no button.
4. Implement the rule as a shared `useBackTarget()` composable returning a
safe path + a label *hint* (not a resolved label — see RR-RV4LA).
5. Reuse the composable in:
   - The existing `scope-nav` Back button on `EntityView` and `CustomView`
(today hard-wired to `?from=`; switch to the composable).
   - `DocumentView` (replaces the bespoke `goBack()` function).
6. Add a minimal `<BackButton>` affordance to views that currently have no Back
UI: `ListView`, `KanbanView`, `AnalyzeView`, `SearchView`. Styling uses the
`scope-nav-btn` class.
7. `DynamicForm` is **NOT** refactored to use the composable (RR-5K8I2 resolution).
It keeps calling `readReturnTo(route.query)` directly. The shared primitive
between form and composable is `readReturnTo` in `returnPath.ts` — already
exists.
8. Security: the composable rejects unsafe `return_to` values via the
`isSafeReturnPath` guard — reject silently, fall through to `?from=` (symbolic
list id, not an arbitrary URL, so safe).
9. **Rewriter/cache contract** (RR-3Y6BM): explicitly preserve the invariant
that `return_to` is never baked into the on-disk cache — the rewriter runs
post-cache in `api_v1.go`, not inside `doRender`. Add a test asserting the cache
file contains no `return_to` tokens.

**Scope — Out.**
- `?from=` is **not** subsumed by `?return_to=` — scope-nav (prev/next through
a list) keeps reading `?from=` directly. Only the Back button within the
scope-nav bar changes.
- Dashboard / Settings / Conflicts screens — not a typical target of document
links; can be added later.
- Back-button label richness beyond title lookup (e.g. "Back to <doc title>").
For this ticket: `?return_to` → `← Back`; `?from=<list-id>` → `← <list title>`
when resolvable, else `← Back`.
- Multi-hop history stacks. Single-hop only.
- Refactor of `DynamicForm` to use the composable (see scope #7).

**Acceptance Criteria.**

AC1. **Rewriter injects return_to on non-form internal links.** A document
linking `[Detail](/entity/ticket/TKT-001)` rendered with `returnPath=/doc` emits
`<a href="/entity/ticket/TKT-001?return_to=/doc">`. External links, mailto:,
anchor-only, and legacy `edit://`/`create://` unchanged. Form routes keep their
existing id-anchor injection.

AC2. **Precedence rule.** Given URL `?return_to=/A&from=B`, Back routes to `/A`.
Given `?from=B` alone, Back routes to `/list/B`. Given neither, no Back button
renders. `?return_to=//evil.com` is treated as absent (fall through to `?from=`
or nothing).

AC3. **EntityView & CustomView scope-nav bar** show a Back button that follows
the precedence rule. Prev/Next unaffected (still driven by `?from=`).
**Prev/Next preserve `?return_to=` in the URL** so Back keeps pointing at the
original source across in-list navigation (RR-97NAZ).

AC4. **DocumentView's existing Back button** follows the precedence rule. Its
bespoke `?from=<list-id>` → `/list/<id>` handler is deleted; the composable
provides the fallback.

AC5. **ListView, KanbanView, AnalyzeView, SearchView** each render a Back button
when the composable returns non-null. Styling: existing `.scope-nav-btn` class
only (no new `.back-btn` — avoids collision with SearchView's filter-picker
`.back-btn`, RR-AY8HG). Button placed inside each view's `<header
class="page-header">` area.

AC6. **DynamicForm Cancel/submit** continues to honour `return_to` identically
to today. No refactor; existing `readReturnTo` call is the shared primitive.

AC7. **E2E behavioural** (RR-27ZFC). Test visits
`/entity/category/backend?doc=category_overview`, clicks a ticket-detail link
inside the doc, lands on the entity detail page, asserts a Back button is
visible, **clicks** the Back button, asserts URL matches the original (including
`?doc=category_overview`), and asserts the document body re-renders. No
assertion on the href attribute value — it's behaviour under test, not DOM
shape.

AC8. **Server isSafeReturnPath case-folded** (RR-IFA9K).
`internal/dataentry/return_path.go` rejects `/%5c`, `/%2f` (lowercase) in
addition to uppercase. A 4-row table test asserts both cases.

AC9. **Cache invariance** (RR-3Y6BM). A test renders a document twice with
different `return_to` values. Both responses carry the matching value; the
on-disk `.rela/documents/<entry>-<hash>.html` file contains zero occurrences of
the string `return_to=`.

AC10. **Rewriter idempotency** (RR-RFYUI). (a) `RewriteDocumentLinks(html, "/A",
nil)` then `RewriteDocumentLinks(result, "/A", nil)` is byte-equal to one pass.
(b) `RewriteDocumentLinks(html, "/A", nil)` then `RewriteDocumentLinks(result,
"/B", nil)` yields the `/B` variant, not both.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing code to reuse:**
- `frontend/src/utils/returnPath.ts` — `isSafeReturnPath(s)` and
`readReturnTo(query)`. Already open-redirect-hardened with 34 unit tests.
- `frontend/src/components/entity/EntityDetail.vue:361–383` — existing
`scope-nav` bar. Back is `<router-link :to="scopeNav.backUrl">`; class
`.scope-nav-btn` defined in scoped `<style>`.
- `frontend/src/views/CustomView.vue` — same scope-nav pattern.
- `frontend/src/views/DocumentView.vue:81–87` — bespoke `goBack()` (to be
deleted). `fromList` computed is used only here (verified by grep).
- `frontend/src/components/forms/DynamicForm.vue:119–122, 364–378` —
existing `return_to` read via `readReturnTo` + push on submit/cancel. NOT
modified by this ticket.
- `frontend/src/composables/useScopeNavigation.ts` — composable pattern
we're following.

**Not applicable / ruled out:**
- History API (`history.state`) for back — doesn't survive reload or deep-link.
- Subsuming `?from=` under `?return_to=` — rejected by user: "keep scope
navigation separate."

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Rewriter decision table** (RR-LJ8IB). Encodes behaviour for every combination
of path-class and returnPath-presence:

| Path class                    | `returnPath == ""`                             | `returnPath != ""`                                       |
|-------------------------------|------------------------------------------------|----------------------------------------------------------|
| Form (`/form/<id>[/...]`)    | Strip pre-existing `return_to`; emit anchor id; no injection. | Strip pre-existing `return_to`; emit anchor id; inject ours. |
| Non-form internal (`/...`)   | Strip pre-existing `return_to`; pass through.  | Strip pre-existing `return_to`; inject ours.             |
| External, mailto:, anchor-only | Passthrough unchanged.                       | Passthrough unchanged.                                   |
| Legacy `edit://`/`create://` | Log warning; passthrough.                     | Log warning; passthrough.                                |

Resolution of RR-AN2J8: **strip pre-existing `return_to` in every internal
branch**, so the rewriter is the single source of truth for the key on emitted
HTML. Author-planted `return_to` values are discarded regardless of
form-vs-non-form and regardless of whether the rewriter has a new one to inject.

**Technical approach.**

1. **Composable** `frontend/src/composables/useBackTarget.ts`:
   ```ts
   export interface BackTarget {
     to: string                  // validated, same-origin path
     labelHint: LabelHint | null // caller resolves text
   }
   export type LabelHint = { kind: 'list'; id: string }

   export function useBackTarget(): ComputedRef<BackTarget | null> {
     const route = useRoute()
     return computed(() => {
       const safe = readReturnTo(route.query)
       if (safe) return { to: safe, labelHint: null }
       const from = typeof route.query.from === 'string' ? route.query.from : null
       if (from) return { to: `/list/${from}`, labelHint: { kind: 'list', id: from } }
       return null
     })
   }
   ```
No `schemaStore` coupling here (RR-RV4LA). Returns a reactive `computed` because
`route.query` can change via `router.replace` (e.g. DocumentsPanel writing
`?doc=X`).

2. **BackButton component** `frontend/src/components/common/BackButton.vue`:
   - Props: `target: BackTarget` (required; parent uses `v-if`).
   - Resolves label: `← <list title>` if `labelHint.kind === 'list'` and the
list is known to `schemaStore`; else `← Back`. Label lookup happens here, NOT in
the composable — component already lives in the Vue/SPA layer where
`schemaStore` is expected.
   - Template: `<router-link :to="target.to" class="scope-nav-btn">{{ label }}</router-link>`.
   - Styling: imports from `src/styles/back-button.css` (shared file,
RR-AY8HG). The existing `.scope-nav-btn` CSS moves there; no new class.

3. **Global styles location.** `src/styles/back-button.css`, imported by
`main.ts`. App.vue is untouched (avoids the `max-lines: 500` warning, RR-AY8HG).
SearchView's existing scoped `.back-btn` (filter picker) keeps its scoped
declaration — different class name, no collision.

4. **EntityDetail / CustomView**: replace hard-coded `<router-link :to="scopeNav.backUrl">`
with `<BackButton :target="backTarget" v-if="backTarget">`. Prev/Next unchanged.
`scopeNav.backUrl` field removed — composable owns it.
`useScopeNavigation.navigateScope` unchanged — it already preserves
`route.query`, so `?return_to=` rides along on Prev/Next (AC3, RR-97NAZ).

5. **DocumentView**: replace `goBack()` + `<button @click="goBack">` with
`<BackButton :target="backTarget" v-if="backTarget">`. Delete `fromList`
computed.

6. **ListView / KanbanView / AnalyzeView / SearchView**: add
`<BackButton :target="backTarget" v-if="backTarget">` inside the existing
`<header class="page-header">` area.

7. **DynamicForm**: no changes. Keeps calling `readReturnTo(route.query)` at
mount for the submit/cancel redirect.

8. **Rewriter changes** in `internal/dataentry/document.go`:
   - Replace the `isFormRoute(base) { ... } else { passthrough }` branching
with a single flow that follows the decision table above.
   - `stripQueryKey` already handles removing pre-existing `return_to`. Call
it on every internal path, whether or not we're injecting.
   - Extract a helper `rewriteInternalLink(base, existingQuery, fragment, returnPath, occ)`
that covers both form and non-form internal paths; form-route special case is
"also emit anchor id."

9. **Server isSafeReturnPath fix** in `internal/dataentry/return_path.go:29`:
   ```go
   if strings.HasPrefix(s, "//") || strings.HasPrefix(s, `/\`) ||
       strings.HasPrefix(strings.ToLower(s), "/%5c") ||
       strings.HasPrefix(strings.ToLower(s), "/%2f") {
       return ""
   }
   ```
Or simpler: lowercase `s` once for the prefix comparison; the returned path
retains its original casing. Table test extends the existing
`return_path_test.go` with the lowercase cases.

**Alternatives considered.**

- *Single mechanism (`?from=` absorbs `?return_to=`)*: rejected — `?from=` is
a symbolic list id, can't express arbitrary paths.
- *Single mechanism (`?return_to=` absorbs `?from=`)*: rejected by user
("keep scope navigation separate"). Minor duplication, clearer mental model.
- *App-global banner in `App.vue`*: rejected — Back button far from content
it's backing away from; poor discoverability.
- *Status-bar pill*: rejected — always-reachable but easy to miss.
- *DynamicForm routed through composable*: rejected (RR-5K8I2) — square peg;
composable is for rendering, DynamicForm does post-submit redirect. Shared
primitive (`readReturnTo`) already exists.

**Files to modify.**

Go:
- `internal/dataentry/document.go` — rewriter follows decision table.
- `internal/dataentry/document_test.go` — new test cases: non-form internal
paths, idempotency (single- and multi-pass), cache invariance.
- `internal/dataentry/return_path.go` — case-fold prefix check (AC8).
- `internal/dataentry/return_path_test.go` — lowercase-encoded payload cases.
- `internal/dataentry/api_v1_test.go` — cache-invariance integration test
(AC9).

Frontend new files:
- `frontend/src/composables/useBackTarget.ts`
- `frontend/src/composables/useBackTarget.test.ts`
- `frontend/src/components/common/BackButton.vue`
- `frontend/src/components/common/BackButton.test.ts`
- `frontend/src/styles/back-button.css`

Frontend modified:
- `frontend/src/components/entity/EntityDetail.vue` — swap scope-nav Back.
- `frontend/src/views/CustomView.vue` — swap scope-nav Back.
- `frontend/src/views/DocumentView.vue` — replace bespoke goBack(); delete fromList computed.
- `frontend/src/views/ListView.vue` — add Back button.
- `frontend/src/views/KanbanView.vue` — add Back button.
- `frontend/src/views/AnalyzeView.vue` — add Back button.
- `frontend/src/views/SearchView.vue` — add Back button (separate from filter-picker `.back-btn`).
- `frontend/src/composables/useScopeNavigation.ts` — drop `backUrl` field from ScopeNav.
- `frontend/src/main.ts` — import `styles/back-button.css`.

Frontend test modified:
- `frontend/src/composables/useScopeNavigation.test.ts` — update for dropped `backUrl` field; add test asserting `navigateScope` preserves `return_to` (RR-97NAZ).

E2E:
- `e2e/document-links-roundtrip.spec.ts` — extend with AC7 scenario.

Documentation:
- `docs/data-entry.md` + `docs-project/entities/guides/GUIDE-data-entry.md`
— update "Links in rendered documents" to note internal non-form links now carry
`return_to`, and every receiving view renders a Back button.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input sources & validation.**

1. `?return_to=<path>` from the URL bar. Validated via `isSafeReturnPath`
on **both** server and client. Fix (AC8): both sides use case-folded prefix
checks for `/%5C` / `/%5c` / `/%2F` / `/%2f`. Invalid → fall through to `?from=`
or no button.

2. `?from=<list-id>` from the URL bar. Validated via schema lookup; unknown
ids still produce `/list/<id>` (backend 404s on next nav). No regression.

3. Document author-supplied `return_to` in hrefs. Rewriter strips any
pre-existing `return_to=` on internal paths (both form and non-form, whether or
not it has a replacement). Author can't inject a hostile `return_to` that
reaches the user's URL bar via the rewriter.

**Security-sensitive operations.**

- Navigating via `router.push(safe)` — `safe` is always the output of
`isSafeReturnPath`, so it's a same-origin path. No external-URL risk.
- Rewriter appends `return_to` to internal paths only. External paths
untouched. No new exposure.
- Disk cache (`.rela/documents/<entry>-<hash>.html`) must NOT contain
`return_to` tokens. Invariant: rewriter runs AFTER `GetCached`, not inside
`doRender`. Enforced by AC9 and by comment + test in document.go (RR-3Y6BM).

**Threat model.**

- *Open redirect via planted `?return_to=//evil.com`*: rejected by
case-folded guard on both sides. No Back button renders.
- *Stale return_to served from poisoned cache*: rendered impossible by
AC9's test harness (disk file must not contain the token).
- *Author-planted return_to in document href*: stripped by the rewriter
regardless of path class (see decision table).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test scenarios.**

- AC1 (rewriter, non-form internal): Go table-driven. Cases:
`/entity/ticket/TKT-001` → `/entity/ticket/TKT-001?return_to=/doc`, `/list/all`
→ `/list/all?return_to=/doc`, with + without pre-existing query.
- AC2 (composable precedence): vitest. Query shapes: `?return_to=/A`,
`?return_to=/A&from=B`, `?from=B`, `?`, `?return_to=//evil.com&from=all`. Assert
`{ to, labelHint }` or null.
- AC3 (EntityView + CustomView): vitest mount with `?from=all_tickets`;
BackButton renders with label `← All Tickets` and href `/list/all_tickets`.
Mount with `?return_to=/document/…`; href is the document path.
- AC3 + RR-97NAZ (Prev/Next preserves return_to): vitest on
`useScopeNavigation.navigateScope`; URL before has both `from` and `return_to`;
assert the push call preserves both keys.
- AC4 (DocumentView): vitest mount; Back button replaces old bespoke handler.
- AC5 (views without prior Back): vitest mount each of ListView / KanbanView /
AnalyzeView / SearchView with `?return_to=`; Back renders. With no query; no
Back.
- AC6 (DynamicForm regression): existing unit + e2e pass unchanged.
- AC7 (E2E behavioural, RR-27ZFC): extend `document-links-roundtrip.spec.ts`.
- AC8 (server case-fold): Go table test for `isSafeReturnPath`:
`/%5c`, `/%5C`, `/%2f`, `/%2F` all rejected; `/ok` accepted.
- AC9 (cache invariance): Go integration test in `api_v1_test.go`:
render doc w/ returnPath=`/A`, assert HTML contains `return_to=%2FA`; render
again w/ returnPath=`/B`, assert HTML contains `return_to=%2FB`; read the cache
file from disk; assert `!strings.Contains(string(file), "return_to")`.
- AC10 (rewriter idempotency): Go table-test pairs:
  - `Rewrite(html, "/A")` twice → byte-equal.
  - `Rewrite(html, "/A")` then `Rewrite(result, "/B")` → only `/B` present.

**Edge cases.**

- Empty `return_to=`: `isSafeReturnPath("")` → "" → composable null → no button.
- Array-valued `return_to`: `readReturnTo` returns null (existing).
- `?from=` with unknown list id: BackButton falls back to `← Back` label;
navigation goes to `/list/<unknown>` (backend 404). Same as today.
- Rewriter on pre-existing `return_to=` (both forms + non-form, both
returnPath empty and set): stripped, re-injected per decision table.
- Goldmark `&amp;` encoding in pre-existing query: existing `stripQueryKey`
handles both `&` and `&amp;`; test covers both.
- `return_to` pointing at same route (cycle): user clicks Back, lands on
same page. Harmless; `?return_to=` removed on arrival (gone from URL).
- Unicode / very-long `return_to`: guarded by `isSafeReturnPath`, URL-encoded
on emit, browsers accept ~2000 chars.

**Negative tests.**

- `?return_to=javascript:alert(1)` → null.
- `?return_to=//evil.com` → null.
- `?return_to=/\\evil.com` → null.
- `?return_to=/%5Cevil.com` (upper) → null (existing).
- `?return_to=/%5cevil.com` (lower) → null (AC8 new).
- `?return_to=/%2fevil.com` (lower) → null (AC8 new).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks.**

| Risk | Impact | Mitigation |
|------|--------|------------|
| Moving `.scope-nav-btn` style out of EntityDetail breaks visual parity | Medium | Keep class name identical; visual smoke test before review |
| Multiple views add Back button in inconsistent positions | Low | One pattern: inside `<header class="page-header">`; document in frontend/CLAUDE.md |
| Rewriter change breaks existing e2e (document-links-roundtrip) | Low | Form links keep prior behaviour (anchor id + return_to). Non-form additions extend behaviour, don't break it. Manual verification + idempotency tests. |
| useBackTarget `computed` not invalidating when route.query mutates | Low | Vue-router's `useRoute()` returns a reactive object; `computed` picking `route.query.return_to` auto-invalidates. Unit test covers it. |
| Case-fold fix changes server behaviour on paths that happened to slip through | Very low — these were never valid inputs | Document in commit message; the guard's contract never admitted these values anyway. |

**Effort: m** (medium). Concrete line estimate: ~180 new (includes RR-fixes),
~100 modified.

## Documentation Planning

- [x] User-facing docs identified
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: single-file doc touch; evidence captured in review-checklist REV-NZMA9)

**Documentation impact.**
- [x] User guide / reference docs — `docs/data-entry.md` + `docs-project/…/GUIDE-data-entry.md` "Links in rendered documents" section.
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] CLAUDE.md — added `BackButton` / `useBackTarget` / `styles/` row to `frontend/CLAUDE.md` package-layout table.
- [x] ~~README.md~~ (N/A: no project-level changes)
- [x] ~~API docs~~ (N/A: no API surface changes)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| Finding ID | Severity | Status | Resolution |
|------------|----------|--------|------------|
| RR-3Y6BM | critical | addressed | AC9 + explicit invariant in Security + rewriter comment |
| RR-IFA9K | critical | addressed | AC8 + server guard fix in scope + table test |
| RR-AN2J8 | significant | addressed | Decision table; always strip pre-existing return_to |
| RR-RFYUI | significant | addressed | AC10 + two test cases |
| RR-RV4LA | significant | addressed | Composable returns `{to, labelHint}`; BackButton resolves label |
| RR-27ZFC | significant | addressed | AC7 rewritten as behavioural (click + waitForURL) |
| RR-LJ8IB | significant | addressed | 2x4 decision table in Approach |
| RR-AY8HG | significant | addressed | Single `.scope-nav-btn` class; styles in `src/styles/back-button.css`, not App.vue |
| RR-5K8I2 | minor | addressed | Dropped DynamicForm refactor from scope |
| RR-97NAZ | minor | addressed | AC3 amended + explicit scope-nav unit test |
