---
id: REV-DV3ROC
type: review-checklist
title: 'Review: lua: ReadDeps reads through visibility.Reader + visible tracer; scheduler jobs get explicit AllowAllReader; prove one role-scoped job'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — full sweep green except `internal/docscapture` (pre-existing browser-env failure)
- [x] Lint clean — golangci-lint 0 issues; `just arch-lint` OK; `just plimsoll` OK; gofmt clean
- [x] Coverage maintained — no floors touched

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (3)
- [x] All significant review-responses addressed (4)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** ten findings, all resolved.

**Critical (3)** — the review earned its keep here; each was a real defect I had
missed:
- **RR-ZA452J** `rela.md.entity_refs` kept `context.Background()`, so with a policy configured the binding fail-closed for *every* user and returned an empty map silently. Fixed to `callerCtx()`.
- **RR-QS4WQV** No test constrained any production wiring site — reverting all three to the raw store passed the entire suite. My own reported mutation had been applied to the test's own fixture, which is self-referential and proved nothing about production. Added black-box wiring tests in dataentry and appbuild plus helper tests for the cascade path; each mutation now fails.
- **RR-KYWIMZ** The AC6 "data-destruction guard" never invoked `rela.update_entity` — the exact regression it claimed to pin passed silently, while both its godoc and a new CLAUDE.md rule cited it as protection. Rewritten to run the real binding through a persisting mutator and assert on stored state; verified failing on the mutant.

**Significant (4):** RR-CCBZBH (Bind wired into ScriptReader — one `acl.Request`
per read instead of ~21 membership walks per list call), RR-GKCZO5 (added
`DenyReader`/`DenyTracer`; unattended paths now refuse rather than silently
reverting to full-graph reads, dataentry deliberately keeps the permissive
fallback), RR-7408F5 (scheduler field-redaction limitation stated plainly
instead of "never wrong"), RR-FJWIMT (`run_as` documented for operators in
scheduled-tasks.md + lua-scripting.md).

**Minor (3):** RR-QSP6X2 (fault-vs-deny logging in search), RR-3U1V80
(allocation godoc corrected to name the per-row and per-endpoint costs),
RR-R0G3DF (deferred to TKT-1WV50C with rationale — the risk it mitigates is now
covered by the wiring tests).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1–AC9, AC11, AC13 PASS. AC10/AC12 split to TKT-76JP2A
by user decision — though the appbuild wiring test added for RR-QS4WQV *does*
deliver AC12's substance: a task under `system:reports`, granted only `ticket`
in acl.yaml, provably cannot read a `secret`.

**Mutation verification** (the thing the first round lacked): repointing
dataentry's `VisibleReader` at the raw store fails 2 tests; the tracer mutation
fails 1; the appbuild scheduler mutation fails 2; pointing `luaUpdateEntity` at
`VisibleReader` fails the AC6 guard. Every gate is now pinned by a test that
fails when it is removed.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
