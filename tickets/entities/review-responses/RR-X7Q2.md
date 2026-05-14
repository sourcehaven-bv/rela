---
id: RR-X7Q2
type: review-response
title: Plan promises a unit test for MarkdownEditor but no test file exists today
finding: 'Plan lists ''Edit: frontend/src/components/forms/MarkdownEditor.test.ts (if it exists; otherwise add one).'' The codebase doesn''t have one. EasyMDE/CodeMirror are notoriously hard to test in JSDOM (CodeMirror v5 measures DOM dimensions at init). The plan should either (a) commit to mocking EasyMDE wholesale in the test, OR (b) drop the MarkdownEditor-level unit test and rely on the helper unit test + e2e for behavior coverage. Implementation will otherwise spend a day fighting JSDOM.'
severity: nit
resolution: 'Plan §Files: dropped MarkdownEditor-level Vitest test. The helper unit tests + Playwright e2e cover the behavior end-to-end; JSDOM/CodeMirror v5 interaction is not worth the cost.'
status: addressed
---
