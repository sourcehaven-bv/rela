---
id: BUG-RIM0CT
type: bug
title: RenameEntity skips ACL when the pre-fetch returns a non-not-found error
description: Manager.RenameEntity nested its ACL check inside `if current, err := Store.GetEntity(oldID); err == nil`. Any GetEntity error other than not-found (transient I/O, backend hiccup) skipped authorization entirely and proceeded to rename — fail-open on an authorization gate. A flaky store read could turn an ACL-gated rename into an ungated one.
priority: high
why1: The ACL block was guarded by `err == nil`, so every non-nil GetEntity error — not just not-found — fell through to the unauthorized rename.
why2: The guard's comment assumed the only failure mode worth handling was not-found ('only consult ACL when there's an entity to authorize against'), conflating 'no entity' with 'fetch failed'.
why3: GetEntity's error contract (returns store.ErrNotFound for missing, other errors for I/O/backend failures) was not distinguished at this call site.
why4: Authorization gates had no established fail-closed convention for collaborator errors; the default fell through to the action.
why5: Fail-open vs fail-closed for a flaky dependency on an authz path is a security-critical default that no lint or review checklist enforces.
prevention: 'The pre-fetch now classifies the error: skip ACL only on errors.Is(err, store.ErrNotFound); fail closed (return the wrapped error, no rename) on any other error. A regression test (flakyGetStore returning a non-not-found error under a deny-all ACL) asserts the error surfaces and zero store writes happen; it fails without the fix.'
status: done
---

## Bug

Found in the 2026-06-09 backend review (write-path theme A2 — ACL enforcement
consistency).

`Manager.RenameEntity` (`manager.go:531-538`):

```go
if current, err := m.deps.Store.GetEntity(ctx, oldID); err == nil {
    // ... ACL check ...
}
res, err := renameEntity(...)  // runs regardless
```

The ACL check is inside `err == nil`. `GetEntity` returns `store.ErrNotFound`
for a missing entity but *other* errors for transient I/O or backend failures.
On any non-not-found error the ACL block is skipped and `renameEntity` runs with
**no authorization** — fail-open on an authz gate.

## Fix (PR pending)

Classify the fetch error:

- `err == nil` → run the ACL check as before.
- `errors.Is(err, store.ErrNotFound)` → skip ACL, fall through (renameEntity returns `ErrEntityNotFound`; nothing to authorize).
- any other error → fail closed, return the wrapped error without renaming.

Regression test `TestRename_FailsClosedOnNonNotFoundFetchError` uses a
`flakyGetStore` returning a non-not-found error under a deny-all ACL: asserts
the underlying error surfaces and zero store writes occur. Verified it fails
without the fix (the masked error became `entity not found` from rename's own
second fetch, proving the rename ran).
`TestRename_NotFoundStillReturnsTypedError` pins the not-found path.
