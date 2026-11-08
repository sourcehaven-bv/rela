---
id: DOCS-MMPGWJD
type: docs-checklist
title: 'Docs: Range fingerprints for types with redacted fields'
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing surface yet; the endpoint is step 5)
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PR titles)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API changes; POST /api/v1/_reconcile is step 5)

Notes: package docs updated in internal/reconcile (where completion happens, the
roll-up name ETag exception, RR-3HN68CI decision),
internal/store/elementrange.go (callers must not depend on Pending),
internal/visibility/policyreader.go (body-redaction TODO lists Elements), and
the storage rule in .claude/rules/storage.md (visibility.Elements as the one
gated caller).
