---
id: PLAN-QZN4166
type: planning-checklist
title: 'Planning: Lint: no comments in function bodies (Go and TS)'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: Go and TS gates for comments inside function bodies and a 10-line
doc cap, per-file baselines, just/CI wiring, rules doc. Out: doc comments on
declarations, test files, generated files, Vue templates.

**Acceptance Criteria:**
1. A comment inside a function body fails the gate (unit test per language).
2. A directive or `why: <reason>` comment passes (unit test).
3. A doc comment over 10 prose lines fails (unit test).
4. A count above the baseline fails; below prints a note (compare test).


## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small tooling change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A for small changes

**Existing Solutions:** No linter bans comments by position: ESLint
no-inline-comments and golangci-lint godot do not. Reused pattern:
bare-strings.json ratchet and archguard allowlists.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Go analyzer under tools/commentbody (go/parser with
comments). Local ESLint rule plus runner script. Baselines per file, may only
shrink.

**Files to modify:** tools/commentbody/, frontend/eslint-rules/,
frontend/scripts/comment-body.mjs, justfile, ci.yml, CLAUDE.md,
.claude/rules/code-comments.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Source files only, parsed by go/parser and the
ESLint parsers. A parse error fails the run.

**Security-Sensitive Operations:** None. Read-only, runs in CI, no network, no
secrets.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Table tests in tools/commentbody/scan_test.go and
frontend/eslint-rules/no-body-comments.test.ts.

**Edge Cases:** Closures, nested closures counted once, generated files,
directives, bare why:, tag-only JSDoc.

**Negative Tests:** New comment above baseline fails; bare why: fails.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Stale baselines after develop moves (mitigated: lower counts only
note). why: becoming a reflex (reviewed in PRs).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal tooling, documented in .claude/rules/code-comments.md)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] CLAUDE.md - rules table row
- [x] .claude/rules/code-comments.md

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-JLKY6L8, RR-F4UREG8, RR-7FSXSFE, RR-S1GT3AN,
RR-5DBL2FQ
