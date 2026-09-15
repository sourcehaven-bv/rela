---
id: IMPL-D8DV43
type: implementation-checklist
title: 'Implementation: One scoped-read funnel for data-entry collection reads (the ACL verdict switch is copied four times)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`scopedread_test.go` covers the funnel directly: branch parity per narrowing
dimension, zero-verdict refusal, withheld, no-mutation-of-a-shared-ACL-query,
and the guard scan. The integration coverage is the existing data-entry suite,
which AC3 requires to pass unmodified — it exercises all four migrated surfaces
over HTTP, and a unit test of the funnel alone could not detect a migration that
dropped a narrowing at one call site.

Errors are surfaced, never degraded: a store error fails the whole read rather
than returning a partial slice, because a truncated collection read is
indistinguishable from a genuinely small one.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The branch-parity test is table-driven over narrowing dimensions rather than one
assertion per dimension, so a dimension added to `scopeRequest` without a row is
visible as a gap in the table.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verified by | Result |
| --- | --- | --- |
| AC1 one verdict switch | `TestScopedHeaders_IsTheOnlyVerdictSwitch` | PASS — regexp scan with four justified exemptions |
| AC2 both branches narrow | `TestScopedHeaders_NarrowingsApplyToBothVerdictBranches` | PASS |
| AC3 no behaviour change | full `internal/dataentry` suite | PASS unmodified except one helper, see below |
| AC4 zero-verdict defence | `TestScopedHeaders_ZeroVerdictIsRefused` | PASS |
| AC5 the copy survives | `TestScopedHeaders_DoesNotMutateTheACLQuery` | PASS |

**AC3's one exception, as the AC requires justifying:** `test_helpers_test.go`
changed by 7 lines. `rebindApp` restated the gantt handler's production wiring
by hand, so the new collaborator arrived nil and the tests segfaulted. Replaced
with the real `newGanttHandler` constructor. This does not weaken AC3 — it
removes a duplication of production wiring from a test helper, which is why the
test could go stale in the first place.

Independently confirmed in review: the security reviewer diffed all four
migrated sites against `3097c5a3^` and found no widening at any of them. Two
sites are TIGHTENED — the feed and the gantt AllowAll path gain world and face
narrowing they previously lacked, and `visibleEntitiesOfType` gains the
denied-world guard.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The guard test follows `ceilingguard_test.go`'s exemption-list pattern. The one
sanctioned duplicate (`planListPushdown`, which builds the same narrowed query
in the paged shape the store serves directly) is named in the funnel's doc with
the instruction to change both.

Two bugs were introduced and caught during implementation, both by pre-existing
tests, both recorded in the planning checklist's risk section: `Props` on one
verdict branch only (the exact bug class the funnel exists to prevent), and a
lost DenyAll early return that let a denied principal reach the search backend
(RR-X56H).
