---
id: TKT-W2UXGQ1
type: ticket
title: 'Triage 2026-11-02 security scan findings (GH #2073, #2074, #2076)'
kind: chore
priority: medium
effort: xs
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

The scheduled security scan of 2026-11-02
(https://github.com/sourcehaven-bv/rela/actions/runs/41404964363) opened three
issues:

- GH #2073, semgrep, 2 findings: `trailofbits.go.missing-unlock-before-return`
at `internal/comments/filecomments/file.go` lines 107 and 114, in `lockWrite`.
The function returns with `s.mu` held on purpose; the returned closure unlocks
it. False positive. Code-scanning alert #81 (`dangerous-exec-command`,
`internal/cmdexec/cmdexec.go:387`) is also open.
- GH #2074, osv-scanner: GHSA-g3hv-75wg-m249, npm `minimatch` 10.2.5 (dev) in
`e2e/package-lock.json`, fixed in 11.0.1. It comes in through `eslint`,
`@eslint/config-array` and `@typescript-eslint/typescript-estree`.
- GH #2076, archplan weekly: `security.untraced-08311e3c50b2`. The entry point
`serveInboxSummary` (`internal/dataentry/inbox_handler.go`, TKT-V05706X) is
never called in the seqtrace routes suite.

## Scope

In scope: the three issues above. Out of scope: changing how the scan workflow
uploads suppressed semgrep results.

## Acceptance criteria

- A local semgrep scan (`p/default`, severity ERROR, semgrep 1.181.0) reports
zero unsuppressed results.
- `e2e/package-lock.json` resolves `minimatch` to 11.0.1 or later, and
`npm run lint` and `npm run typecheck` in `e2e/` still pass.
- The seqtrace routes suite calls `GET /api/v1/_inbox` and
`GET /api/v1/_inbox/summary`, so archplan no longer reports them untraced.
