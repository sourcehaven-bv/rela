---
id: RR-NFXSZ7
type: review-response
title: rela migrate baseline writes migration state without holding the migration lock
finding: '[security] MigrateBaselineCmd.Run was the only writer of the migration record doing an unserialized load-decide-save. Every other writer takes the migration lock (Runner.Run holds it for the whole apply; Gate.adopt/adoptWithDrift go through withLock). A concurrent `migrate data --apply` that records its first file is clobbered by a baseline whose existing==nil check ran before that write landed, and Save is a whole-record replace on all three durable backends. The migrations the baseline then claims are applied never run and never will, because Resolve keys purely on name membership - the exact failure StatusUnbaselined exists to prevent, reintroduced by the command meant to resolve it. The un-baselined state is the normal starting point for both commands, so the window is not exotic.'
severity: significant
resolution: 'Fixed: baseline now takes migrationLock(svc).TryAcquire before the Load, releasing on return. Contention FAILS rather than skipping (unlike the gate), because a baseline is an explicit operator claim and silently doing nothing would leave the operator believing a claim that was never recorded - Runner.Run sets that precedent. Pinned by TestMigrateBaseline_RefusesWhileTheMigrationLockIsHeld, which was verified to fail when the lock is removed.'
status: addressed
---

## Finding

`MigrateBaselineCmd.Run` (`internal/cli/migrate_data.go:426-473`) was the only
one of the five `migrate` subcommands that never called `migrationLock(svc)`. It
performed an unserialized read-modify-write:

```go
existing, err := svc.MigState.Load(ctx)   // :432
if existing != nil { ... return nil }     // :436
// ... no lock ...
svc.MigState.Save(ctx, st)                // :462
```

Every other writer of this record is serialized. `Runner.Run` holds the lock for
the whole apply (`run.go:120-127`) and `Gate.adopt` / `adoptWithDrift` go
through `Gate.withLock` (`gate.go:271-290`). The runner's own doc comment states
the lock exists so "record advances and bulk rewrites must not interleave with
another runner, a GC apply, or a gate adoption."

The clobber is total, not partial: `Save` is a whole-record replace on all three
durable backends (`pgmigstate/pg.go:87` and `sqlitemigstate/sqlite.go:84` both
`ON CONFLICT ... DO UPDATE SET state = excluded.state`;
`filemigstate/file.go:137` overwrites the file).

## Failure scenario

1. `migrations/` holds three files; the store has no recorded state — the
un-baselined case both commands exist to resolve.
2. Operator A runs `rela migrate data --apply`. It takes the lock, runs file 1,
and `advanceMarker` saves `Applied: [mig1]`.
3. Operator B runs `rela migrate baseline --apply`. Its `Load` returned `nil`
before step 2's write landed, so the `existing != nil` guard does not fire.
4. B saves `Applied: [mig1, mig2, mig3]`, overwriting A's record.
5. A continues and runs file 2. `mig3` stays recorded as applied while its steps
never ran — and never will, because `Resolve` keys purely on name membership
(`resolve.go:45-55`).

The data `mig3` was written to transform is silently left untransformed. This is
precisely the failure `StatusUnbaselined` exists to prevent ("silently
baselining marks them applied forever", `gate.go:38-48`), reintroduced by the
command meant to resolve it. The un-baselined state is the *expected* starting
point for both commands, so this is not an exotic window.

## Severity

Significant. Not an unauthenticated attack path — the trust boundary is
correctly the operator shell — but an operator-concurrency failure with silent
data-integrity consequences, in a subsystem whose whole job is transforming user
data correctly.

## Resolution

`baseline` now acquires the migration lock before the `Load` and releases on
return, so the load-decide-save is atomic against every other writer.

Contention **fails** rather than skipping, deliberately diverging from the
gate's `withLock`: a baseline is an explicit operator claim, and silently doing
nothing would leave the operator believing a claim that was never recorded.
`Runner.Run` already sets that precedent by returning `ErrLockHeld`.

Pinned by `TestMigrateBaseline_RefusesWhileTheMigrationLockIsHeld`, which I
verified genuinely catches the defect: it fails when the lock acquisition is
removed and passes with it.
