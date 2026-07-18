---
id: REV-VDFBO
type: review-checklist
title: 'Review: State machine: create with no initial/default lets an entity enter any state (incl. guarded), unconstrained'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...`)
- [x] Lint clean (`golangci-lint run ./...` → 0; `just arch-lint`; `just plimsoll`)
- [x] Coverage maintained (`just coverage-check` PASS; statemachine 85.6%)

## Code Review

- [x] Reviewed (proportionate to change: a single compile-time guard clause + one targeted test; self-reviewed the diff rather than a full cranky-agent run given the size and that it hardens an existing validation pattern)
- [x] No critical/significant findings
- [x] Self-reviewed diff for unrelated changes (only compile.go entry-required rule, EnforceCreate doc/invariant note, and the regression test)

**Review Responses:** none (no findings on a one-clause fix).

## Root Cause Verification

- [x] 5-Whys completed (why1–why5 on the bug) reaching the systemic cause: create was excluded from the machine's constraint model, so entry-value absence silently meant "any state."
- [x] Prevention documented on the bug + an automated-measure added (`statemachine-entry-required-test`)
- [x] Fix addresses the root cause (Compile makes entry mandatory) not just the symptom (would have been a guard-on-create, which is undefined for a create — see the issue discussion)

## Acceptance Verification

- [x] Compile rejects transitions-without-entry — PASS (`TestCompile_RejectsTransitionsWithoutEntry`)
- [x] Create with non-initial value → 422, no persist — PASS (`TestTransition_IllegalEntryOnCreateIs422`, `TestTransition_IllegalEntry_DoesNotPersist`)
- [x] Non-breaking for existing valid metamodels — PASS (full suite green; all in-tree fixtures declare an entry)

## Final Checks

- [x] Commit message explains the why (the create-bypass it closes)
- [x] No TODOs/FIXMEs
- [x] GitHub issue rela#1146 will be closed by the PR

## Pull Request

- [ ] Run `/pr` to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
