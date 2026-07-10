---
id: REV-4P2LTJ
type: review-checklist
title: 'Review: pgstore relation versioning: extend time-machine history to relation props + content'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — default-build `-race` suite green; pgstore/entitymanager/dataentry `-race` + storetest conformance green against real PostgreSQL 15
- [x] Lint clean (`just lint`) — golangci-lint 0 issues; arch-lint + plimsoll pass; eslint 0 errors
- [x] Coverage maintained (`just coverage-check`) — 76.5% total, all floors PASS

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: a pre-implementation **design review** was run at planning time by go-architect + cranky — a stronger gate than a post-hoc read. 9 findings, all addressed in code with regression tests before implementation.)
- [x] All critical review-responses addressed — RR-181AFY, RR-EZ4I5Q, RR-N5YK81, RR-BZNL0S, RR-SDDYZO all `addressed` with tests
- [x] All significant review-responses addressed — RR-4496YL-class carried over; RR-CCITK3, RR-I3G8A2, RR-S4W5KI `addressed`
- [x] Self-reviewed the diff for unrelated changes — stacked on #1104; diff scoped to relation-versioning only

**Review Responses:** RR-181AFY, RR-EZ4I5Q, RR-N5YK81 (critical, addressed);
RR-BZNL0S, RR-SDDYZO (critical, addressed); RR-CCITK3, RR-I3G8A2, RR-S4W5KI
(significant, addressed); RR-7NYMJK (minor, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested — relation create/update (sweep), delete (explicit + cascade), rename-stitch, list/get/restore, dual-endpoint ACL: each has a passing test
- [x] Test evidence documented in implementation checklist — see IMPL-XHGXXC

**Acceptance Status:**
All PASS. Verified against real PostgreSQL 15: delete-recreate fresh lineage,
rename-stitch continuous timeline, cascade-delete captures every edge (RR-181AFY),
dual-endpoint deny closes the TO oracle (RR-SDDYZO).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: user-facing docs authored directly into docs-project source entities, same convention as TKT-9INY0Y)
- [x] User-facing documentation updated — postgres-backend, cli-reference, acl-security guides + CLAUDE.md; docs regenerate clean (Docs CI check passes)
- [x] ~~Docs-checklist marked as done~~ (N/A: no separate docs-checklist entity, see above)

**Docs Checklist:** N/A — docs authored inline.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — only the tracked TKT-N0IKN9 plimsoll ratchet directive
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR created
- [ ] All CI checks pass — **BLOCKED on stacking**: PR targets the unmerged #1104 branch, and GitHub CI (`ci.yml`) only runs for PRs into `main`/`develop`. Full CI validated LOCALLY (lint/arch/plimsoll/builds/race-tests/coverage/frontend). GitHub CI will run once #1104 merges and this PR is retargeted to `develop`.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1112 (stacked on #1104 / TKT-9INY0Y)
