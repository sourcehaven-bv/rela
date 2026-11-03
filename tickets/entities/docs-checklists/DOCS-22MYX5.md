---
id: DOCS-22MYX5
type: docs-checklist
title: 'Docs: palette extension (TKT-ZKNWSK)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (dark mirror and restated light keys in ResolvePalette, -fg token clash, class grammar re-check in styleColors.ts, hairline on property dots)
- [x] Function/type docs if public API (ParseColor, Color, OperatorColor, ForegroundFor, Config.OperatorColors, DotColor, styleSwatch)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule; existing token sync rule covers the kit tokens)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; the stricter palette validation is called out in the PR body)
- [x] API docs updated (if applicable) (GUIDE-data-entry Styles section: color name forms, hues, shades, dark mirror, palette.colors, palette.dark.colors, validation; regenerated docs/data-entry.md)
