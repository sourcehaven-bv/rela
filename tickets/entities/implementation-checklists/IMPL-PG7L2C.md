---
id: IMPL-PG7L2C
type: implementation-checklist
title: 'Implementation: lua: extend rela.bypass_acl''s admin handle with read methods (elevated reads, closure-scoped)'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the capability is inert
without an `allow_acl_bypass` automation action + an `ElevatedProvider` Mutator;
the two-key gate is exercised by the automated tests at both layers)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Verified by **mutation testing** — each mutation applied to production code,
confirmed the intended test fails, then reverted:

| Mutation | Caught by |
|----------|-----------|
| Route elevated reads through `VisibleReader` (the plausible "they're both readers" tidy-up) | 5 tests |
| Drop the liveness guard from the read methods | `TestElevatedRead_HandleInvalidatedAfterClosure` |
| Fall back to the gated reader when `ElevatedReader` is nil | `TestElevatedRead_DeniesWhenNoReaderWired` |
| Grant the reader outside the two-key branch | 3 deny rows of `TestRun_ElevationRequiresBothKeys` |
| Record the audit row only on the success path | `TestElevatedRead_AuditsEvenWhenClosureRaises` |
| Remove per-closure dedup (record per read) | `TestElevatedRead_AuditsOncePerClosure` |
| Emit a record when no reads happened | 2 tests |
| Return concrete `*ElevationRecorder` instead of the interface (typed-nil trap) | `TestNewElevationAuditor_NilSinkYieldsNilInterface` |

The typed-nil mutation initially **survived** — the first version of that test
compared the return value directly, and `nil *T != nil` is false, so it passed
against the very defect it targeted. Rewritten to assert through the
`lua.WriteDeps` field the value actually lands in; it now fails correctly.

Full suite green except `TestScriptReadSeam_PolicylessProjectStaysUnrestricted`,
which fails identically on a pristine `develop` worktree (pre-existing, fixed by
PR #1228). Race detector clean on all touched packages. `just lint`, `just
arch-lint`, `just plimsoll`, `just lint-md`, coverage floors: all pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the elevated read bindings deliberately
do NOT share code with the gated `luaGetEntity`/`luaListEntities`/
`luaGetRelations`: those funnel through `r.reader()` (which resolves
`VisibleReader`), and a shared helper parameterized by reader would be one edit
away from letting a gated binding read raw. Extracted `elevatedRelationQuery`
(shared option-table parsing) and `readUsage.mark` where it did sharpen the
contract.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Architecture notes

- `plimsoll` flagged `Runtime` at 125/120 methods. Resolved by making the four
new read builders + `recordElevatedReads` **package functions** taking a
`ctxFn`, rather than raising the directive — the guidance says prefer splitting
the type.
- `arch-lint` rejected `appbuild → principal`. The audit adapter moved to
`internal/audit` (which already depends on `principal` and owns the record
shape); it satisfies `lua.ElevationRecorder` structurally, so `audit` gains no
dependency on `lua`. **No arch-lint rule additions were needed.**
- `visibility.Unrestricted` names the new ungated path, so
`grep -rn "visibility.Unrestricted"` still enumerates every one (TKT-1WV50C).
