---
id: IMPL-VUR6HM
type: implementation-checklist
title: 'Implementation: Style HTML comments as muted chips in the Milkdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

20 tests in `commentNode.test.ts`: unit tests on the `commentBody` matcher, and
integration tests that mount the real `MilkdownEditor` and assert on the
rendered `.ProseMirror` DOM plus the `guardedValue()` write-back path.

Every edge case from planning is covered: empty comment, multi-line, padded,
non-string value, two-on-one-line, unterminated, comment-followed-by-markup.

There are no error paths to swallow — the matcher returns null for anything it
does not claim, and null means "fall through to the preset's html node", which
is the pre-existing safe behaviour rather than a failure.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`mountEditor` and `guarded` are the shared helpers; round-trip assertions
compare against the `src` string the test itself defined rather than a repeated
literal. Table-driven `it.each` for the matcher's rejection cases.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against `prototypes/data-entry/project` with a scratch entity
carrying all comment shapes, and opened `/form/edit_ticket/TKT-901` in a real
browser. Screenshot confirmed:

- AC1 — standalone comment renders as a muted italic chip, no `<!--` or `-->`
visible.
- AC2 — `**Research Doc:** <!-- ... -->` stays on one line, chip inline after
the bold run.
- AC3 — the multi-line `Options` scaffold renders as a full-width dashed block
with interior newlines preserved.
- AC4 — `<img src=x onerror="alert(1)">` still renders as visible raw text; no
element created, no handler fired.
- AC5 — see below.
- AC6 — chip is `contenteditable="false"` with a single TEXT_NODE child.

**This is where the one real defect was found.** The browser showed `<!-- a -->
text <!-- b -->` rendering as TWO chips, while AC5 asserted zero and a test
agreed. Probing remark-parse showed why: remark splits that line into two
separate inline `html` nodes, each already a well-formed comment, so the matcher
never receives the combined string. Both chipping is correct CommonMark; the
test had passed for the wrong reason. AC5 was rewritten, the test replaced with
one asserting two chips, and a round-trip test added. The `<!-- note --><div>`
half of AC5 was valid and still holds.

Worth recording: a green suite asserted the opposite of what the editor did.
Only running the app surfaced it.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Modelled on `entityRefNode.ts` (atomic node, `parseMarkdown`/`toMarkdown` pair).
CSS duplicated per shell prefix in both stylesheets, matching how `entity-ref`
is already handled — the two editors scope rules differently, so this is the
established pattern rather than avoidable duplication. Tokens come from
`scales.css` (`--space-*`, `--radius-*`), which `frontend/CLAUDE.md` explicitly
sanctions for editor chrome.

`unist-util-visit` was deliberately NOT imported (transitive-only dependency);
the four-line traversal is hand-rolled with the reason recorded in a comment.

Security: the label reaches the DOM as a ProseMirror text child, never
`innerHTML`, asserted by the "one indivisible leaf" test. The matcher is an
anchored allowlist, so unrecognised markup keeps its existing visible-text
behaviour.

Local checks: 2920 frontend tests pass (185 files), `vue-tsc --noEmit` clean,
`eslint` 0 errors (130 pre-existing warnings, all in `stress/`, none in the
changed files). Scratch entity and dev server removed after verification.

Docs: `frontend/CLAUDE.md` updated with the comment-node rules, as the plan
committed to.
