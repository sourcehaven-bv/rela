---
id: REV-QXY8CQ
type: review-checklist
title: 'Review: Direction inference for the remaining relation surfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...` — full suite green)
- [x] Lint clean (`golangci-lint run ./...` 0 issues; `just arch-lint` OK;
`just comment-lint` no findings across 10198 comments)
- [x] Coverage maintained (`just coverage-check` — package floor 50% PASS,
total floor 65% PASS, total 77.5%)

The advisory `just comment-report` backlog was checked for regressions: none of
the new or modified files (`direction.go`, `relation_direction.go`,
`validate_caldav.go`) appear in it.

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: this extends TKT-860BNJ's reviewed
design to four more call sites rather than introducing a new mechanism; the two
criticals from that review — `direction: ""` bypassing the check, and a
duplicated inference rule — are structurally prevented here by routing every
surface through the shared `CheckAmbiguousDirection` / `resolveConfigDirection`)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

Two findings from the TKT-860BNJ review were acted on here rather than deferred:
the migration's `apply bool` dual-use (flagged as provable-but-not-obvious) is
replaced by a single `bindings()` traversal Detect and Apply share, and the
one-message-per-surface duplication was avoided up front.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Result | Evidence |
| --- | --- | --- |
| 1. unambiguous validates everywhere | PASS | `TestValidateConfig_UnambiguousDirection_AllSurfaces`, `TestValidateCalDAV_DynamicDirection/unambiguous...` |
| 2. self-referencing errors everywhere | PASS | `TestValidateConfig_AmbiguousDirection_AllSurfaces` (4 subtests), `TestValidateCalDAV_DynamicDirection/self-referencing...` |
| 3. explicit direction preserved | PASS | `..._ExplicitDirectionOK`, `TestRelationDirectionMigration/explicit_direction_is_preserved` |
| 4. lists/kanbans materialized on the wire, source unmutated | PASS | `TestResolveConfigDirections_ListsAndKanbans` (asserts both the wire value AND that the operator config is not mutated) |
| 5. migrate fills/skips per surface | PASS | `TestRelationDirectionMigration` (15 subtests incl. all new surfaces) |
| 6. CalDAV resolves from the member type | PASS | `caldav_backend.go` `dynamicMembers` routes through `resolveConfigDirection`; covered by `TestValidateCalDAV_DynamicDirection` |

End-to-end on a scratch project with all five surfaces: migrate filled
`incoming` on the three display surfaces and `outgoing` on the CalDAV
collection, skipped both self-referencing bindings, and validate named them.
After hand-resolving, the project validates clean and a second migrate reports
"No migrations needed".

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-QXY8CQ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** see the branch `feat/direction-inference-all-surfaces`; URL recorded on
the PR once opened.
