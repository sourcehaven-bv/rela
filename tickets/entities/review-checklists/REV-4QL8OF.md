---
id: REV-4QL8OF
type: review-checklist
title: 'Review: View section fields render as display by default; opt in to inline edit with `render: input`'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go build ./...` — pass
- [x] `go test ./...` — pass, exit 0, 80 packages
- [x] `golangci-lint run ./...` — **0 issues** (full repo)
- [x] `just arch-lint` — OK, no warnings
- [x] `just coverage-check` — pass, **77.3%** total (up from 76.9%), all package floors satisfied
- [x] `npm run test:run` — 1488 passed / 90 files
- [x] `npm run typecheck` — pass
- [x] `npm run lint` — 0 errors (90 pre-existing warnings in untouched files)
- [x] `npx playwright test` — **235 passed, 0 failed** (up from 231), 8 skipped (postgres-gated)

## Code Review

- [x] `/code-review` run by the cranky-code-reviewer agent against the full diff
- [x] All critical findings addressed
- [x] All significant findings addressed or explicitly declined with reason

**Findings: 6 total.**

| ID | Severity | Status |
|----|----------|--------|
| RR-GLK4UY | critical | fixed |
| RR-5KFD7W | significant | fixed |
| RR-TIUKMA | significant | fixed |
| RR-H39SEJ | significant | wont-fix (premise incorrect — no changelog surface exists in this repo) |
| RR-VBJ91V | minor | addressed (documented) |
| RR-1SNYI1 | nit | addressed |

**RR-GLK4UY (critical)** is the one that mattered: the render default promoted a
previously-unreachable stale-display-value read from an edge case to the common
path. `mapFieldsToProperties` read the server string mirror, which is never
updated after a PATCH. Fixed with `entryDisplayValue()`, mirroring the existing
`rowDisplayValue` (RR-FC1C) rather than inventing a second mechanism. I verified
the mechanism in source before accepting the finding.

The reviewer independently confirmed the security core sound: it traced every
path that can put a widget in `mode="edit"` and found the conjunction complete,
`RR-PGGRBD` correctly honoured, the four wire-conversion sites present, both Go
builders correct, validation genuinely unconditional, and the wire types
field-for-field identical. It also endorsed the `sectionShouldRouteToInlineEdit`
deviation after checking all four routing cases.

## Verification

- [x] Feature verified end-to-end against a running server
- [x] Acceptance criteria met
- [x] No regressions (full Go + frontend + e2e suites green)

Live-server verification: configured views emit `render: 'input'`; synthesized
default views emit `'display'`; the inert-render warning fires with the exact
expected message.

4 new e2e specs (`view-section-render-mode.spec.ts`) cover the mixed-render page
the plan identified as uncoverable by unit tests, plus a regression guard for
RR-GLK4UY.

**Not done:** browser-level *visual* confirmation — the Chrome extension was not
connected in this environment. Rendering is asserted structurally (no
`select`/`input`/`.form-field` in a display section; an enabled control in an
opted-in one) but no human has looked at the page. Recommend a glance via `just
dev` before merge.

## PR

- [x] PR created and CI green — see the PR link recorded below

**PR:** opened from branch `feat/view-section-render-mode-hoix1` after a clean
local `just ci`. CI monitored to green before merge; any failure is fixed on
this branch and re-pushed rather than merged around.
