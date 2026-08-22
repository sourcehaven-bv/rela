---
id: REV-CB5T2K
type: review-checklist
title: 'Review: CheckboxWidget is unstyled — the only widget with no design tokens'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Frontend: **1828 tests / 113 files pass** (up from 1824 — this change adds 4:
two `it.each` arms of the hook test, the edit-arm negative, and the cascade
test). `vue-tsc --noEmit` clean. ESLint **0 errors**.

Go: `just lint` **0 issues**; `just comment-lint` **no findings across 10183
comments**; `go build ./...` OK; `internal/dataentry` tests pass.

Coverage: no Go code changed, so no package floor moves. The frontend has no
coverage enforcement by design (`CLAUDE.md`: "unit tests run plain").

`npm run format:check` reports 128 pre-existing files repo-wide; **neither
file in this diff is among them**, and it is not a CI gate.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** [[RR-CBC1XZ]] (critical, addressed), [[RR-CBS2QW]]
(significant, addressed), [[RR-CBS3AC]] (significant, addressed),
[[RR-CBM1TS]] (minor, addressed), [[RR-CBM3TR]] (minor, addressed),
[[RR-CBM5OP]] (minor, addressed), [[RR-CBM2SL]] (minor, wont-fix),
[[RR-CBLEV8]] (minor, deferred), [[RR-CBM4WP]] (nit, addressed).

The critical finding was a **real defect in my own first draft**, not a false
positive: `.display-checkbox:disabled` lost the cascade to
`input[type='checkbox']:disabled`, so every read-only boolean rendered
`not-allowed` — the precise opposite of the comment sitting above it, and a
regression against the pre-change code. It was confirmed independently in a CSS
engine before being fixed, and it is now reproduced by a test that fails when
the guard is removed.

Diff contains no unrelated changes: two files, both named in the planning
checklist. The demo-data write produced by live verification
(`TKT-001.md` gaining `is_blocked: true`) was reverted; the tree is clean.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 (visual consistency) — PASS.** Live computed style: `appearance: none`,
  18×18, `border-radius: 4px` (`--radius-sm`), `cursor: pointer`. Screenshotted
  beside its grid neighbours.
- **AC2 (checked state follows the theme) — PASS.** Fill resolves to
  `#6f93ff` (dark) and `#4772fb` (light) — both the live `--accent-color`.
  Checkmark renders 5×10 at 45°. The focus ring now also resolves from the
  accent (`color(srgb 0.435 0.576 1 / 0.3)`) rather than the indigo literal
  the first draft copied.
- **AC3 (display arm stays legible as read-only) — PASS, and materially
  stronger than at first submission.** Verified in a real browser against the
  compiled stylesheet: display arm `cursor: default` / `opacity: 0.6`;
  ACL-denied edit arm `not-allowed` / `0.6`; live edit arm `pointer` / `1`.
  The first draft failed this criterion silently — see [[RR-CBC1XZ]].
- **AC4 (no behaviour regression) — PASS.** Full suite green; a live toggle
  persisted to disk through the autosave PATCH.

Scope confirmed live: the 4 markdown task-list checkboxes rendered in the same
DOM kept `appearance: auto`, so the scoped rule did not leak.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** [[DOCS-CB3N8V]]

No user-facing docs *changed*: every checkbox mention in `docs/data-entry.md`
was read and each describes widget selection or value semantics, not
appearance. Recorded in the docs checklist with the reasoning rather than
waved through as "N/A".

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1405

**Local `just test` note.** The full race-enabled Go suite fails locally on
`TestAnalyzeProperties_StopsScanningAtCap` (`internal/dataentry`) — a 10-minute
`testing` timeout after the test ran 8m20s, not an assertion failure. This
branch changes **zero Go files** (`git diff --stat develop...HEAD -- '*.go'` is
empty), so it cannot be the cause; the test dates to #1337 and is being
reproduced on a clean `develop` worktree to confirm. CI is the independent
verdict and is authoritative here.
