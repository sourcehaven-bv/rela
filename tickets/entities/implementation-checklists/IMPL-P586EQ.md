---
id: IMPL-P586EQ
type: implementation-checklist
title: 'Implementation: Guarded copy into the bare face is refused unless the caller can already write the target'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The affordance test (`TestCopiesForSource_GuardedCopyIntoTheBareFaceIsOffered`)
is the full-flow one: it goes through `CopiesForSource` and then the write, and
asserts the two agree.

Edge cases: guard absent, guard held, guard not held, source unreadable,
cross-entity with a guard, and same-entity-but-same-face with a guard. The last
was missing from the first cut and is what code review caught.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no
interpolated values in these assertions)
- [x] Property comparisons use original object, not hardcoded strings

Reused the package's existing helpers rather than adding parallel ones:
`newCopyAuthzManager` gained two layered variants (`…WithGuard`, `…Full`) so
existing call sites are unchanged, and the copy-list tests reuse the existing
`withACL` option and `seedPage`.

Two test-design defects were found in review and fixed. A test asserting only
"some `*acl.ForbiddenError`" under `acl.ReadOnlyACL` proves nothing about
scoping, because that ACL denies everything with one fixed decision; the
cross-entity test now uses `createOnlyACL` against an existing target and
asserts `RuleKind`. And `denyReadGate` now records whether it was called,
because `ErrCopySourceMissing` is also what an absent source produces.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Every clause of the condition was mutation-tested by deleting it and confirming
a distinct test fails:

| Clause dropped | Test that fails |
| --- | --- |
| `guarded` | `TestCopy_IntoTheBareFaceNeedsUpdate` |
| `IsSameEntity()` | `TestCopy_GuardDoesNotOverruleACrossEntityWrite` |
| `sourceTail != targetTail` | `TestCopy_GuardDoesNotOverruleASameFaceCopy` |

The original bug was also reproduced against pre-fix code and confirmed fixed:

- `TestCopy_GuardIsTheAuthorizationForASameEntityCopy` pre-fix:
`got forbidden: this rela instance is configured read-only`.
- `TestCopiesForSource_GuardedCopyIntoTheBareFaceIsOffered` pre-fix:
`got Allowed=false reason="this rela instance is configured read-only"`.

The same-face regression was reproduced directly before fixing: under
`acl.ReadOnlyACL` with a permissive guard, a `from: ticket` / `to: ticket` copy
returned `err=<nil>` and left `status=escalated`. Confirmed it was a regression,
not pre-existing, by running the same probe against the old condition, which
correctly refused.

Full `go test ./...` clean. `just lint` 0 issues, `just arch-lint` no warnings,
`just comment-lint` clean, `just coverage-check` passing.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

On DRY: the guard expression is computed once (`perm` / `guarded`) and reused by
the exemption, so the "the guard already ran" invariant is structural rather
than two textual copies that could drift.

On security: this loosens a check, so the reasoning is recorded in the
`authorizeCopy` doc comment rather than left implicit, including what each
clause buys and what breaks without it. The exemption sits *after* the guard
check, so it is only reachable once the guard has passed. Check (1) is untouched
— and the comment now says plainly that it does not bound field-level
disclosure, which it never did. Cross-entity copies keep both the write check
and the per-edge authorization. Elevation is documented as an unsupported copy
path, since `bypassACL` would waive only check (3).
