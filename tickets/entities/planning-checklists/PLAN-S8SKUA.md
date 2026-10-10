---
id: PLAN-S8SKUA
type: planning-checklist
title: 'Planning: Go 1.26.9 and x/net v0.60.0 for govulncheck findings'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Toolchain go1.26.9 in go.mod and all pinned workflow go-version
values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck` reports no
actionable vulnerabilities; CI green. Risk: a patch release can change
behaviour; mitigated by the full CI run. No security-relevant inputs change.
Docs: N/A.

**Acceptance Criteria:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: dependency bump)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows PR #1338, the previous toolchain bump)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.

**Existing Solutions:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.

**Files to modify:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Toolchain go1.26.9 in go.mod and all pinned
workflow go-version values; golang.org/x/net v0.60.0. Acceptance: `just
govulncheck` reports no actionable vulnerabilities; CI green. Risk: a patch
release can change behaviour; mitigated by the full CI run. No security-relevant
inputs change. Docs: N/A.

**Security-Sensitive Operations:** Toolchain go1.26.9 in go.mod and all pinned
workflow go-version values; golang.org/x/net v0.60.0. Acceptance: `just
govulncheck` reports no actionable vulnerabilities; CI green. Risk: a patch
release can change behaviour; mitigated by the full CI run. No security-relevant
inputs change. Docs: N/A.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.

**Edge Cases:** Toolchain go1.26.9 in go.mod and all pinned workflow go-version
values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck` reports no
actionable vulnerabilities; CI green. Risk: a patch release can change
behaviour; mitigated by the full CI run. No security-relevant inputs change.
Docs: N/A.

**Negative Tests:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Toolchain go1.26.9 in go.mod and all pinned workflow go-version
values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck` reports no
actionable vulnerabilities; CI green. Risk: a patch release can change
behaviour; mitigated by the full CI run. No security-relevant inputs change.
Docs: N/A.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: version bump, no design)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** Toolchain go1.26.9 in go.mod and all pinned workflow
go-version values; golang.org/x/net v0.60.0. Acceptance: `just govulncheck`
reports no actionable vulnerabilities; CI green. Risk: a patch release can
change behaviour; mitigated by the full CI run. No security-relevant inputs
change. Docs: N/A.
