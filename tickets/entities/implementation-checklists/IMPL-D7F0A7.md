---
id: IMPL-D7F0A7
type: implementation-checklist
title: 'Implementation: fail validate on partial reads'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (incomplete_test.go: per-call-site
propagation, file naming, the sharper pin where the violation sits on the
entity AFTER the unreadable one; validate_incomplete_test.go: exit codes)
- [x] Integration tests written — both suites drive `rela validate` end to end
- [x] Happy path implemented (`collectEntities` returns `([]*entity.Entity,
error)`; `IncompleteScanError` with `IsIncompleteScan` /
`IncompleteScanFiles` predicates)
- [x] Edge cases from planning handled (`errors.Join` so every unreadable file
is reported in one run, not just the first)
- [x] Error handling in place (partial data AND the error are returned, so a
summary can still render what it read while the gate refuses to call it a pass)

## Test Quality

- [x] Using table-driven subtests with t.Run
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Reproduced the original incident on this repo's own `tickets/` project by
restoring the unquoted scalar in `AM-feed-field-redaction.md`. Old binary:
`All validations passed.` / EXIT=0. New binary: EXIT=2, no success message,
offending file named. Healthy project still exits 0 with unchanged output.
- The fix caught a real error during its own review: a colon in this PR's new
ticket frontmatter failed to parse, and the new validator named the file and
exited 2 where develop's would have skipped it and printed success.
- Full suite: 112 packages, 0 failures. `golangci-lint`, `just arch-lint`,
`just comment-lint`, `just coverage-check` all green (total 79.7%).

## Scope note

`--check cardinality` alone scans only entity types that declare a bound, so
it cannot detect an unreadable file of an unconstrained type. Every constraint
it has IS evaluated over every subject it governs, so the claim it makes is
honest. Documented on `CheckCardinality` rather than papered over;
`--check properties` and `--check validations` both scan all types, which is
why CI runs all three.
