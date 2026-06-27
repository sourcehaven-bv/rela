---
id: IMPL-7P4MR2
type: implementation-checklist
title: 'Implementation: Test hygiene batch: pin vacuous tests, git skip guard, context-aware timeout handlers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] All four hygiene items implemented (cascade-count pin, frontmatter pin, requireGit guard, ctx-aware timeout handlers)
- [x] Edge cases: AI ctx-deadline-vs-keepalive mechanics handled via CloseClientConnections (review finding); frontmatter boundary stated honestly
- [x] Error handling — n/a (test-only)

## Test Quality

- [x] Pins probed empirically before asserting (cascade 3/1 write counts; frontmatter deterministic error)
- [x] Reviewer traced the cascade shape through the manager/cascade/upsert code paths and confirmed both the 3/1 pin and the 4/2 failure mode; coupling to the upsert shape matches existing house style (TestCreate_AutomationSetsStatus)
- [x] requireGit completeness verified by reviewer: runCmd is the only binary exec, reachable only via the two guarded helpers; library-backed tests correctly unguarded; gitless-PATH simulation skips

## Manual Verification

- [x] Five packages green under `-race -count=2 -shuffle=on`; lint 0 issues; full `just ci` green
- [x] Wall-time: lua timeout handler unblocks in µs on client close (verified by reviewer); AI handler unblocks via CloseClientConnections (timing verified post-fix)

## Quality

- [x] No security issues; no silent failures; no debug code; probes removed
