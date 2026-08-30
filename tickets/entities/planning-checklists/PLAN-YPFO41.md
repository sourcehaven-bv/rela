---
id: PLAN-YPFO41
type: planning-checklist
title: 'Planning: Promote ProjectionProvider, SweepConfig and VersionStore into internal/store so sweep capabilities are backend-neutral'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** the ticket's, with one placement decision made — `ProjectionProvider`
and `SweepConfig` go into `internal/store` beside `VersionService`, not a new
package. That is where the other optional-capability contract types already live
(`DerivedObjectSpec`, `ReconcileOptions`, the version DTOs), so it adds no
package and no import-graph edge.

OUT, restated: implementing any of this for a second backend. This closes the
seam; it does not use it.

**Acceptance Criteria:** the ticket's five. AC-2 is the one that actually proves
the work — a test double satisfying the interfaces from a package that does not
import pgstore.

## Research

- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant rela concepts for prior art
- [x] ~~/research~~ (N/A: the design was settled in TKT-415WA7's review)
- [x] ~~External libraries~~ (N/A)
- [x] ~~Reference implementations~~ (N/A — in-tree precedent is the model)

**Existing Solutions / survey findings.** Three things the survey established
that change the shape of the work:

1. **`*pgstore.VersionStore` already satisfies `store.VersionService`** — it is
asserted at `version_store.go:47`. So that half is a signature change, not a
type move.
2. **`ProjectionProvider` and `SweepConfig` have no pgstore dependencies.**
One is a single-method interface; the other is a struct of four durations plus
an int. Both move cleanly.
3. **The blast radius outside pgstore is one file** —
`internal/appbuild/versionsweep_postgres.go`. Everything else referencing these
types is inside pgstore itself.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- Move `ProjectionProvider` and `SweepConfig` to `internal/store`, and keep
**type aliases** in pgstore (`type SweepConfig = store.SweepConfig`) so
pgstore's internal call sites and any external caller keep compiling. Aliases,
not new named types — a named type would make `pgstore.SweepConfig` and
`store.SweepConfig` non-interchangeable and reintroduce the coupling in a
subtler form.
- Change `Store.VersionStore()` to return `store.VersionService`. The concrete
type already satisfies it, so this is a return-type widening.
- `StateStoreFor`: **re-check the plimsoll rationale rather than assume it.**
The doc argues for a package function over a `Store` method to avoid growing the
method set. That argument still holds — but it does not require the RETURN type
to stay concrete. Change it to `state.KV` and let appbuild assert a
consumer-side `stateStoreProvider` interface instead of calling the pgstore
function, which is what actually removes the coupling.

**The nil guards stay, and that is the point of the ticket.** TKT-415WA7's
review found that widening discovery while leaving return types concrete opened
a nil-contract hole: a nil pointer boxed into an interface is non-nil, so every
downstream nil-check passes and the panic lands at write time. Promoting the
return types makes that hole *more* reachable, not less — any implementation may
now return nil. `TestCapabilityPresentButHandleNilYieldsUntypedNil` must keep
passing, and the guards must not be "simplified" away as redundant.

Alternatives rejected:

- **A new `internal/store/versioning` package.** Groups the contract, but costs
an import for every consumer to solve a problem `internal/store` does not have —
it is already the home for capability contract types.
- **Aliasing from `internal/store` back to pgstore definitions.** Smallest
diff, but a second backend would still transitively import pgstore, so AC-2
would fail. It looks like decoupling without being it.
- **Adding accessor methods to `pgstore.Store`.** Forbidden by its plimsoll
directive, which explicitly says a third capability accessor must not raise the
count.

**Files to modify:** `internal/store/store.go` (+ the two types),
`internal/store/pgstore/sweep.go` (aliases), `version_store.go` (return type),
`statekv.go` (return type), `internal/appbuild/versionsweep_postgres.go`
(interfaces name store types), plus a new test double for AC-2.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** none new. This is a type-visibility change; no
new input reaches the process, no wire format changes, no parsing added.

**Security-Sensitive Operations:**

- **`state.NewValidatedKV` must keep wrapping the raw KV.** It applies the key
rules FSKV gets from RootedFS; if `stateKVFor` starts obtaining the handle
through an interface, the wrapper must still be applied at the same place.
Dropping it would let a caller write arbitrary keys. Asserted by the existing
test that the project directory stays clean.
- **Widening a return type does not widen privilege.** An interface admits an
implementation that has the methods; it grants no capability the store did not
already have, and the build tags still decide which backend is compiled.
- **ACL is untouched** — read gating lives in `internal/visibility` decorators
at the wiring site, never in a store (DEC-ZBI39P).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Pass condition |
|----|------|----------------|
| 1 | `grep pgstore\. internal/appbuild/*.go` | the capability interfaces name no pgstore types |
| 2 | test double implementing all three capabilities, in a file that does not import pgstore | each resolver reaches it |
| 3 | `just plimsoll` | pgstore's directive unchanged |
| 4 | postgres conformance + appbuild wiring against a live DB | pass unchanged |
| 5 | `just arch-lint`, `just lint --build-tags postgres` | clean |

AC-2 is the real test and the reason the others are not sufficient: it is the
only one that would fail if the promotion were cosmetic.

**Edge Cases:** the typed-nil path on every widened return (a provider handing
back a nil pointer must still yield an untyped nil interface); an implementation
satisfying `VersionStore()` but not the sweep, and vice versa — the interfaces
stay separate so partial adoption works.

**Negative Tests:** a store implementing none of the capabilities still gets the
FSKV / in-memory fallbacks; a capability returning nil does not produce a
non-nil interface.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (m)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Promoting return types re-opens the typed-nil hole | **Medium** | The guards stay and `TestCapabilityPresentButHandleNilYieldsUntypedNil` must keep passing; widening makes the hole more reachable, not less |
| Using named types instead of aliases silently re-couples | Medium | Aliases only, so `pgstore.SweepConfig` and `store.SweepConfig` remain the same type |
| `state.NewValidatedKV` dropped while moving the state handle | Medium | Existing test asserts the project dir stays clean through the KV |
| Cosmetic promotion that passes greps but not reality | Medium | AC-2's double lives in a file that does not import pgstore, so a transitive dependency fails to compile |
| pgstore's method set grows | Low | plimsoll directive is a hard gate; the approach adds no methods |

**Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A user-facing — internal refactor, no behaviour change.

In-code: the `NOTE:` comments in `versionsweep_postgres.go` that currently say
"the signature still names pgstore types … tracked separately" become false and
must be replaced, not left. `StateStoreFor`'s doc keeps its plimsoll rationale
(still valid) but stops implying the return type must be concrete.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the design came out of TKT-415WA7's review, which
identified this work and its one real hazard — that promoting return types makes
the typed-nil path reachable by more implementations, so the guards become more
load-bearing rather than less. That is carried into the Approach and the risk
table. A fresh review runs against the finished diff.
