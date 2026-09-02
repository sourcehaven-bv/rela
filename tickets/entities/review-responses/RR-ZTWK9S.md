---
id: RR-ZTWK9S
type: review-response
title: sync push nil-derefs the applier; the silent fallback makes it reachable
finding: NewEngine's godoc and buildSyncEngine's comment both claim push needs no applier - push.go:302 dereferences it on the normal create-id-adoption path
severity: critical
resolution: 'Guarded the deref in recordCreate: a nil applier on the id-adoption path now returns errLocalApplierRequired (renamed from errRemoteApplierRequired since push qualifies too) instead of panicking. Corrected NewEngine''s godoc - the nil case is ''a run that never writes locally'' which is narrower than ''push-only''. Pinned by TestRecordCreate_NilApplier_ErrorsNotPanics; verified it genuinely catches the bug by reverting the guard and observing the nil-pointer panic.'
status: addressed
---

`internal/cli/sync/engine.go:54` documents *"applier may be nil for a push-only
run (push never writes locally)"*, and `internal/cli/sync.go:119-120` repeats
it: *"a build that doesn't [satisfy this] would only break pull (push needs no
applier)"*. **Both are false.**

`Pull` (`pull.go:78`) and `Force` (`force.go:58`) guard with
`errRemoteApplierRequired`. `Push` does not, and `push.go:302` dereferences it
unconditionally:

```go
if newID != ch.Key {
    if _, err := e.applier.RenameEntity(ctx, ch.Key, newID, entity.RenameOptions{}); err != nil {
```

`newID != ch.Key` is the **normal** create-adoption path — the primary mints an
id different from the local temp id. So a nil applier there is a nil-interface
method call and a panic, not a degraded pull.

The bug is pre-existing, but this ticket is what makes it worth fixing now: the
field went from a concrete type to an interface, so the assertion at
`sync.go:117` is genuinely fallible for the first time.

CLAUDE.md's "Constructors reject nil required fields" covers this exactly, and
it says *"never substitute a no-op or sentinel implementation silently"* — which
is verbatim what `applier = nil` does.
