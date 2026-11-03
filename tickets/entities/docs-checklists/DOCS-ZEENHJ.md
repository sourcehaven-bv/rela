---
id: DOCS-ZEENHJ
type: docs-checklist
title: 'Docs: TKT-TXZZZ2'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: per-key inheritance, why only `style` applies to composer buttons, why the dialog stays red for cancelled or failed outcomes, the `confirm_text` invariant
- [x] Function/type docs if public API: godoc on `ButtonOptions`, `PlainYAML11Bool` and `MaxConfirmTextRunes`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` (preset `confirm` and `style`, `decision_button`, inheritance, load checks, web app behaviour) and `GUIDE-data-entry.md` (unquoted booleans refused, quote or tag `!!str`); examples pinned by docs example tests; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: display only; the server checks every post as before)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI gains the button options and `decision_button` schemas
