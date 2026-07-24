---
id: REV-3ZM7EB
type: review-checklist
title: 'Review: visibility: new internal/visibility package — Reader (PolicyReader/AllowAllReader) + tracer decorator + conformance suite'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test -race ./internal/visibility/...` green; full local `just test` fails ONLY in `internal/docscapture` (browser-env dependent, verified identical on a clean tree — pre-existing, unrelated); CI is the authoritative gate
- [x] Lint clean (`just lint`) — golangci-lint 0 issues on the package; `just arch-lint` OK; plimsoll OK
- [x] Coverage maintained (`just coverage-check`) — package at 94.2% against its new explicit 85 floor (RR-L3UQL4)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-MXKD2O, RR-J6022V)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-MXKD2O (significant — Bind helper + wiring-requirement
doc), RR-J6022V (significant — body-redaction scope corrected + TODO hook),
RR-MV5EM0 (minor — nil guards), RR-RT5YV8 (minor — arch-lint store allowance
removed), RR-JGVD30 (minor — swallowed-store-fault godoc), RR-L3UQL4 (minor — 85
coverage floor). All `addressed`. Reviewer also confirmed the core invariants
held under adversarial reading and endorsed the HasCycle documented residual.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All 10 PLAN-RR12W4 criteria PASS — each has a named
conformance case (see IMPL-D9V52D Verification Evidence for the AC → test
mapping). Plus post-review additions: NilElementsDroppedNotPanicked,
BindScopesOperation.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal package with no user-facing surface in PR 1 — per PLAN-RR12W4, docs-checklists land with PR 2/3 where behavior changes surface; CLAUDE.md package table + rules updated in this PR, godoc carries the contract)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A: same reason)

**Docs Checklist:** N/A (see above)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — the single `TODO(body-redaction)` is deliberate and tracked (RR-J6022V): it marks the future hook for policy-expressible body redaction, which is not expressible today
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
