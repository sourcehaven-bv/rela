---
id: PLAN-GEDCKQ
type: planning-checklist
title: 'Planning: Scheduled security scan: gitleaks, semgrep, osv-scanner'
started: "2026-10-18"
status: done
completed: "2026-10-18"
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: one scheduled workflow (gitleaks, semgrep, osv-scanner), SARIF upload to code scanning, one tracking issue per tool, `.gitleaksignore` for existing false positives.
Out: any PR-path check (deliberately, see `security.yml`), fixing the findings themselves, and changes to `security.yml`.

**Acceptance Criteria:**
1. Weekly and `workflow_dispatch` only. Test: the `on:` block has no `pull_request`.
2. SARIF per tool in code scanning. Test: first run shows alerts under categories gitleaks, semgrep and osv-scanner.
3. One issue per tool, updated and closed. Test: run with findings creates the issue; a re-run edits it; a clean run closes it.
4. No gitleaks findings for known false positives. Test: local and CI gitleaks report 0.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small CI change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: tools are used with their documented CLI)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: no product concept involved)

**Research Doc:** N/A

**Existing Solutions:**
- `security.yml` (scheduled govulncheck, dedupe-by-title issue) and `fuzz-sweep.yml` (one issue per failure): same issue pattern reused.
- gitleaks-action needs a licence for organisation repos, so the release binary is used, checksum-verified.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
One job: checkout with full history, install pinned gitleaks/osv-scanner binaries (checksums verified) and semgrep via pipx, run each tool to SARIF, upload each to code scanning, then open/update/close one issue per tool with `gh`.
Alternative rejected: details in issues or artifacts. The repo is public, so those would disclose findings.

**Files to modify:**
`.github/workflows/security-scan.yml`, `.gitleaksignore`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
Triggers are schedule and `workflow_dispatch` only; no PR or issue text is interpolated into `run:`. Tool binaries are pinned and checksum-verified.

**Security-Sensitive Operations:**
Permissions are `contents: read`, `issues: write`, `security-events: write`; checkout uses `persist-credentials: false`. Finding details go to code scanning only; gitleaks runs with `--redact`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: no input handling)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
Run the workflow on the branch (temporary push trigger, removed afterwards) and check the issues and alerts.

**Edge Cases:**
Zero findings with no open issue: nothing happens. Zero findings with an open issue: the issue is closed. Unchanged count: body edited, no comment.

**Negative Tests:**
N/A.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Noise from semgrep `p/default`: limited to severity ERROR. Disclosure in a public repo: details only in code scanning.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: CI only)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, no user-facing docs)

**Documentation Impact:**
N/A. The workflow header documents behaviour and false-positive handling.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small CI chore; design checked against `security.yml`)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
