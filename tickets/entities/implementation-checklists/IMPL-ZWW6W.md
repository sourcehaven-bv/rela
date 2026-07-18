---
id: IMPL-ZWW6W
type: implementation-checklist
title: 'Implementation: Re-verify relation-rename versioning against the atomic store.RenameEntity path (post #1127)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Rewrote stale `TestRelationVersionRenameStitchesLineage` → `TestRelationVersionRenameAtomicPath` (drives real `store.RenameEntity`)
- [x] Added `TestRelationRenameDoesNotBumpUpdatedAt` pinning the no-updated_at-bump fact
- [x] Documented sync-only-best-effort decision (manager.go comment, relation_version.go godoc, CLAUDE.md)

## Quality

- [x] Tests pass (`go test -tags postgres ./internal/store/pgstore/` + entitymanager)
- [x] Lint clean (default build, changed pkgs)
- [x] `go vet -tags postgres` clean
- [x] ~~New user-facing behavior~~ (N/A: test + doc only, no behavior change)
