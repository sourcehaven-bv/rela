---
id: BUG-ZL4K7QW
type: bug
title: Session screen and confirms behind dialogs, 401 noise, tenant 404, action confirm colour
description: The "session ended" screen and a confirm raised from a dialog rendered behind that dialog; after the session ended, error toasts and list errors repeated "Request failed (401)"; a single-tenant server answered GET /api/v1/_tenant with 404 on every load; and the confirm of a style danger action used the primary button.
priority: high
effort: m
why1: main.ts imports rl.css, which rebased the library's z tokens, before tokens.css, which repeats the library's values; all are unlayered :root blocks, so the later library values won (--rl-z-modal 100, so a confirm sat under a 900 dialog and the session screen at 301). The 401 errors reached the toasts and the views' error blocks like any failure; the SPA read /_tenant on every server; the action confirm calls passed no danger flag.
why2: tokens.css became a generated copy of the library palette, including its z tokens, after the rebase was written; the stacking test read rl.css alone, so it kept passing.
why3: The test checked one file's contents, not what the bundle resolves in import order.
why4: The session screen (BUG-9DR9867) was added as one more overlay without checking its stacking against dialogs and confirms in a browser, and without deciding what the views show under it.
why5: Cross-file cascade facts (import order, generated files) had no test that models the bundle.
prevention: rlStacking.test.ts resolves the --rl-z tokens across every stylesheet main.ts imports, in order, and asserts session screen > confirm > dialog; it fails on the old import order. Unit tests pin the suppressed toasts and error blocks, the single-tenant status route and the danger confirm.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in a browser test of a SaaS setup built on rela.

- "Your session has ended / Sign in again" (z 301) and "Discard new note?"
  (z 100) rendered behind the New Note dialog (z 900).
- After the session ended, a "Request failed (401)" toast and the list's
  "Request failed (401) / Retry" showed with the session dialog.
- A single-tenant server answered `GET /api/v1/_tenant` with 404 on each
  load, and the browser logged a red console error.
- The confirm dialog of an action with `style: danger` used the primary
  blue button.

## Acceptance Criteria

- [x] The overlay order is session screen > confirm > dialog, as the bundle resolves it, and a test guards it.
- [x] While the session or workspace screen is up, no error or warning toast shows, and lists, entity pages, boards, calendars, flyouts and side panels show no error or Retry under it.
- [x] A single-tenant server answers `GET /api/v1/_tenant` with 200 `{"status":"active"}`; the multi-tenant answer is unchanged.
- [x] The confirm of a `style: danger` action uses the danger style on the entity page, in lists, toolbars, batch runs and piles.
- [x] The behaviour is documented.
