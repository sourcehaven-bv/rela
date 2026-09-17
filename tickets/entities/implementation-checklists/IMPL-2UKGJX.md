---
id: IMPL-2UKGJX
type: implementation-checklist
title: 'Implementation: Insert and edit external links in the Milkdown editor (plus horizontal rule and undo/redo)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Five new modules, split so the decisions are testable without an editor:
`linkUrl.ts` (the URL gate, pure), `linkSelection.ts` (which link the selection
is in, pure), `linkPaste.ts` (the `$prose` paste rule), `useLinkUI.ts` (dialog
and panel state), plus `LinkDialog.vue` and `LinkTooltip.vue`.

Two extractions to keep `MilkdownEditor.vue` from growing: `insertEntityRef.ts`
(which also removed a near-duplicate of the insertion routine) and
`mentionTrigger.ts`. The script block went 572 → 545 lines against a 500-line
warning; it was under before this ticket, so the net growth is ~45 lines rather
than the ~115 the feature would otherwise have added.

Errors surface: a refused URL shows a message in a live region with an error
ring and keeps the dialog open; a dropped `mailto:` query raises a toast,
because silently changing what someone typed is worse than refusing it.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Table-driven with `it.each` throughout. The obfuscation tests build their
characters with `String.fromCharCode` so the test file itself carries no
invisible characters — which the linter would otherwise reject, as it did once.

One test deserves naming: `linkUrl.test.ts` asserts the RETURNED value contains
no ignored characters, not merely that bad input is refused. A refusal-only
suite passes against the original (wrong) design, so that assertion is the one
carrying the finding.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against `prototypes/data-entry/project` with a Vite dev
server, and drove the real `create_ticket` form in a browser. (Two false starts
worth recording: the server refuses to boot until `just build-frontend` has run,
and a Vite dev origin needs `-allowed-origin` or every API call is a 403.)

Observed in the running app:

- All four new buttons present and in the right groups: `Bold, Italic,
Strikethrough, Inline code, Link, Heading 1-3, Bullet list, Numbered list, Task
list, Quote, Code block, Table, Divider, Insert entity reference, Undo, Redo`.
- Selecting "documentation" and pressing Link opened the dialog.
- `javascript:alert(1)` → refused, red error ring, message "Only http, https and
mailto links are allowed", dialog stayed open, document unchanged. The message
names the rule and does not echo the input.
- `example.com/guide` → accepted and normalized; the rendered anchor carried
`href="https://example.com/guide"` over the text "documentation".
- Caret inside the link → panel appeared (`data-show="true"`) showing
`https://example.com/guide` with Edit and Remove, and the toolbar grew its
Remove-link button.

One defect found and fixed this way, invisible to the unit tests because
happy-dom has no layout: the panel defaulted to `placement: 'top'`, so for a
link on the first line it rendered on top of the toolbar. Now `bottom-start`.

Automated: 343 tests in `src/components/forms/milkdown/`, 2799 across the
frontend, all passing. `npm run typecheck` clean; `eslint` clean apart from one
pre-existing warning in `activeFormats.ts` and the `max-lines` warning noted
above.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the house conventions: commands named by slice-name STRING, a
`BlockIcon` glyph keyed on the command id, floating surfaces built on the same
provider shape as the `@` menu, the dialog registered with `useModalStack` so
the global Escape handler stands down.

Undo/redo went through `EditorCommand` rather than a parallel channel, which is
what earns them the toolbar's `aria-disabled` handling — verified in the tests,
and the case that matters most, since their availability flips on every
transaction.

Two bugs were found and fixed during implementation, both real:

1. `LinkDialog` watched `props.open` with `flush: 'sync'`, which fires
mid-update, so it read the PREVIOUS `initialUrl` and opened empty whenever an
existing link was edited. Found by the retarget test.
2. `src/test/setup.ts` stubbed `ResizeObserver` as `vi.fn(() => ({...}))` — an
arrow function has no `[[Construct]]`, so any legitimate `new ResizeObserver()`
threw. floating-ui's `autoUpdate` does exactly that. Pre-existing, latent until
something in a test positioned a floating element.
