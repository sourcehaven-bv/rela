---
id: RR-10GMGV
type: review-response
title: Stylesheet error path was live in the test suite and asserted by nothing
finding: RR-V8P47K added a link error handler. The vitest run emits a real NetworkError for _rela-editor.css
  on every editor test, so happy-dom genuinely attempts and fails the load — the branch runs constantly
  with no assertion on it. The ambient console noise also masks any genuine future error in these tests.
severity: minor
resolution: Added a test that stubs console.error, dispatches error on the link, and asserts the message
  names the href.
status: addressed
---
