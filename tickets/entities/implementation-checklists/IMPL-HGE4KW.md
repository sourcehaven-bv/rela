---
id: IMPL-HGE4KW
type: implementation-checklist
title: 'Implementation: Deleted-relation history id-reuse disambiguation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] `ListRelationLifetimes` enumerates every stitched-history head of a reused key (newest-first, claimed-set dedup)
- [x] `RelationHistoryQuery{RecordID}` selector on the reader methods; store-side membership validation as the auth boundary
- [x] Purge compliance fix: multi-lifetime key without a selector is refused; `--all-lifetimes` erases every lifetime; store-side RecordID/AllLifetimes mutual exclusion
- [x] CLI `--list-lifetimes` / `--lifetime` (history, restore) and `--lifetime` / `--all-lifetimes` (purge) + footer
- [x] HTTP `_lifetimes` route (auth-gated) + `?record_id=` (400 on malformed)

## Quality

- [x] Tests pass (`go test -tags postgres ./internal/store/pgstore/` + dataentry + cli)
- [x] Lint clean (default build), `go vet` both tags clean
- [x] plimsoll + arch-lint clean (versioning already extracted to VersionStore in the prep PR)
