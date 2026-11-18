---
id: IMPL-JNP204X
type: implementation-checklist
title: 'Implementation: Audit service seam and SQL audit log: in-transaction, hash-chained, verifiable'
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `sqlaudittest.RunAll` passes on sqlite and on a local postgres
(`pgaudit` with `RELA_TEST_DATABASE_URL`): append, verify, edits of each column,
deleted and relinked rows, forged anchor, prune (nothing, all, broken chain,
clock skew), import (atomic, refuses maintenance records), sink drain, 2500-row
paged verify, concurrent appends.
- Sink retry, hung database on Close and queue cap under `-race`.
- appbuild sqlite: entity writes land in `audit_log`, and the sink
survives a reassembly.
- CLI sqlite: import, prune, verify JSON, `--expect-head` (match, rewritten,
deleted newer rows, pruned), tamper detection.
- Manual: a sqlite build on a scratch project; create commands produced
rows, verify passed, a 30-day prune was refused, and editing `principal_user` at
seq 1 was reported.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
