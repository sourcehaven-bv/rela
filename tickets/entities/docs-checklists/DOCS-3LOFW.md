---
id: DOCS-3LOFW
type: docs-checklist
title: 'Docs: Operator ''purge version'' primitive (TKT-BW6UUL)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Godoc on the purge store capability (`purge.go` package doc + method docs, all guardrails documented)
- [x] `VersionOpPurge` tombstone documented in store.go
- [x] plimsoll directive comment updated on pgstore.Store

## Project documentation

- [x] CLAUDE.md postgres section: version-purge bullet (guardrails, capabilities, necessary-not-sufficient)

## External / user-facing documentation

- [x] `docs/postgres-backend.md` (via docs-project source): "Purging history for compliance" section — dry-run default, live-first, --force-live tombstone, rename refusal, necessary-not-sufficient
- [x] `docs/cli-reference.md` (via docs-project source): `rela history-purge` + `rela relation-history-purge` command reference
- [x] Docs regenerate clean (Docs CI check passes)

**Note:** User-facing docs authored directly in the docs-project source entities
(same convention as TKT-9INY0Y / TKT-92JL8P); no separate authoring task needed.
