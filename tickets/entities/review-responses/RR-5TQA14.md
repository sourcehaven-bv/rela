---
id: RR-5TQA14
type: review-response
title: Gate.Persist blind-overwrites a record that moved since Evaluate; Verdict aliases the loaded slice
finding: Evaluate reads state outside the lock and captures pendingShape/pendingApplied; Persist writes NewState(...) inside the lock without re-reading. The lock serializes the writes but not the read-modify-write pair, so a concurrent migrate data --apply that records a file between the two has its entry overwritten and the migration silently re-runs. On the StatusBootstrapped path it is worse - the captured applied list is nil, so a record that appeared in between is replaced with an EMPTY one. Separately, classifyAgainst aliased st.Applied into the published Verdict, which is documented as immutable and shared across goroutines; the backends happen to return copies but nothing in the StateStore contract required it.
severity: significant
resolution: 'Fixed: Persist re-loads under the lock via recordMoved and declines when the stored shape differs from what Evaluate saw (a read error is NOT treated as unchanged - failing to confirm is when a blind overwrite is most dangerous). Verdict now clones the slice. migstatetest gained RunNoAliasingTests so the contract is enforced for every backend rather than relied on. Pinned by TestGate_PersistDeclinesWhenTheRecordMoved, verified to fail when the guard is removed.'
status: addressed
---

## Finding

Two related issues in the Evaluate/Persist split.

### The TOCTOU

`Evaluate` reads the state and captures `pendingShape` and `pendingApplied`.
`Persist` writes `NewState(pendingShape, pendingApplied, EvaluatedAt)` — a
**blind overwrite**, never re-reading.

`withLock` does not close this: the lock is taken inside `Persist`, long after
the read in `Evaluate`. It serializes the *writes* but not the read-modify-write
pair.

**Scenario:** `rela migrate status` evaluates. Concurrently a `rela migrate data
--apply` completes and records `20260919-x.yaml`. The first process then calls
`Persist`, writing back the applied list *as it was at evaluate time* — without
`20260919-x.yaml`. That migration is now un-recorded and will re-run.

The `StatusBootstrapped` path is worse: its captured applied list is `nil`, so a
record that appeared in between is replaced with an **empty** one rather than a
merely-stale one.

### The aliasing

`classifyAgainst` did `v.pendingApplied = st.Applied` with no clone. `Verdict`
is documented as immutable and shared across goroutines by atomic publication,
but that slice header pointed into a `*State` the gate neither owns nor controls
the lifetime of. All four backends happen to return copies; nothing in the
`StateStore` contract *required* it, so a future backend could break the promise
from a distance.

## Severity

Significant. The window is real but narrow today because `Persist` is CLI-only —
though `migrate gc` and `migrate status` both evaluate, and nothing stops two
shells.

## Resolution

- `Persist` now calls `recordMoved`, which re-loads under the lock and declines
when the stored shape differs from `v.StoreHash`. A read error is **not**
treated as unchanged: failing to confirm the record is where we left it is
exactly when a blind overwrite is most dangerous.
- `Verdict.pendingApplied` is cloned.
- `migstatetest` gained `RunNoAliasingTests`, which mutates a loaded `State` and
asserts a second `Load` is unaffected — so the contract is *enforced* for every
backend rather than relied upon. All four pass, postgres included.

Pinned by `TestGate_PersistDeclinesWhenTheRecordMoved`, verified to fail when
the guard is removed.

`Runner.advanceMarker` has the same read-modify-write shape but is genuinely
safe: the whole `Run` holds the lock, so its load and save are inside one
acquisition.
