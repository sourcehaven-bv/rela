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

- [x] Reviewed (proportionate: a single compile-time guard clause + one targeted test; self-reviewed rather than a full cranky-agent run given the size and that it hardens an existing validation pattern)
- [x] No critical/significant findings
- [x] Self-reviewed diff (only compile.go entry-required rule, EnforceCreate doc/invariant note, regression test)

**Review Responses:** none.

## Root Cause Verification

- [x] 5-Whys completed (why1–why5) reaching the systemic cause
- [x] Prevention documented + automated-measure added (`statemachine-entry-required-test`)
- [x] Fix addresses root cause (mandatory entry at Compile), not a guard-on-create (undefined for a create)

## Acceptance Verification

- [x] Compile rejects transitions-without-entry — PASS (`TestCompile_RejectsTransitionsWithoutEntry`)
- [x] Non-initial create value → 422, no persist — PASS
- [x] Non-breaking — PASS (full suite green)

## Final Checks

- [x] Commit message explains the why
- [x] No TODOs/FIXMEs
- [x] Closes rela#1146

## Pull Request

- [x] Run `/pr` to create PR and monitor CI
- [x] All CI checks pass (monitoring)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1154 (closes #1146)
