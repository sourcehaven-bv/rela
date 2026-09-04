---
id: TKT-8FXP1B
type: ticket
title: 'Extract fsIndexLoader off fsstore.FSStore: index build/persist as a pure config→data value (84 → 72)'
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: ready
---

Sub-ticket of [[TKT-N0IKN9]], `fsstore.FSStore` arc. Follows the attachmentStore
extraction. Same shape as `fileLayout` (layout.go:41): a value type whose
methods are pure functions of construction-time config plus the index handed to
them.

## What moves (12 methods, all `internal/store/fsstore/index.go`)

`loadPersistedIndex` (:46), `savePersistedIndex` (:62), `syncIndex` (:97),
`syncEntities` (:119), `syncRelations` (:137), `rebuildPropCache` (:159),
`scanEntityDirs` (:176), `scanRelationDir` (:252), `newestEntityFileMtime`
(:313), `newestRelationFileMtime` (:328), `entitiesDirMtime` (:342),
`relationsDirMtime` (:369). `cacheKey` is touched by index.go only (4 sites) and
leaves FSStore.

```go
type fsIndexLoader struct {
    rooted   *storage.RootedFS
    layout   fileLayout
    codec    mdCodec
    cacheKey string
}
func (l fsIndexLoader) load() (entities map[string]entityMeta, order []string, relations …, propCache …, err error)
func (l fsIndexLoader) save(…) error
func (l fsIndexLoader) mtimes() (entities, relations time.Time)
```

(If the metaIndex ticket lands first, `load` returns a `*metaIndex` and `save`
takes one; otherwise return the raw maps and let FSStore assign. Either order
works — coordinate so the two PRs don't both rewrite `New`.)

## Watch

- `scanEntityDirs` spawns concurrent goroutines (index.go:198-235); keep the
fan-in exactly as is and run with `-race`.
- No lock on the new type (see the arc-wide locking rule on the
attachmentStore ticket). `New` calls `load` before the store is published, so no
`mu` is involved; `savePersistedIndex` is called under `mu` today — keep that
call site's locking unchanged.
- The persisted-index format on disk (`.rela/` cache) must not change;
`TestPersistence*`/recovery tests pin it.

## Done when

storetest conformance + fuzz corpus green, `go test -race ./internal/store/...`,
ratchet `//plimsoll:max-methods` (fsstore.go:159).
