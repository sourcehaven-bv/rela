---
id: REV-M4X4CX
type: review-checklist
title: 'Review: appbuild: widen the three *pgstore.Store concrete-type assertions to interfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

Branch `refactor/appbuild-widen-assertions-TKT-415WA7` · `caa2923b` (refactor)
+ `78e543f5` (review fixes).

## Automated Checks

- [x] **Postgres suites against a LIVE database** (not skipped):
`internal/appbuild` 2.5s · `appbuildtest` 1.3s · `backendtest` 0.6s ·
`store/pgstore` 31.6s — all ok.
- [x] **All three build tags**: default, `memorybackend`, `postgres` — all build.
- [x] `just arch-lint` — "OK - No warnings found".
- [x] `golangci-lint run --build-tags postgres ./internal/appbuild/...` —
0 issues. Run with the tag explicitly, because the default lint pass does not
compile the postgres-tagged files this ticket changes.
- [x] Default-build `./internal/appbuild/... ./internal/store/...` — pass.
- [x] `go mod tidy` — dropped the `modernc.org/sqlite` dependency carried over
from the spike branch.

## Code Review

- [x] Reviewed by the `cranky-code-reviewer` agent against the full diff
- [x] All critical and significant findings addressed

**It found a real bug I introduced, and the diagnosis was better than mine.**

`versionServiceFor` returned `s.VersionStore()` directly into
`store.VersionService`, so a nil pointer boxes into a **non-nil** interface —
exactly what the doc comment three lines above promises not to do. The insight I
had missed: this was **new, not pre-existing**. Asserting `st.(*pgstore.Store)`
had bounded the reachable implementations to exactly one, whose `VersionStore()`
is unconditionally non-nil. The concrete assertion was doing load-bearing work I
had not credited it for; widening discovery removed that bound while the return
type stayed concrete, so the code still *read* as safe. Downstream,
`versionRecorderFor` and `startDataMigration` both nil-check, both would have
passed, then panicked on first use — at write time, in production.

I verified the mechanism in an isolated 30-line program before fixing (unguarded
`!= nil` = true; guarded = false).

| ID | Severity | Status | Finding |
|----|----------|--------|---------|
| RR-E6ADZK | critical | addressed | `versionServiceFor` leaks a typed nil (+ same shape in `storeUserStateFor`) |
| RR-ZGLA3W | significant | addressed | The negative test could not reach the branch that regressed |
| RR-HNIT8N | significant | addressed | Dead assertion: `x == nil && len(x) != 0` is unsatisfiable |
| RR-WJK3DQ | significant | addressed | `stateKVFor` unwidened while the test implied parity |
| RR-XGJLO1 | minor | addressed | Compile-time assertion wrapped in a `func Test`; unused `*testing.T` |
| RR-IEXU1I | nit | **deferred** | `versionSweeper` naming; `StartVersionSweep` has no stop counterpart |

Only the nit is deferred, with a reason: both points are about the capability's
own contract rather than this refactor, and TKT-L3FNEN reshapes these signatures
anyway — renaming now would just churn.

**Every fix was verified by deliberate regression, not just by going green:**

- Typed-nil guard → removing it makes
`TestCapabilityPresentButHandleNilYieldsUntypedNil` fail with `versionServiceFor
= (*pgstore.VersionStore)(nil), want untyped nil`.
- Call-order assertion → swapping the order in the resolver fails with
`call order = [Reconcile SetUniqueSpecProvider], want [SetUniqueSpecProvider
Reconcile]`.
- Interface discovery → restoring `st.(*pgstore.Store)` fails with
`storeUserStateFor did not reach a non-pgstore store implementing UserState`.

That last one matters because the reviewer's mutation test showed the *original*
`TestDerivedSchemaPublishesSpecsBeforeReconciling` passed even with
`SetUniqueSpecProvider` deleted outright — a test that existed only to look like
coverage. I had independently suspected that line and flagged it in the review
request; the mutation test confirmed it.

## Acceptance Verification

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — no `st.(*pgstore.Store)` in appbuild | **PASS** | `grep` returns nothing |
| 2 — pgstore behaviour unchanged | **PASS** | Full postgres suites green against a live DB |
| 3 — non-pgstore store wired identically | **PASS** | `TestCapabilitiesDiscoveredByInterface_NotConcreteType`, proven non-vacuous |
| 4 — untyped-nil contract holds | **PASS** | Now covers BOTH branches: no-capability and capability-returns-nil |
| 5 — arch-lint + build tags | **PASS** | arch-lint clean; three tags build; lint 0 issues under the postgres tag |

**Scope honestly stated, not overclaimed.** Three of four sites are widened;
`stateKVFor` is deliberately not, because `pgstore.StateStoreFor` is a package
function by design (pgstore's plimsoll line forbids growing its method set).
That exclusion is now documented at the call site with its runtime consequence —
a second backend would silently fall back to node-local FSKV for state — and
TKT-L3FNEN says it must close **before** a second backend ships, not merely
before the work is "complete".

## Documentation

- [x] ~~User-facing docs~~ (N/A: internal refactor, no behaviour change)
- [x] Doc comments updated in place — they previously asserted the concrete
type as fact; they now describe the capability, and the typed-nil guard carries
a comment explaining *why* it became necessary, so it is not deleted later as
redundant
- [x] Follow-up filed and sharpened — TKT-L3FNEN gained two sections from this
review: why the half-migration was a correctness gap rather than aesthetic debt,
and the `stateKVFor` exclusion
