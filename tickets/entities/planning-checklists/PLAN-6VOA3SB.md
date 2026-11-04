---
id: PLAN-6VOA3SB
type: planning-checklist
title: 'Planning: Triage 2026-11-02 security scan findings (GH #2073, #2074, #2076)'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: the semgrep findings of GH #2073, the minimatch advisory of GH #2074 in
`e2e/package-lock.json`, the archplan `security.untraced` finding of GH #2076.
Out: changing how `security-scan.yml` uploads suppressed semgrep results, and
the other baselined `security.untraced` debt.

**Acceptance Criteria:**

1. Local semgrep 1.181.0 (`p/default`, severity ERROR) over the repo reports
zero results without a suppression.
2. `e2e/package-lock.json` resolves `minimatch` to >= 11.0.1; `npm run lint`
and `npm run typecheck` in `e2e/` pass and lint still checks the files.
3. `just archplan-security` reports no `security.untraced` finding for
`serveInboxSummary` (or `serveInbox`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small triage chore)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows in-repo precedent)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `nosemgrep` with a reason: `internal/piles/kvpiles/kv.go:117`,
`internal/lock/memlock.go:61` (TKT-D8WL4R).
- npm `overrides` for minimatch 11: `frontend/package.json` (TKT-MLH774W).
- Route coverage for archplan: the `SEQTRACE_SUITE=routes` block in
`tools/seqtrace/demo/run.sh`, which already appends config the perf project
lacks (commands, actions, transforms).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. semgrep: add `// nosemgrep: trailofbits.go.missing-unlock-before-return...`
above both returns in `filecomments.Store.lockWrite`, with the reason on the doc
comment. Alert #81 (cmdexec.go:387) is already suppressed in the SARIF (verified
locally: `suppressions` set). It is open only because
   #2030 moved the line, so GitHub opened a new alert; earlier alerts were
dismissed by hand. The issue counts only unsuppressed results, so #2073 has 2,
not 3. Alert #81 needs a manual dismissal, no code change.
2. minimatch: no eslint, `@eslint/config-array` or
`@typescript-eslint/typescript-estree` release takes minimatch 11 yet (latest
require ^10.2.x). Add `"overrides": {"minimatch": "^11.0.1"}` to
`e2e/package.json`, as the frontend does. `frontend/package-lock.json` already
resolves 11.0.3.
3. archplan: "untraced" means no seqtrace routes trace calls the entry
point. Add `_inbox` and `_inbox/summary` to the routes loop, and write a minimal
`realtime.yaml` into the seeded project so realtime is on and the request
reaches the service's gate instead of the realtime-off 404.

Alternatives rejected: baselining the finding (the route is cheap to trace);
`SEMGREP_EXCLUDE_RULES` (the rule is right elsewhere).

**Files to modify:**

- `internal/comments/filecomments/file.go`
- `e2e/package.json`, `e2e/package-lock.json`
- `tools/seqtrace/demo/run.sh`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No runtime input changes. The demo script writes
a fixed realtime.yaml into a throwaway project.

**Security-Sensitive Operations:** The suppression is reviewed: `lockWrite`
returns the unlock to the caller, which defers it.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. Run semgrep 1.181.0 as the workflow does; count results without
`suppressions`.
2. `npm install`, check the lockfile version, `npm run lint`,
`npm run typecheck`, and `npx eslint . --format json` file count.
3. Run `just archplan-security`; check the response codes of the inbox
requests and that no new finding names the inbox handlers.

**Edge Cases:** realtime.yaml failing to load would stop the demo server; the
run would fail loudly.

**Negative Tests:** N/A, no behaviour change in product code.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** minimatch 11 could break eslint's glob matching; mitigated by running
lint and checking it still lints files. Effort: xs.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal tooling and dependency change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore)

**Documentation Impact:** N/A - Internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: three mechanical changes that follow existing precedent)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
