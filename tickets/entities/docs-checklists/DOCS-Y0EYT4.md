---
id: DOCS-Y0EYT4
type: docs-checklist
title: 'Docs: Owned items'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not list schema options)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; rules live in docs/metamodel.md and docs/acl-security.md)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog file; release notes come from PRs)
- [x] API docs updated (if applicable)

**Evidence:**

- `docs/metamodel.md`: `owning` property row and the "Owned Items (`owning:`)"
  section, including copies and refused restores.
- `docs/data-entry.md`: the `related` section display.
- `docs/data-entry/api-reference.md`: `_owner` and the `owning_rule` 422.
- `docs/acl-security.md`: the two residual one-bit signals.
- `rela analyze owning` has help text in `internal/cli/analyze.go`; `rela
  delete` reports owned entities.
- OpenAPI: `_owner` on the entity schemas (`internal/openapi/schemas.go`).
