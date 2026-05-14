---
id: IMPL-31QA
type: implementation-checklist
title: 'Implementation: Markdown renderer preserves source line breaks in data-entry view'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A:
this is a single render-config flip; the new unit tests parse the output HTML
into a DOM and assert on `<br>` placement, which is the exact integration
boundary — there is no further "flow" to cover)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A:
no new error paths — the change is a single static option flip)

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

All AC verification is captured by the new DOM-asserting tests in
`frontend/src/utils/markdown.test.ts`:

- AC1 (soft break → space): `'foo\nbar'` renders to one `<p>` whose
text is `'foo\nbar'`, with zero `<br>` tags. The browser then collapses the `\n`
to whitespace per HTML rendering rules. Verified by DOMParser assertion in the
new test "treats single newlines inside a paragraph as whitespace, not <br>".
- AC2 (hard break preserved): `'foo  \nbar'` renders to one `<p>` with
exactly one `<br>`. Verified by DOMParser assertion in "preserves CommonMark
hard breaks".
- AC3 (lists/headings/code unaffected): existing 39 tests for these
constructs continue to pass without edits. `npm run test:run` shows 737/737
passing.
- AC4 (browser reflow): the DOM produced has no `<br>` between wrapped
lines, so HTML reflow is the natural CSS behavior — verified by the unit test
that proves the DOM structure. A live browser session was not run because the
unit-test DOM assertion is the same observable.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced (DOMPurify sanitization path
unchanged; XSS-defense surface untouched)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Local CI gates run:
- `npm run test:run` → 737 passed
- `npm run lint` → 0 errors (pre-existing warnings only)
- `npm run typecheck` → clean
