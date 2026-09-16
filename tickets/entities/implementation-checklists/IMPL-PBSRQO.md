---
id: IMPL-PBSRQO
type: implementation-checklist
title: 'Implementation: verify renderer security assumptions (Milkdown passthrough, flattenToLine)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the
change adds only tests; the Milkdown suite already mounts the real editor
component rather than a unit under a harness)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no
production code path added)

`rawHtmlPassthrough.test.ts` mounts the real `MilkdownEditor` with `attachTo:
document.body`, so the assertions observe the actual rendered DOM rather than a
schema description. `markdownLineEndings.test.ts` goes through `renderMarkdown`,
the same entry point the app uses, so DOMPurify is in the path too.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The Milkdown suite reuses the `mountEditor` shape from the neighbouring
`MilkdownEditor.test.ts`. The line-ending cases are `describe.each` tables over
code points built with `String.fromCharCode`, not literal characters: the exotic
ones are invisible in an editor, and a copy-paste normalising one into a plain
space would silently turn the test into a tautology.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. `<img src=x onerror=alert(1)>` creates no element — asserted by "does not
create an element for an inline `<img onerror>`"; the markup survives as visible
text. PASS.
2. Raw HTML round-trips unchanged — asserted through the editor's own
write-back guard, which also reports no spurious `update:modelValue`. PASS.
3. No survivor forges a heading in marked, and text is never dropped — 10
assertions across the five characters. PASS.
4. Every assertion checked against the bug it names — see the mutation check
below. PASS.

Mutation check (the tests are not vacuous), run against the installed dependency
and reverted after each:

- Patching the `commonmark` preset's `html` node `toDOM` to assign
`span.innerHTML = node.attrs.value` fails 3 of the 5 Milkdown tests (`<img
onerror>`, `<script>`, container subtree).
- Making `sanitizeLinkHref` return its argument unchanged (pre-7.21.3
behaviour, CVE-2026-57530) fails the `javascript:` test.
- The marked suite carries `\n` and `\r` positive controls asserting a heading
IS forged; they are what make the five negative cases evidence.

Suite results: `npm run test:run` → 165 files, 2671 tests pass. `npm run
typecheck` and `npm run lint` clean (0 errors). `go test ./internal/dataentry/
./internal/markdown/`, `gofmt`, and `just comment-lint` pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Each suite has one small local helper (`mountEditor`, `delivery`) matching the
file it sits beside. The two suites are deliberately NOT merged: they cover
different renderers on different paths and need different vitest environments
(happy-dom for the editor, jsdom for DOMPurify serialization, per BUG-SQSV6V).

The temporary dependency patches used for the mutation checks were reverted and
the suite re-run green; `node_modules` is untracked and nothing from it is
staged.
