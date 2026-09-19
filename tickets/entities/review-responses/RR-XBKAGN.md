---
id: RR-XBKAGN
type: review-response
title: No test coverage for the sandboxed app editor
finding: Grepping `relaComment` in src/app-editor/ returned only the import and the two `.use()` calls. Combined with the per-editor registration finding, the second editor — the one that can actually drift — was entirely unverified.
severity: significant
resolution: 'Added a `rela-editor comment chips` describe block to src/app-editor/relaEditor.test.ts with three tests against the real mounted element: a comment renders as a chip with no delimiters, a multi-line comment round-trips unchanged through the `.value` getter (the app''s save path, which reads through guardWriteBack), and non-comment raw HTML stays visible as text with no element created. 39 tests pass in that file.'
status: addressed
---

Worth stating why this matters more than ordinary coverage: the shared preset
removes the drift risk structurally, but a test that only ever runs the SPA
cannot observe the two editors disagreeing. The app-editor tests are what make
the structural fix checkable.

The raw-HTML test is deliberately duplicated from the SPA side rather than
shared: the app editor renders untrusted content inside a page an embedding app
controls, so a chip hiding live markup would be worse there, not better.
