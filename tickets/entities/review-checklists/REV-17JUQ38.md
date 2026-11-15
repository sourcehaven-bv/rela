---
id: REV-17JUQ38
type: review-checklist
title: 'Review: Archguard allowlist for unpaged list reads'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: test-only change in one package; `go test ./internal/archguard/` passes, repo-wide run left to CI to avoid loading the machine)
- [x] ~~Lint clean (`just lint`)~~ (N/A: ran `golangci-lint run --allow-serial-runners ./internal/archguard/...` on the only changed package: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: test files only; no production code changed)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0B26O0B, RR-4BI3RGG, RR-D2TGC2L, RR-EUNN5FP,
RR-GV4AI2P, RR-N7HVS5X, RR-QQMM3R4, RR-VS01Y2M, RR-X5FMH7N, RR-ZU973VA (all
addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestNoNewUnpagedReads` passes on the branch.
2. PASS: `TestScanUnpagedReads_FindsSyntheticRead` fails an unlisted read.
3. PASS: stale entries fail (seen on the real tree while moving helper entries to callers).
4. PASS: `TestUnpagedReadAllowlist_HasReasons`.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal test guard, no user-facing behaviour)
- [x] ~~User-facing documentation updated~~ (N/A: only the agent rule file `.claude/rules/collection-reads.md` changes)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the caller asked for a local commit only, no push or PR)
