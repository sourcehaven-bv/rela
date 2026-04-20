---
id: REV-M40S7
type: review-checklist
title: 'Review: Replace backend per-file coverage ratchet with package floors; add govulncheck + gosec CI gates'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — CI Test job SUCCESS
- [x] Lint clean (`just lint`) — CI Lint job SUCCESS (0 issues locally)
- [x] Coverage maintained (`just coverage-check`) — total 71.8% vs 65% floor; all package floors satisfied

## Code Review

- [x] Run `/code-review` command (invoked cranky-code-reviewer agent)
- [x] All critical review-responses addressed (4 of 4)
- [x] All significant review-responses addressed (0 raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Critical (all addressed):
- RR-VGS0D — gh issue list search now uses quoted phrase + jq equality
- RR-CMTD1 — labels pre-created via `gh label create --force`
- RR-M4AD2 — floors relaxed to ~5pp headroom (dataentry 60→55, entity 85→80, project 85→80)
- RR-I57P0 — explicit pipefail + temp-file pattern (no pipe dependency on implicit shell flags)

Minor (addressed):
- RR-CL9FG — justfile stale comment fixed
- RR-LYFQJ — post-merge-sync file-existence guard added

Nit:
- RR-GFLRY — CLAUDE.md floor wording tightened (addressed)
- RR-UC0SQ — heredoc hardened against shell expansion in log body (addressed)
- RR-YLD0H — store-floor addition deferred to follow-up

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All 9 ACs verified PASS per IMPL-2RG24 evidence table. CI
on PR #483 also confirms: Vulnerability Check SUCCESS, no Coverage Baseline
Guard job runs (removed), no Codecov comment.

## Documentation (enhancements only)

N/A — refactor ticket. CLAUDE.md updates captured inline.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (pending final Rela Tickets re-run after status transitions)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/483
