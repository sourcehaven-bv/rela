---
id: DOCS-M5CJG5
type: docs-checklist
title: 'Documentation: ACL doc-fields (rela-docs phase 1b)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `Policy.Description` and `RoleDef.Description` states the non-authz "documentation only" contract and cross-references phase 1a (`internal/acl/policy.go`)
- [x] ~~Inline comments for complex logic~~ (N/A: two struct-tag fields, no logic)

## Project Documentation

- [x] User-facing docs updated — `docs-project/entities/guides/GUIDE-acl-overview.md` (source) documents both optional `description` fields in the worked `acl.yaml` example + a paragraph stating they never affect an authorization decision; regenerated to `docs/acl-overview.md` via `just docs`
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern/convention)
- [x] Example populated — `prototypes/data-entry/project/acl.yaml` (the phase-2 generator corpus)

## External Documentation

- [x] ~~API reference~~ (N/A: no wire/API change — roles are not exposed over the API)
- [x] ~~CLI reference~~ (N/A: no command change)
