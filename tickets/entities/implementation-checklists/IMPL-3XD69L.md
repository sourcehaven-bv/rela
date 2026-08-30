---
id: IMPL-3XD69L
type: implementation-checklist
title: 'Implementation: Promote ProjectionProvider, SweepConfig and VersionStore into internal/store so sweep capabilities are backend-neutral'
status: done
---

<!-- @managed: claude-workflow v1 -->

Branch `refactor/promote-sweep-types-TKT-L3FNEN`, off develop.

## Development

- [x] Unit tests written for new code — `backendneutral_postgres_test.go`
- [x] Integration tests written — the postgres conformance suite and the
appbuild wiring suite both run against a live database; they exercise the real
resolvers through the widened signatures rather than only the doubles
- [x] Happy path implemented
- [x] Edge cases from planning handled — typed-nil on every widened return;
partial adoption (a backend with one capability but not the others)
- [x] Error handling in place — the nil guards are unchanged and now guard more

## Test Quality

- [x] Using fixture builders or factories for test data — doubles embed a real
`memstore` so they satisfy `store.Store` without hand-writing 26 methods
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — each double implements exactly one
capability, which is what demonstrates the interfaces stay independent
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — no pgstore types in the capability interfaces | **PASS** | the only remaining `pgstore.` reference is the state-handle lookup, whose *return* is now the `rawStateStore` interface |
| 2 — satisfiable without importing pgstore | **PASS** | `backendneutral_postgres_test.go` builds every double from store-package types only |
| 3 — pgstore's method count unchanged | **PASS** | `just plimsoll` clean; no method added |
| 4 — postgres suites unchanged | **PASS** | `pgstore` 143s, `appbuild` 8.2s, `appbuildtest`, `backendtest` — all green against a live DB |
| 5 — arch-lint / lint | **PASS** | arch-lint "OK - No warnings found"; my files 0 issues under both tags |

**AC-2 is proven STRUCTURALLY, not by assertion.** The test file does not import
pgstore, so if a pgstore type crept back into a signature the file would stop
compiling — the loudest available failure. Verified by reverting
`VersionStore()` to its concrete return type:

```
cannot use (*pgstore.Store)(nil) as versionServiceProvider value:
  *pgstore.Store does not implement store.VersionServiceProvider
  (wrong type for method VersionStore)
```

That is the difference between this and a cosmetic promotion that greps clean.

**Two findings from implementation worth recording:**

1. **`StateStoreFor` could not be widened the obvious way.** Returning
`state.KV` reads as the tidier fix, but **pgstore must not import
`internal/state`** — arch-lint forbids a store depending on an application
package, and CLAUDE.md names that rule as what keeps key validation the state
package's job. I wrote the change, hit the build failure, and moved the widening
to the consumer side via a `rawStateStore` interface instead. Recorded at both
sites so the next person does not retry it.

2. **`withDefaults` had to stop being a method.** Go forbids methods on an
aliased type. That is the right split regardless: the FIELDS are a
backend-neutral contract, but the DEFAULTS (5m/5m/1h/500) are pgstore's tuning
and should not be imposed on another backend.

## Quality

- [x] Code follows project patterns — the promoted types sit beside
`VersionService` and `DerivedObjectSpec`, which are contract types for optional
capabilities for exactly the same reason
- [x] Checked for DRY opportunities — aliases rather than duplicate
definitions, so there is one definition and pgstore's call sites are untouched
- [x] No security issues introduced — no new input, no wire change;
`state.NewValidatedKV` still wraps the raw handle at the same site, which is
what applies the key rules
- [x] No silent failures — the typed-nil guards are retained and now cover more
implementations than before
- [x] No debug code left behind

**Pre-existing lint findings left alone:** 7 issues under `-tags postgres` in
`internal/tenant`, `internal/jobs` and two in files I did not create. Verified
they reproduce on a clean develop (stashing with `-u`, since an untracked test
file made the first check invalid). Fixing them is unrelated scope.
