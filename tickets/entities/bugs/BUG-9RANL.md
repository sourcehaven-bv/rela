---
id: BUG-9RANL
type: bug
title: Playwright force:true click on disabled GFM checkbox doesn't fire Vue click handler in e2e
description: Playwright's force:true click on the disabled GFM checkbox does not reliably fire the Vue-installed click handler, so e2e can't drive the markdown-checkbox toggle flow via a plain .click() call. Product behaviour works for real users; the gap is test-harness-only.
priority: low
status: backlog
---

In e2e/tests/checkboxes.spec.ts, the "clicking a checkbox persists the toggle on
the server" test is skipped because Playwright's `force: true` click on a
disabled `<input type="checkbox" data-cb-idx="0">` doesn't reliably fire the
Vue-installed click handler under test. The behaviour is a potential
product-regression surface (if the handler ever breaks, users can't toggle
checkboxes) but reproducing it through Playwright needs a dispatched InputEvent
or a real user-gesture click.

Repro: unskip the test, run `npm test -- checkboxes.spec.ts`. Expected: first
checkbox toggles and the server sees the updated content. Actual: API poll times
out.

Scope: test-infra, not a product bug. Add a harness that dispatches a native
MouseEvent on the checkbox (or drives via keyboard Space key after focus).

See TKT-4Q2VI review-response RR-OK9RH.
