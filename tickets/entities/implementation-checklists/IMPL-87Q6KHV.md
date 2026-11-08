---
id: IMPL-87Q6KHV
type: implementation-checklist
title: 'Implementation: Help texts for every form placement'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: optional display strings; no error paths added)

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

Ran `rela-server` on a copy of `prototypes/data-entry/project` with `hint`,
Markdown `help`, relation `help`/`hint`, form `description` and a two-step
wizard with step `description` and `details`. In the browser:

- `create_ticket`: the form description renders under the heading; the title
help renders its link; the info buttons next to "title" and "belongs to" carry
`aria-label` "Meer informatie over ..." and toggle a Markdown panel (bold, list)
between the label and the control; relation help renders emphasis.
- `wizard_ticket`: form description renders bold; step description renders
its link; "Meer informatie" is collapsed and opens to the Markdown details.

Integration: `TestFormHelpKeys_YAMLAndJSON` covers YAML to the JSON the SPA
reads; `DynamicForm.help.test.ts` mounts the whole form from config.

Sanitization is tested in `MarkdownHelp.test.ts` under jsdom. DOMPurify under
happy-dom (the default test environment) returned an `<img onerror>` and
`<script>` input unchanged, so a sanitization test there proves nothing.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (help rendering shared in `MarkdownHelp`, the toggle in `FieldHint`)
- [x] No security issues introduced
- [x] ~~No silent failures~~ (N/A: no error paths added)
- [x] No debug code left behind
