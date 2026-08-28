---
id: REV-DB4IPJ
type: review-checklist
title: 'Review: Ignore TypeScript major bumps until TS 7.1 lands a compiler API'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] `dependabot.yml` parses as valid YAML; both `ignore` blocks land in the intended `/frontend` and `/e2e` entries
- [x] Full CI green on PR #1331 apart from this ticket gate (lint, Test, Frontend, Postgres, cross-compile, CodeQL)
- [x] ~~Go tests~~ (N/A: config-only change, no Go code touched)

## Code Review
- [x] ~~cranky-code-reviewer~~ (N/A: no code change — a Dependabot config rule; nothing to review beyond the YAML diff)
- [x] Self-reviewed the diff: only `.github/dependabot.yml`, two additive `ignore` entries

## Acceptance Verification
- [x] Blocker re-verified upstream: `typescript-eslint` 8.67.0 still caps `peerDependencies.typescript` at `>=4.8.4 <6.1.0`
- [x] `ERESOLVE` reproduced locally against the latest release; `--legacy-peer-deps` also fails on a hard runtime guard ("does not support TS 7.0")
- [x] Scope confirmed majors-only — TS 6.x patch/minor updates still flow; `build-tooling` group is already minor/patch-only so no group PR can reintroduce the major

## Documentation
- [x] Rationale recorded inline as YAML comments next to each rule, with the upstream issue link (typescript-eslint#10940)
- [x] Revisit condition documented: delete both entries when TS 7.1 ships the new compiler API

## Final Checks
- [x] Commit message explains the why (upstream peer cap, no API in TS 7.0)
- [x] Follow-ups noted on the PR: close #1220 and #1227 once merged; `e2e/tsconfig.json` needs `moduleResolution: node16` before any future TS 7 bump
- [x] Ready to merge

## Pull Request
- [x] PR #1331 opened against develop.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1331
