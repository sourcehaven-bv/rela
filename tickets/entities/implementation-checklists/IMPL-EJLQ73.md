---
id: IMPL-EJLQ73
type: implementation-checklist
title: 'Implementation: Milkdown editor renders GFM task lists as bullets with no checkbox'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

16 unit tests in `taskList.test.ts` mount the real editor; 2 e2e tests in
`checkboxes.spec.ts` drive a real browser. The three-state `checked` edge case
identified in planning is handled by a new `nodeWhere` probe kind and has its
own tests (an open and a done item must both read as pressed).

"Errors surfaced" is mostly N/A here — the code paths are a node view and a
ProseMirror command, neither of which does I/O. The command returns `false` when
it cannot apply, which is the ProseMirror contract the toolbar's dry-run
availability check reads; swallowing that would grey out the wrong buttons.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Unit tests use the `mountEditor` / `emitted` / `checkboxes` helpers rather than
repeating mount boilerplate, and the round-trip test asserts against its own
`src` variable instead of restating the markdown. E2E reuses the existing
`SEED.features.checkboxBody` fixture (one unchecked, one checked item) and the
`FormPage` page object; no new seed data. Repeated `beforeEach`/`afterEach`
blocks were hoisted to one top-level pair after `jscpd` flagged them.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Screenshot of the editor rendering `FEAT-004`'s body confirms two real
checkboxes, the second ticked, no stray bullets, labels on the checkbox row.

Non-vacuity was verified for every assertion, and the verification corrected two
wrong assumptions:

1. **The unit suite fails pre-fix**: 12 of 15 failed against the old editor.
The 3 passing were the negative cases (plain bullets render no checkbox, task
lists round-trip unchanged) — correct, since the bug never touched
serialization.
2. **The e2e pair fails with the node view disabled.** An earlier attempt
showed them passing, which briefly suggested the CSS was unnecessary. The cause
was a stale `bin/rela-server`: the server embeds the SPA bundle at compile time,
so `npm run build:e2e` alone does not change what the browser loads. After `go
build`, both fail as they should.
3. **Both CSS rules were then individually confirmed load-bearing.** Removing
`display: inline-block` puts the label underneath the checkbox (`content.x`
equals `checkbox.x`); removing the paragraph rule restores a 14px bottom margin
under every row. An assertion now covers each.

Undo was verified rather than assumed before documenting it: toggling gives `-
[x] todo`, Ctrl-Z restores `- [ ] todo`. That is now a permanent test.

Gates: 2703 frontend unit tests pass, `vue-tsc` clean, eslint 0 errors (127
pre-existing warnings unchanged), `just arch-lint` OK, prettier clean on all
changed files, 21 e2e tests pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: commands named by registered slice name (the module comment
in `editorCommands.ts` explains why `.key` is unusable), the new command's name
is covered by the existing `commandNamesExistInEditor` guard, and both
schema-probe guard tests were extended to recognise `nodeWhere` so the new probe
cannot escape their upstream-rename protection.

Security: the node view builds DOM with `createElement` and `textContent` only.
No `innerHTML`, so no new sink for entity-body content — the property
`milkdown-raw-html-no-dom-sink` exists to protect.

Styling deliberately stays in the shared `markdown-content.css`. The two rules
added to `milkdownEditor.css` cover only the node view's `contentDOM` wrapper,
which has no counterpart in the rendered HTML. No `[data-checked='true']`
styling was added: the rendered view does not dim completed items, and doing it
in the editor alone would recreate the very divergence this bug is about.

Debug code: the throwaway probe specs used during investigation were deleted;
`git status` shows only intended files.
