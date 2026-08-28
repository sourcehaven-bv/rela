---
id: PLAN-V0003S
type: planning-checklist
title: 'Planning: appbuild: widen the three *pgstore.Store concrete-type assertions to interfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Four `st.(*pgstore.Store)` concrete-type assertions in `internal/appbuild` make
pgstore capabilities undiscoverable by any other backend. Replace them with
consumer-side interfaces, per the pattern the codebase already uses everywhere
else (`HistoryReader`, `Formatter`, `TypeWatermark` are all interface-asserted
at the wiring site).

IN scope — the four sites:

| Site | Capability |
|---|---|
| `derivedschema_postgres.go:23` | `SetUniqueSpecProvider` + `Reconcile` |
| `userstate_postgres.go:27` | `UserState()` |
| `versionsweep_postgres.go:42` | `StartVersionSweep` |
| `versionsweep_postgres.go:93` | `VersionStore()` |

Plus `pgstore.StateStoreFor` (`statekv.go:72`), which performs the same concrete
assertion internally on behalf of `stateKVFor`.

NOT in scope:

- Implementing any capability for a new backend. This removes the structural
block only.
- Changing behaviour. The postgres build must be behaviourally identical.
- Adding accessor methods to `pgstore.Store` — see the risk table; its plimsoll
load line explicitly warns against exactly that.

**Acceptance Criteria:**

1. **No `st.(*pgstore.Store)` remains in `internal/appbuild`.** Test: `grep`
returns nothing; verified in CI-visible form by the assertion being gone from
all four sites.
2. **pgstore behaviour unchanged.** Test: the postgres conformance suite and
the appbuild wiring tests pass against a real database (`RELA_TEST_DATABASE_URL`
is available in this environment — verified, the suite runs rather than skips).
3. **A non-pgstore store satisfying the interfaces is wired identically.**
Test: a fake implementing the new interfaces, asserted through the same
resolvers, receives the same calls. This is the criterion that actually proves
the block is gone; without it the refactor is unverified.
4. **The nil-fallback contract still holds.** Test: a store implementing none
of the interfaces yields a genuinely nil interface (not a typed nil), so callers
fall back to FSKV / in-memory user state. A typed nil would silently defeat
every `!= nil` check downstream.
5. `just arch-lint` clean; `go build` for default, `memorybackend` and
`postgres` tags.

## Research

- [x] For larger features: run `/research` — N/A, RES-03TUXO already covers it
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-03TUXO — identified these assertions as the structural
prerequisite for a third smart backend.

**Existing Solutions:**

The pattern to copy is already in-tree, which is what makes this low-risk:

- **`store.VersionService`** (`store.go:909-916`) — an umbrella interface used
purely as a wiring vehicle, returning nil on non-pg builds. `versionServiceFor`
already RETURNS an interface; only its discovery is concrete.
- **Optional capabilities asserted as interfaces**: `HeaderReader`
(`store.go:474`), `Formatter` (`cli/fmt.go:20`), `TypeWatermark`
(`dataentry/caldav_backend.go:660`).
- **Consumer-side interface declaration** is the documented house rule
(CLAUDE.md; `dataentry/sync.go:17` `manifestProvider` is the model).
- **The genuinely-nil contract** is spelled out in `userstate_nostore.go:16`
and `versionsweep_nosweep.go:20,26` — preserve it verbatim.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Declare consumer-side interfaces in `internal/appbuild` (the consumer), assert
against those, and let pgstore satisfy them structurally with **no change to
pgstore at all**. That last point is the crux: `pgstore.Store` carries
`//plimsoll:max-exported-methods=38` plus an explicit warning that a third
capability accessor "should not raise these numbers again", so this refactor
must not add methods there.

Four interfaces, each named for what the consumer needs:

```go
// derivedSchemaReconciler — derivedschema_postgres.go
type derivedSchemaReconciler interface {
    SetUniqueSpecProvider(specs []store.DerivedObjectSpec)
    Reconcile(ctx context.Context, desired []store.DerivedObjectSpec,
        opts store.ReconcileOptions) ([]store.DerivedObjectOutcome, error)
}

// userStateProvider — userstate_postgres.go
type userStateProvider interface {
    UserState() (userstate.Store, error)
}

// versionServiceProvider — versionsweep_postgres.go
type versionServiceProvider interface {
    VersionStore() *pgstore.VersionStore   // see note below
}

// versionSweeper — versionsweep_postgres.go
type versionSweeper interface {
    StartVersionSweep(provider pgstore.ProjectionProvider, cfg pgstore.SweepConfig)
}
```

**Two of these still name pgstore types, and that is a deliberate limit of this
ticket.** `StartVersionSweep` takes `pgstore.ProjectionProvider` and
`pgstore.SweepConfig`; `VersionStore()` returns `*pgstore.VersionStore`. A
second backend could satisfy them only by importing pgstore, which is not
genuine decoupling. Fully generalising means promoting those types into
`internal/store` — a larger, separate change touching pgstore's public API.

What this ticket delivers is nonetheless the load-bearing half: **discovery is
no longer welded to one concrete type**, the files stay build-tagged so nothing
non-postgres links pgstore, and the remaining coupling is now visible in an
interface signature instead of hidden in a type assertion. The residual is
recorded as a follow-up rather than pretended away.

`stateKVFor` keeps calling `pgstore.StateStoreFor`, whose internal assertion is
pgstore's own business (it is documented there as a deliberate alternative to a
`Store` method, for the same plimsoll reason). Widening it is the same
promote-the-types problem; noted in the follow-up.

**Files to modify:**

- `internal/appbuild/derivedschema_postgres.go` — interface + assertion
- `internal/appbuild/userstate_postgres.go` — interface + assertion
- `internal/appbuild/versionsweep_postgres.go` — two interfaces + assertions
- `internal/appbuild/widen_assertions_postgres_test.go` — NEW: fake-backed test
proving a non-pgstore type is wired identically (AC-3) and that a store
implementing nothing yields untyped nil (AC-4)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None. This is a compile-time discovery change;
no new input reaches the process, no wire format changes, no new parsing.

**Security-Sensitive Operations:**

- **`state.NewValidatedKV` must keep wrapping the raw KV.** It applies the key
rules FSKV gets from RootedFS; dropping it would let a caller write arbitrary
keys. Untouched here, and the test asserts the wrapper is still applied.
- **No ACL surface.** Read gating lives in `internal/visibility` decorators at
the wiring site, never in the store (DEC-ZBI39P).
- **Widening does not widen privilege.** An interface assertion admits a type
that implements the methods; it does not grant a capability the store did not
already have. The capabilities remain per-build via the existing tags.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Scenario | Pass condition |
|----|----------|----------------|
| 1 | `grep -rn 'st\.(\*pgstore\.Store)' internal/appbuild` | no matches |
| 2 | `go test -tags postgres ./internal/store/pgstore/ ./internal/appbuild/...` against a live DB | all pass, unchanged |
| 3 | Fake implementing `derivedSchemaReconciler` / `userStateProvider` / `versionServiceProvider` / `versionSweeper`, passed to each resolver | each resolver calls through to the fake |
| 4 | A bare `store.Store` (memstore) passed to each resolver | returns are `== nil` as an INTERFACE, not a typed nil |
| 5 | `just arch-lint`; build all three tags | clean |

**Edge Cases:**

- **Typed-nil trap.** `var s *pgstore.Store; return s` satisfies the interface
but is `!= nil`. The tests assert `got == nil` on the interface value
specifically — this is the failure mode the existing doc comments warn about
three separate times, so it deserves a test rather than a comment.
- **Partial implementation.** A store implementing `UserState()` but not
`Reconcile` must get user-state wiring and skip derived-schema. Separate
interfaces (not one umbrella) is what makes this work.
- **`UserState()` returning an error** must still yield untyped nil so the
caller falls back, rather than a non-nil interface wrapping a broken handle.
- **`Reconcile` returning `ErrReconcileBusy`** must remain a Debug-level
non-failure, not a warning.
- **memstore/fsstore under the postgres tag** — the "should not happen" path;
must skip silently, not panic.

**Negative Tests:**

- Store implementing none of the interfaces → every resolver returns nil and
the FSKV / in-memory fallbacks engage.
- Sweep never started for a store lacking `StartVersionSweep`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Typed nil silently defeats every downstream nil-check | Medium | Explicit tests on the interface value; the existing doc comments already flag it, now pinned by assertions |
| Temptation to add accessor methods to `pgstore.Store` | Medium | Forbidden: its plimsoll line says a third capability accessor "should not raise these numbers again". Interfaces are declared consumer-side; **pgstore is not modified at all** |
| Refactor looks complete but coupling remains via pgstore types in signatures | **High** | Stated plainly in the Approach and filed as a follow-up. Do NOT let this ticket claim full decoupling |
| Postgres tests skipped, so AC-2 unverified | Low | A local postgres is running and `rela_test_twio` is created; the suite was confirmed to RUN (13.2s) rather than skip before starting |
| Behaviour drift in log levels / error paths | Low | Resolver bodies are unchanged below the assertion; only the discovery line changes |

**Effort:** m — four small mechanical changes plus a genuinely new test file.
The care is in the nil semantics and in not overstating the outcome.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — internal refactor, no user-facing behaviour change. The doc
comments at each site are updated in place to say "a store that implements X"
instead of "a *pgstore.Store", since those comments currently assert the
concrete type as a fact.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Self-reviewed against the codebase before implementation; the prior ticket's
external review surfaced the two traps already folded in above:

1. **Do not add methods to `pgstore.Store`** — its plimsoll directive and the
`StateStoreFor` doc comment both warn that capability accessors re-invite the
pattern being removed. Resolved: interfaces declared consumer-side, pgstore
untouched.
2. **Do not overclaim.** Two interfaces still name pgstore types, so a second
backend cannot satisfy them without importing pgstore. Resolved by stating the
residual explicitly in the Approach and filing it as a follow-up rather than
letting "no concrete assertions remain" imply full decoupling.
