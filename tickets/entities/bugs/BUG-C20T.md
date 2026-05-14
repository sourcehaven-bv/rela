---
id: BUG-C20T
type: bug
title: DeleteEntity silently swallows non-NotFound relation-delete errors
priority: medium
effort: s
status: backlog
---

## Summary

`entitymanager.Manager.DeleteEntity` and
`entitymanager.cascadeHost.DeleteEntity` both contain this pattern:

```go
if delErr := m.deps.Store.DeleteRelation(...); delErr != nil &&
    !errors.Is(delErr, store.ErrNotFound) {
    continue
}
```

A non-`ErrNotFound` error (I/O failure, lock contention, permission denied,
etc.) is silently swallowed AND the relation is not counted in
`DeletedRelations`. The entity-delete then proceeds at the bottom of the
function. End result: a dangling relation pointing at a deleted entity, with no
caller-visible error.

Surfaced by cranky review on TKT-IU2S (#11). Pre-existing — shipped in TKT-QTNX.
The shape exists in two places after TKT-IU2S (Manager.DeleteEntity at
manager.go:294-298, 301-306 and cascadeHost.DeleteEntity at
cascadehost.go:94-99, 100-105).

## Approach

Either:
1. Log + count: `slog.Warn` on the failure, but still abort and
return the error so the caller knows the delete was partial.
2. Fail fast: stop the loop on the first non-NotFound error and
return it.

Pick (2) — graph corruption is a worse failure mode than a partial delete that
the caller can retry.
