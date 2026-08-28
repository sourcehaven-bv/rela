---
id: DOCS-EY6Y95
type: docs-checklist
title: 'Docs: stop passing user-controlled IDs on the command line'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not document `documents:`)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; internal/dataentry/CLAUDE.md already mandates cmdexec for external commands — this change brings documents into line with it)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no CHANGELOG; release notes are generated from PR titles)
- [x] ~~API docs updated~~ (N/A: no HTTP API change; `command:` is data-entry.yaml config)

## What changed

- `docs/data-entry.md` — `command:` documented as an argv array with `{in}`;
  the `{id}`/`{id_lower}` table replaced; a migration note added covering all
  three breaks (array shape, removed placeholders, no working directory).
- `docs-project/entities/guides/GUIDE-data-entry.md` — same edits mirrored.
- Godoc on `DocumentConfig.Command`, `documentRenderConfig.Command`,
  `renderCommand`, `executeCommand`, `renderEntityMarkdown`, and the
  `projectRoot` field records the rationale where the next reader will be.
