---
id: IMPL-92ILKV
type: implementation-checklist
title: 'Implementation: Ctrl/Cmd-click (and middle-click) should open data-entry rows and cards in a new browser tab'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Ten surfaces converted from `router.push`-on-a-`<div>` to real links. Nine use
`RouterLink` directly and need no click handler at all; only the list `<tr>`
(which HTML forbids being an anchor) keeps a handler, guarded by
`shouldDeferToBrowser`.

New: `frontend/src/utils/openIntent.ts` (`shouldDeferToBrowser`,
`safeInternalHref`) + 18 unit tests.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The central test derives BOTH sides from the component rather than asserting a
literal: it reads the rendered href, triggers the click, and asserts the href
equals the pushed target's serialized path+query. A new query param cannot make
the two diverge without failing.

**Mutation-verified** (both reverted after):

1. Replaced `:to="entityTarget(entity)"` with a bare `/entity/${type}/${id}` —
the two scope tests failed, exactly the RR-6PBTF1 regression.
2. Removed the `shouldDeferToBrowser` guard — all five modifier-click tests
failed.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Driven in a real Chromium against a live `rela-server`:

```
ROW HREF:  /entity/feature/FEAT-001?from=features&scope=list:features
POPUP URL: http://localhost:50269/entity/feature/FEAT-001?from=features&scope=list:features
ORIGINAL UNCHANGED: true
```

- **AC1/AC2** cmd-click and middle-click each opened a real browser tab
(`context.waitForEvent('page')`), and the originating tab did not navigate.
- **AC7** the popup URL matches the href character-for-character INCLUDING
`from=` and `scope=` — the divergence RR-6PBTF1 warned about does not occur.
- **AC4** `a.row-link` is present with a resolved href.
- **AC5** plain click still routes in-SPA (`list.spec.ts:187` still green).
- **AC6** screenshot review: rows render identically to before — no link
colour, no underline, delete buttons and checkboxes unaffected.

Screenshot confirmed the visual result is unchanged; the links are invisible to
the eye and real to the browser, which is the intent.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities

Followed `DashboardView.vue:263-272`, the existing in-repo precedent for
navigating table cells via `<router-link>`. Each surface exposes one
`entityTarget()` used by both the link and any push, rather than duplicating
route construction.

**Full results:** 2059 frontend unit tests pass (127 files), 270 e2e pass / 8
skipped / 0 fail, `npm run typecheck` clean, `npx eslint src/` 0 errors, `just
arch-lint` OK, `go build ./...` clean.

**One shared-fixture fix:** `frontend/src/test/setup.ts` globally stubbed
`RouterLink` as `<a><slot/></a>`, silently discarding `to`. Every href assertion
would have been vacuous against an anchor-shaped element with no href. The stub
now serializes `to` (string or path+query object) into a real `href`.

## Documentation

- [x] `docs/data-entry.md` — new "Opening things in a new tab" section, incl.
the accepted text-selection trade-off on list rows.
- [x] ~~docs/metamodel.md, docs/cli-reference.md, README.md~~ (N/A: no
metamodel, command or project-level surface change)
