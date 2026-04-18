---
id: TKT-SUREF
type: ticket
title: Refactor storeutil — decompose ValidateID, return bool from SortedRemove, move MatchRelation to RelationQuery
kind: refactor
priority: medium
effort: s
status: backlog
---

## Description

Three improvements to `internal/store/storeutil/storeutil.go` identified during
the architecture review of PR #403 (TKT-CO4YP). Each is a small API-shape change
with callers in `fsstore`, `memstore`, and the planned `boltstore`.

### 1. Decompose `ValidateID`

Today `ValidateID` conflates three distinct rule sets:

- `--` sequence → collides with the `from--type--to` relation key format
  (a `store` package concern)
- Path separators + NUL → filesystem safety (fsstore concern)
- Control characters → bucket-key safety (boltstore concern)

Split into composable primitives:

```go
func ValidateIDChars(id string) error       // NUL, control, separators
func ValidateIDRelationKey(id string) error // also rejects "--"
func ValidateID(id string) error            // calls both (common path)
```

This makes each rule's rationale visible per-backend and lets a hypothetical
pure-KV backend opt out of relation-key rules honestly.

### 2. `SortedRemove` should not panic

Current contract panics when the key is missing ("callers confirm presence
first"). A bug in any backend panics the whole process. Change to:

```go
func SortedRemove(s []string, key string) (result []string, removed bool)
```

Mirrors `sync.Map.LoadAndDelete`. Call sites that truly must-exist can wrap
with their own assertion.

### 3. Move `MatchRelation` to `RelationQuery.Matches`

`storeutil.MatchRelation(r, q)` has zero overlap with ID/cursor/sort
concerns and depends entirely on `store.RelationQuery` / `store.Direction`.
Natural home is `internal/store/store.go`:

```go
func (q RelationQuery) Matches(r *entity.Relation) bool
```

Discoverable next to the type definition and removes the
`store → storeutil → store` conceptual round-trip.

## Motivation

From architect review in PR #403. None of these block the PR, but they are
real improvements worth landing as a follow-up.
