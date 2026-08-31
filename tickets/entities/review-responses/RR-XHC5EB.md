---
id: RR-XHC5EB
type: review-response
title: writeServices should hold the sync applier as a typed field rather than reconstruct it by assertion
finding: appbuild now hands out the concrete *Manager so the type is statically known at the wiring site - re-asserting it is a downgrade
severity: significant
resolution: Adopted. writeServices now carries a typed SyncApplier syncclient.LocalApplier field assigned from the concrete svc.EntityManager() at the wiring site. The fallible assertion and its fail-open 'applier = nil' fallback are deleted from buildSyncEngine.
status: addressed
---

`Services.EntityManager()` now returns the concrete `*entitymanager.Manager`, so
the applier capability is **statically known** at the CLI wiring site
(`cli_wiring.go`). Narrowing to `entityWriter` and then reconstructing the
capability via a runtime assertion turns a compile-time fact into a runtime one
— and then needs a test to compensate for the downgrade.

Give `writeServices` a second explicitly-typed field, assigned from the same
concrete value at the wiring site:

```go
type writeServices struct {
    readServices
    EntityManager entityWriter
    // SyncApplier is the id-preserving write path `rela sync` needs. A separate
    // field, not an assertion on EntityManager: sync's capability is distinct
    // from the human-intent write surface, and wiring it explicitly makes a
    // missing applier a compile error rather than a silently disabled pull.
    SyncApplier syncclient.LocalApplier
}
```

The assertion, the fallback, and the regression test all disappear, replaced by
the compiler. This is CLAUDE.md's "define a typed dependency instead of a
back-channel type assertion" applied to the case it was written for.
