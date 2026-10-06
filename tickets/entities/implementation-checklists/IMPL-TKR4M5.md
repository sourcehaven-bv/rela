---
id: IMPL-TKR4M5
type: implementation-checklist
title: 'Implementation: Body inline edit: start editing from a sticky pencil button, not from a click on the text'
started: "2026-10-05"
completed: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (story play tests `ExplicitTrigger`, `LongContent` in RlInlineEdit.stories.ts; both fail on the old component)
- [x] Integration tests written (test full flow, not just units) (e2e/tests/body-edit-button.spec.ts against rela-server)
- [x] Happy path implemented
- [x] Edge cases from planning handled (read-only and empty body use unchanged branches; short body keeps the button at the top-right as before)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error paths; pure UI interaction change)

## Test Quality

- [x] Using fixture builders or factories for test data (api.createEntity fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- AC1: e2e clicks, double-clicks and triple-clicks a paragraph; selection is non-empty / contains the paragraph; no ProseMirror editor mounts. Story test: click + dblclick on prose emits no edit.
- AC2: e2e and story: the "Body, edit" / "Description, edit" button opens the editor.
- AC3: e2e scrolls the main pane to the middle of a 40-paragraph body; the button lies inside both viewport and body. Story `LongContent` asserts the same in a 240px scroller. Screenshots at 1280x720 and 390x800 (hover) show the pencil at the top-right of the visible body; on mobile it sits clear of the menu button because the pane's top padding already offsets sticky children.
- AC4: story test: Escape from the editor returns focus to the edit button.
- AC5: e2e checkboxes.spec.ts and comments.spec.ts pass (15 tests).
- Finding during verification: the planned 64px mobile offset was wrong. The entity detail page has no sticky topbar (only AnalyzeView uses PageLayout), so the override and the CSS variable were removed.

## Quality

- [x] Code follows project patterns (check similar code) (sticky as in milkdownEditor.css toolbar; page-object helpers in EntityPage)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no error paths)
- [x] No debug code left behind (temporary debug/screenshot specs deleted)
