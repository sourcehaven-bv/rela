---
id: DOCS-32MJTNC
type: docs-checklist
title: 'Docs: validation rules enforced on write'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not cover validation)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing predicate, configedit and partition patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI flags changed; the mode is a config key and env variable)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog)
- [x] API docs updated (if applicable)

Guides updated: CLI reference ("Validation mode": config key, RELA_VALIDATION,
strict semantics and caveats) and metamodel ("Enforcing a rule on write"); docs/
regenerated with `just docs`.
