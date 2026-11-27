---
id: DOCS-YLR01ZO
type: docs-checklist
title: 'Documentation: in-memory config migrations on load'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not describe config migration)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule; the behaviour is documented on migration.MigrateBytes and ReviewRequired)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable)

GUIDE-cli-reference (`rela migrate` → When to use) now says the server and CLI
load deprecated syntax with a warning, lists what still refuses, and that `rela
validate` stays strict. `docs/cli-reference.md` regenerated with `just docs`.
