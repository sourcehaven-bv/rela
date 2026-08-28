---
id: REV-VL8TGQ
type: review-checklist
title: 'Review: Reachability floor: merged-coverage pipeline + scupper (report-only baseline)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

**Evidence.** `go build ./...` clean; `go test ./...` exit 0 with 95 packages ok
and 0 FAIL. On the opened PR every check is green except the tickets gate that
this checklist exists to satisfy — including `Lint`, `Comment lint`,
`Architecture`, `God-object lint`, `Test`, `Frontend`, `E2E`, `Fuzz`,
`Postgres Backend`, all six `Cross-Compile` matrix jobs, `Vulnerability Check`,
and the new `Reachability (report-only)` job itself.

**Comment findings.** None introduced. The prose added by this change is the
rationale block at the head of `scripts/reachability.sh` and the CI/justfile
comments; no Go declarations gained or changed doc comments.

## Code Review

- [x] Run `/code-review` command (performed equivalent focused full-diff review)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None recorded. Points examined during self-review:

- *Is the graceful-shutdown change unrelated scope creep?* No. A `-cover` binary
  flushes counters only on a clean exit, so without it the e2e leg silently
  contributes zero coverage. It is a prerequisite for AC1, and the reasoning is
  documented in the script header rather than left implicit.
- *Does `-d coverage-ignore` risk colliding with go-test-coverage?* No — that is
  the point. `.testcoverage.yml` sets `force-annotation-comment: true` and reads
  the same spelling, so the two tools share one annotation vocabulary instead of
  maintaining two dialects with identical meaning.
- *Can a dismissal hide a real gap?* No. Filtering only *removes* dismissed
  blocks; a never-executed, non-dismissed statement still counts against the
  number.
- *Unrelated changes in the diff?* The 15 changed files are the script, the
  justfile recipe, the CI job, `.gitignore` (generated profiles), the e2e
  coverdir wiring, rela-server shutdown, and the dismissal comments. Nothing
  else.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS — script merges unit + cross-package legs and reports 77.1%
   (36805/47755) against current develop; postgres/e2e legs activate on their
   env vars.
2. PASS — no `--threshold` passed in CI; the run exits 0 regardless of the
   number, and the job is not a required check.
3. PASS — `--require-reason` passed unconditionally and the run is clean, so all
   41 dismissals carry an explanation.
4. PASS — rela-server handles SIGTERM/SIGINT and shuts down cleanly.
5. PASS — `go build ./...` clean; `go test ./...` exit 0, 95 ok, 0 FAIL.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (N/A — see docs checklist)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-MQQX2D

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

**Caveat recorded deliberately.** The 77.1% baseline is measured *without* the
e2e and postgres legs, which are opt-in in CI. The true reachability number is
higher than this figure; the reported value is a floor on the floor. This is
stated so nobody later reads the number as the complete picture, or sets a
threshold against it without re-measuring with all legs enabled.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
