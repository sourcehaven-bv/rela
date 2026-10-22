---
id: PLAN-Z60AAU
type: planning-checklist
title: 'Planning: Triage semgrep findings from scheduled security scan (GH #1923)'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the 10 semgrep findings of GH #1923 (p/default, severity ERROR).
Out: other semgrep severities, CodeQL alerts, gitleaks and osv-scanner findings.

**Acceptance Criteria:**
1. A local `semgrep scan --config p/default --severity ERROR` (1.181.0, as the workflow runs it) reports zero unsuppressed results.
2. Each suppression is a line-level `nosemgrep: <rule>` with a reason that names why the input is trusted.
3. The three workflow findings are fixed, not suppressed.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: triage of scanner findings)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: GitHub documents the env-var fix for run-shell-injection)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The workflow header documents `nosemgrep` and
SEMGREP_EXCLUDE_RULES. The exec call sites already carry trust comments
(`#nosec` in commands.go, an inline note in cmdexec.go). TKT-CATTFF added the
scan.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- run-shell-injection (release.yml x2, security.yml): replace `${{ github.* }}` in `run:` with the runner's `$GITHUB_REF_NAME`, `$GITHUB_SERVER_URL`, `$GITHUB_REPOSITORY`, `$GITHUB_RUN_ID`, quoted.
- dangerous-exec-command (cmdexec.go, dataentry/commands.go): false positive, `nosemgrep` with reason (operator-configured command).
- pgx-sqli (docscapture/scratch_postgres.go x2, pgstore/listener.go): false positive, identifiers are random hex or a constant and are quoted; DDL and LISTEN take no bind parameters.
- missing-unlock-before-return (lock/memlock.go, kvpiles/kv.go): false positive, the unlock is in the returned release closure.

Alternative rejected: SEMGREP_EXCLUDE_RULES. Each rule is useful elsewhere in
the codebase, so line-level suppression keeps the rules active.

**Files to modify:** .github/workflows/release.yml,
.github/workflows/security.yml, internal/cmdexec/cmdexec.go,
internal/dataentry/commands.go, internal/docscapture/scratch_postgres.go,
internal/store/pgstore/listener.go, internal/lock/memlock.go,
internal/piles/kvpiles/kv.go

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Workflow: `github.ref_name` is a tag or branch
name (needs push access). Exec: commands come from project config; requests only
pick a registered key. SQL: identifiers are minted from crypto/rand hex or a
package constant.

**Security-Sensitive Operations:** Process execution (confined by cmdexec;
data-entry commands gated by authorizeCommand), DDL on a scratch schema, LISTEN.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: no behavior change in Go code; workflows checked with actionlint)

**Test Scenarios:**
1. Local semgrep run shows 10 results, all with a suppression.
2. actionlint passes on the edited workflows.
3. Go lint and tests pass for the touched packages (comment-only changes).

**Edge Cases:** `nosemgrep` must sit on the reported line (or the line before)
to take effect; verified by the local run.

**Negative Tests:** N/A: no new code paths.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A suppression could hide a later real injection on the same line.
Mitigation: each reason names the trust assumption, so a reviewer sees when it
stops holding. Effort: s.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, no user-facing docs)

**Documentation Impact:** N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: scanner triage with no design choices; code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
