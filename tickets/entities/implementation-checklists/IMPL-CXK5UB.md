---
id: IMPL-CXK5UB
type: implementation-checklist
title: 'Implementation: pgstore migrate and write advisory locks are not schema-qualified'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`TestXactAdvisoryLocksAreSchemaScoped` is the lock-level pin (both classes,
independent across schemas, still exclusive within one).
`TestMigrateDoesNotBlockAnotherSchema` and `TestWriteTxDoesNotBlockAnotherSchema`
are the integration-level pins — real `Migrate` / `Store.Tx`, real database, real
contention.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Reuses `newScopedPool` / `stubProvider` / `mkEntity`. The two lock-key constants
are restated in the test file because they are unexported; the tests that matter
drive production code rather than the literal.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran against a real throwaway PostgreSQL 15 instance (not a mock).

**Both regression tests verified to fail before the fix.** Production was
patched back to the bare one-key form:

```text
--- FAIL: TestMigrateDoesNotBlockAnotherSchema (10.51s)
    schema B's migration blocked on schema A's migrate lock — the key is not schema-scoped
--- FAIL: TestWriteTxDoesNotBlockAnotherSchema (10.17s)
    schema B's write Tx blocked on schema A's write lock — the key is not schema-scoped
```

Both pass after restoring (0.07s each).

Worth recording: the **first** version of these tests passed against the patched
code — a false negative. Holding only the scoped key space proves nothing,
because Postgres treats one-key and two-key advisory locks as disjoint, so the
bare-key regression grabbed a lock nobody held. Fixed by holding both spaces.
The fail-before check is what caught it; without it the measure would have been
decorative.

Full suite green: pgstore against real Postgres (`ok ... 19.5s`, conformance
harness + fuzz + tx stress), full default `go test ./...`, both build tags,
`just arch-lint`, `just docs-check`, `just coverage-check` (76.3%), markdown lint.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the pattern #1217 set for the sweep lock, deliberately: three locks with
one idiom beats two mechanisms doing the same job. An earlier draft introduced a
Go-side `advisorylock.go` deriving keys as `class<<32 | hash(schema)`; it was
dropped on rebase in favour of matching upstream. That is recorded in the bug
and the analysis checklist, since the reasoning matters more than the diff.

DRY: no helper extracted. Three call sites share a SQL idiom, not code, and
`hashtext(current_schema())` inline at each lock site is clearer than a
constructed string — the surrounding doc comments carry the rationale.

No silent failures: the sweep's lost-lock branch, previously a bare `return nil`,
now counts consecutive skips and warns after 10 that this schema's versions are
not being captured. Deliberately still non-fatal.

Debug/simulation code: the temporary bare-key patch used for fail-before
verification was confirmed removed by grep, and the full suite re-run after.
