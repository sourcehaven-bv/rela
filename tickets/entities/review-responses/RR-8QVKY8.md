---
id: RR-8QVKY8
type: review-response
title: anyFaceOf swallowed transient store errors, reopening the fail-closed ACL gap
finding: anyFaceOf discarded the error from its first GetEntity lookup and fell through to a query, so a transient backend error was reported as not-found. Every caller's not-found branch skips the ACL check by design, so a store hiccup turned an ACL-gated rename or delete into an ungated one. Caught by TestRename_FailsClosedOnNonNotFoundFetchError, which failed in just check.
severity: critical
resolution: anyFaceOf now returns any non-ErrNotFound error as-is; only a genuine miss falls through. Fixed the same collapse-every-error pattern at four further pre-ACL sites (UpdateEntity pre-image, DeleteEntity, DeleteEntityState, both relation endpoints) and added TestDelete_FailsClosedOnNonNotFoundFetchError, which delete previously lacked.
status: addressed
---

## Finding

Introduced by the fix for RR-H0PXTE. Routing `DeleteEntity` and `RenameEntity`
through `anyFaceOf` fixed the faced-entity lookup, but `anyFaceOf` opened with:

```go
if e, err := st.GetEntity(ctx, id); err == nil {
    return e, nil
}
```

The error is discarded and the function falls through to a query. A transient
backend error therefore became "not found" — and every caller's not-found branch
returns *before* the ACL check, because existence is itself a secret.

So a store hiccup turned an ACL-gated operation into an ungated one. That is the
precise defect `TestRename_FailsClosedOnNonNotFoundFetchError` was written to
pin, reintroduced through a different read path.

## How it was caught

`just check`. The failing package scrolled past a `tail -40` in the first run,
so I initially reported the gates as green when they were not.

## Resolution

`anyFaceOf` now returns any non-`ErrNotFound` error as-is; only a genuine miss
falls through to the next lookup. Mutation-verified: restoring the swallow
reproduces the original failure exactly.

Auditing for the same shape found four further pre-ACL sites collapsing every
error into `ErrEntityNotFound` — `UpdateEntity`'s pre-image read,
`DeleteEntity`, `DeleteEntityState`, and both relation endpoints. All now fail
closed. The last three pre-date BUG-HC6I2T.

Added `TestDelete_FailsClosedOnNonNotFoundFetchError`. Delete had no such test,
which is why the same defect could reach it unnoticed while rename was pinned.
