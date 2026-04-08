---
id: RR-JZKU
type: review-response
title: EntityList.test.ts doesn't exist — 'extend existing' is wrong
finding: Plan says 'tests for EntityList.vue (new or extend existing)'. The test file doesn't exist. Writing the first test for a 600-line component with stores and routing is non-trivial setup. Either explicitly task it in the impl checklist with effort, or drop component tests in favor of Playwright e2e.
severity: minor
reason: EntityList component test infra deferred. Coverage instead via composable unit tests (pure functions are easier to test) plus a Playwright e2e test that exercises the URL sync end-to-end.
status: deferred
---
