---
id: DOCS-IL5IUH
type: docs-checklist
title: 'Docs: plantuml_server_url cleartext rule'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — `isLoopbackHost` documents why the name match is exact (a prefix/suffix test accepts `localhost.evil.com`) and why a name resolving to 127.0.0.1 is still treated as remote
- [x] ~~Function/type docs if public API~~ (`validateApp` and `isLoopbackHost` are both unexported; their godoc was still extended)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (README does not cover data-entry.yaml keys)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (no new pattern; extends the existing config-validation shape in the same function)
- [x] ~~Help text accurate (if CLI changes)~~ (no CLI surface changed; `rela validate` gains one more message from the existing validator)

## External Documentation

- [x] ~~Changelog entry added~~ (repo has no CHANGELOG.md)
- [x] API docs updated (if applicable) — `docs/data-entry.md` App section now documents `plantuml_server_url` (previously undocumented entirely) including the http/loopback rule; edited at source in `docs-project/` and regenerated with `just docs`
