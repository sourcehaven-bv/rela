---
id: IMPL-BB3OA5
type: implementation-checklist
title: 'Implementation: appbuild: widen the three *pgstore.Store concrete-type assertions to interfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

Branch: `refactor/appbuild-widen-assertions-TKT-415WA7` · commit `caa2923b`.

## Development

- [x] Unit tests written for new code — `widen_assertions_postgres_test.go`
- [x] Integration tests written — the existing appbuild wiring suite and the
pgstore conformance suite both run against a **live database** here
(`RELA_TEST_DATABASE_URL` pointed at a local `rela_test_twio`), so AC-2 is
verified rather than skipped.
- [x] Happy path implemented — all four assertion sites converted
- [x] Edge cases from planning handled — untyped-nil contract, partial
implementation, `UserState()` error path, `ErrReconcileBusy` still Debug-level
- [x] Error handling in place — resolver bodies below the assertion are
unchanged, so log levels and error paths are untouched

## Test Quality

- [x] Using fixture builders or factories for test data — fakes embed a real
`memstore` so they satisfy `store.Store` without hand-writing 26 methods, and
cannot silently diverge from the interface
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — each fake implements exactly one
capability, which is what proves the interfaces are independent
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — no `st.(*pgstore.Store)` in appbuild | **PASS** | `grep` returns nothing across all four sites |
| 2 — pgstore behaviour unchanged | **PASS** | Against a live DB: `internal/appbuild` 3.2s, `appbuildtest` 0.7s, `backendtest` 1.0s, `pgstore` 28.6s — all ok |
| 3 — non-pgstore store wired identically | **PASS** | `TestCapabilitiesDiscoveredByInterface_NotConcreteType` — three fakes, none a pgstore type, all reached |
| 4 — untyped-nil contract holds | **PASS** | `TestResolversReturnUntypedNilWithoutCapability` + `TestUserStateErrorYieldsUntypedNil` |
| 5 — arch-lint + all build tags | **PASS** | arch-lint "OK - No warnings found"; default / `memorybackend` / `postgres` all build; `golangci-lint` 0 issues, including `--build-tags postgres` |

**The AC-3 test is not vacuous — verified by deliberate regression.** I restored
the old `st.(*pgstore.Store)` assertion in `storeUserStateFor` and re-ran:

```
--- FAIL: TestCapabilitiesDiscoveredByInterface_NotConcreteType/user_state
    storeUserStateFor did not reach a non-pgstore store implementing UserState
```

Then reverted. Without that check the test would pass whether or not the
refactor worked, which is the usual way a refactor test provides false comfort.

**A useful signal fell out of the change:** `userstate_postgres.go` no longer
imports pgstore at all. That file is now genuinely backend-neutral.
`derivedschema_postgres.go` retains the import only for the `errors.Is(err,
pgstore.ErrReconcileBusy)` sentinel — far weaker coupling than a type assertion.

**Partial by design, and filed as TKT-L3FNEN.** `versionSweeper` and
`versionServiceProvider` still name pgstore types (`ProjectionProvider`,
`SweepConfig`, `*VersionStore`), so only pgstore can satisfy them in practice.
Discovery is unblocked; the signatures are not yet neutral. This is stated in
the code comments, the commit message and the follow-up ticket so "no concrete
assertions remain" cannot be misread as "backend-neutral".

## Quality

- [x] Code follows project patterns — consumer-side interfaces at the call
site, per CLAUDE.md and the existing `HistoryReader` / `Formatter` /
`TypeWatermark` precedent
- [x] Checked for DRY opportunities — four separate interfaces rather than one
umbrella, deliberately: a store implementing `UserState()` but not `Reconcile`
must get one and skip the other, which an umbrella would prevent
- [x] No security issues introduced — compile-time discovery change only; no
new input, no new parsing; `state.NewValidatedKV` still wraps the raw KV
- [x] No silent failures — the untyped-nil contract is now pinned by tests
rather than only by doc comments
- [x] No debug code left behind — `go.mod` re-tidied to drop the
`modernc.org/sqlite` dependency carried over from the spike branch
