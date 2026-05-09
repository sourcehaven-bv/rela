---
id: RR-HK4NH
type: review-response
title: Storage iteration errors silently dropped, leaving partial maps
finding: luaMdEntityRefs at markdown.go:1675-1677 does 'if listErr != nil { continue }'. Per store.Store contract, the iterator terminates on error — so 'continue' is misnomer, but more importantly the error is dropped. A corrupt markdown file or disk hiccup yields a silently-incomplete replacements map. resolve_refs then fails to link affected entities with no warning. Should raise a Lua error so callers can pcall if they want resilience.
severity: critical
resolution: Storage iteration error now raises a Lua error naming the failing type. Added test TestMdEntityRefs_StoreError using a failingListStore wrapper that yields an error for type 'ticket'; verified the error message contains both the canonical prefix and the type name.
status: addressed
---

# Finding

```go
for e, listErr := range r.deps.Store.ListEntities(ctx, store.EntityQuery{Type: t}) {
    if listErr != nil {
        continue
    }
    ...
}
```

`Store.ListEntities`'s contract (per `internal/store/store.go:69-70`): "If an
error is yielded, the iterator terminates." So `continue` doesn't actually
iterate further — but that's a minor wart. The real problem is the error is
dropped on the floor.

Practical effect: a corrupt markdown file, fs lock, or storage hiccup yields an
incomplete `replacements` map. `resolve_refs` silently leaves those IDs as plain
text. The user sees no error and may not notice for a long time.

# Resolution

Raise a Lua error:

```go
if listErr != nil {
    ls.RaiseError("rela.md.entity_refs: list entities of type %q: %s", t, listErr.Error())
    return 0
}
```

Users who want resilience to per-type failures can wrap the call in `pcall`.

Add a test using a mock store that yields an error for one type.
