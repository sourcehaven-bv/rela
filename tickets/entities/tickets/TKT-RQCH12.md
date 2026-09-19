---
id: TKT-RQCH12
type: ticket
title: Service.Add lists a whole thread to count it before every comment post
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Raised during the TKT-4LG36M code review. That ticket removed the read
amplification on the authorization path; this is the same shape on the write
path, left in place because fixing it was out of scope.

`Service.Add` enforces `MaxPerTarget` by listing and counting:

```go
existing, err := s.store.List(ctx, target)
if err != nil {
    return Comment{}, err
}
if len(existing) >= MaxPerTarget {
```

So every comment posted pulls up to 500 rows — with their bodies, each capped at
16 KiB — out of the database to produce one integer.

## Why this is worse than the case already fixed

TKT-4LG36M's amplification was on edit and delete. This one is on **create**,
and it is unconditional: there is no path through `Add` that skips it. It is
also the more common operation.

## Why it is still low priority

The cap is already documented as ADVISORY on the database backends, and
deliberately so:

> Left as-is deliberately. The cap exists to bound the FILE backend's
> whole-thread document reads, so overshooting it by a handful of rows on a
> backend that pages costs nothing; making it exact would mean a conditional
> insert and a new method on comments.Store, which is a contract change for an
> invariant that does not need to be exact.

That reasoning is about EXACTNESS under concurrency, and it still holds. It does
not argue for reading 500 bodies to get a count, which is a separate cost.

## Approach sketch

Options, cheapest first:

1. **`Count(ctx, target) (int, error)` on `comments.Store`.** `SELECT count(*)
WHERE target_key = ?` on the database backends, `len(thread)` on the others. One
more interface method, in the same shape as `Get`.
2. **Push the cap into the insert.** A conditional insert would make the cap
exact as well as cheap, but it is the contract change the `MaxPerTarget` doc
already declined, and exactness is explicitly not needed.
3. **Drop the pre-check on paging backends.** The cap exists to bound the FILE
backend's document reads; arguably it need not run where that cost is absent.
Cheapest of all, and it makes the backends behave differently, which is the kind
of divergence the conformance suite exists to prevent.

Option 1 is the recommendation: it matches what was just done for `Get`, keeps
one contract across four backends, and leaves the advisory-cap reasoning intact.

If taken, `commentstest` needs a `RunCountTests` block and a counting-store
assertion mirroring `TestGet_DoesNotReadTheWholeThread`, or the same regression
can return unobserved.

## Acceptance criteria

1. Posting a comment no longer reads the thread's bodies to enforce the cap.
2. The cap still rejects a post to a target already at `MaxPerTarget`.
3. Whatever mechanism is chosen behaves identically on all four backends, pinned
by the conformance suite.
4. A test pins the read cost, so the amplification cannot come back silently.
