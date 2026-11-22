---
id: IMPL-K9MW6VD
type: implementation-checklist
title: 'Implementation: Portal UX fixes: date picker and mobile menu bar and select-all label and phone list columns'
started: "2026-11-20"
completed: "2026-11-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Local copy of harbourmaster on rela-server-sqlite
(as_of capability on).
- 375px dashboard with a next-action card: hamburger bar above the card, no
overlap; after scrolling the button sits in the topbar's left slot.
- 375px entity page: content scrolls under an opaque bar (0-60px).
- 375px invoices list with `primary: true` on total and pdf_url: number,
total and PDF columns shown; select-all checkbox named "Select all".
- 1400px backoffice: "Back office" not truncated; "Now" on its own row.
- Integration: sidebar handler test for `hidden_builtins` with as_of;
existing mobile-layout and space-landing-builtins e2e specs run.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
