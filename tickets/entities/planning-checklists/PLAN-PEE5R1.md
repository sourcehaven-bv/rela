---
id: PLAN-PEE5R1
type: planning-checklist
title: 'Planning: Triage osv-scanner findings from scheduled security scan (GH #1924)'
started: "2026-10-23"
completed: "2026-10-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In scope: the two findings in GitHub #1924 (GHSA-238p-pmpm-9mq7
katex, GO-2026-5932 x/crypto/openpgp). Out of scope: the Go toolchain (GitHub
#1945), the mermaid 12 major upgrade, and advisories published after the scan
(dompurify, storybook, vitest), which have their own Dependabot PRs.

**Acceptance Criteria:**
1. osv-scanner on `go.mod` and `frontend/package-lock.json` reports neither finding.
2. `npm ls katex` shows 0.18.2 or later.
3. Frontend unit tests, typecheck, lint and build pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: dependency triage)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows TKT-9YK7KP and TKT-D8WL4R)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:** katex comes only through mermaid 11, whose latest
release (11.17.2) still requires `katex ^0.16.47`. No 0.16.x backport of the fix
exists. GO-2026-5932 affects every x/crypto version; `go list -deps` under all
four build tags shows no `x/crypto/openpgp` import, and govulncheck reports it
at module level only (trace length 1).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** An npm `overrides` entry scoped to mermaid raises katex
to `^0.18.2`. Mermaid only calls `katex.renderToString` (MathML output), which
is unchanged in 0.18; the 0.18 breaking change is CSS class prefixes, and rela
loads no KaTeX CSS. A root `osv-scanner.toml` ignores GO-2026-5932 with a
reason. The govulncheck scripts need no entry, because they only fail on called
findings.

Alternative rejected: mermaid 12 requires katex 0.18, but it changes the default
layout and theme of every diagram.

**Files to modify:** `frontend/package.json`, `frontend/package-lock.json`,
`osv-scanner.toml` (new).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Math in mermaid labels comes from entity
markdown. Mermaid runs with `securityLevel: 'strict'` and sanitizes its output;
the upgrade removes the inherited-`trust` gadget.

**Security-Sensitive Operations:** None changed in rela code.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: dependency change; CI runs e2e)

**Test Scenarios:** Run osv-scanner 2.7.0 locally on both lockfiles; run `npm ls
katex`; run the frontend unit tests, typecheck, lint and build.

**Edge Cases:** Smoke test: `renderToString` with MathML output still renders
`x^2+\sqrt{y}`.

**Negative Tests:** With `Object.prototype.trust = true`, katex 0.18 does not
render an `\href` link.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Mermaid 11 is not tested upstream with katex 0.18. Mitigation: the
API it uses is unchanged and the smoke test passes. Effort: xs.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore)

**Documentation Impact:** N/A. The ignore reason lives in `osv-scanner.toml`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs dependency triage with no design surface)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** None.
