---
id: AM-form-cancel-stays-in-spa
type: automated-measure
title: 'Test: form Cancel resolves a route target instead of walking out of the SPA'
description: 'Pins BUG-ZE4354: Cancel must never leave the SPA. Unit arms cover return_to-first, the no-history fallback, and the unchanged back() path; the load-bearing E2E arm opens a create form directly in a fresh context (empty history) and asserts the app is still mounted on an in-app route — the existing suite only reaches forms via in-app links, which is why it never caught this.'
kind: test
location: frontend/src/components/forms/DynamicForm.cancel.test.ts + e2e/tests/forms.spec.ts ("Form cancel navigation")
status: active
---

Pins the fix for BUG-ZE4354: **Cancel must never leave the SPA.**

The defect was that `handleCancel` called `router.back()` unconditionally — a
browser-history operation with no route target. On a directly-opened form
(pasted URL, bookmark, new tab) there is no prior in-app entry, so back walked
out of the application.

## What the tests assert (shipped in #1326)

1. **Unit** — with `return_to` set, Cancel pushes that route (it is already
honoured by the submit path; the asymmetry with Cancel was the bug).
2. **Unit** — with no history and no `return_to`, Cancel pushes a fallback
route rather than calling `back()`.
3. **Unit** — with in-app history, Cancel still calls `back()` (no regression
for the common path).
4. **E2E** — open a create form **directly** in a fresh context, click Cancel,
and assert the app is still mounted on an in-app route.

The E2E arm is the load-bearing one. A unit test with a mocked router can be
made to pass while the real history stack still escapes the app, so the
regression guard has to exercise a real browser with an empty history.
