---
id: IMPL-ZWA3HE
type: implementation-checklist
title: 'Implementation: Clear the 4 open go/path-injection and 1 js/xss-through-dom CodeQL alerts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A:
no new flow — this hardens existing paths. The existing store conformance
suite, fsstore tests and the frontend suite already exercise the full flow and
all pass unchanged.)
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

**Verification Evidence:**

Mermaid rendering was verified against REAL mermaid in a REAL browser, not just
in the test environment — the whole risk here is a sanitizer that silently
strips diagram labels, which unit tests with mocked SVG cannot catch on their
own.

- Rendered flowchart, sequence and `stateDiagram-v2` through actual
  `mermaid.render()` with `securityLevel: 'strict'` in Chrome, then compared
  raw vs sanitized output. Confirmed mermaid puts flowchart and state labels in
  `<foreignObject>` (sequence uses SVG `<text>`), and that the naive
  `USE_PROFILES: {svg: true}` fix drops `Start` / `Choice` / `Done` / `Still` /
  `Moving` while leaving shapes and arrows — i.e. it ships as empty boxes.
- Confirmed the shipped config preserves every label and still strips
  handlers, `<script>`, `javascript:` URLs and embedded media.
- Confirmed with a hostile label (`<img src=x onerror=...>`) that strict
  mermaid emits no event handler but DOES emit a live `<img>`; the sanitizer
  removes it.
- Both documented load-bearing config choices (`ADD_TAGS: ['foreignObject']`,
  and omitting the `html` profile) were mutation-tested: each mutation fails 3
  tests. The Go guard test was mutation-tested the same way.

Note: happy-dom disagrees with real browsers on DOMPurify output. This test
file already opts into jsdom for that reason (BUG-SQSV6V), so the new tests run
on the accurate DOM.

Go side: full `go test ./...`, `just arch-lint`, `just plimsoll`,
`golangci-lint run ./...`, `just comment-lint` all pass. The root-is-`/`
containment case was found by an actual fsstore test failure, not by
inspection, and is now pinned by `TestContain_RootIsFilesystemRoot`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
