---
id: IMPL-8QVXRY
type: implementation-checklist
title: 'Implementation: Tracked vite.config.js shadows vite.config.ts — fresh-clone dev server breaks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: config-only change; the CI clean-worktree guard is the executable regression check)
- [x] ~~Integration tests written~~ (N/A: same — the guard runs on every PR build)
- [x] Happy path implemented (artifacts deleted; emit redirected to node_modules/.cache/tsc-node; frontend/.gitignore covers the names; CI guard added after the frontend Build step)
- [x] Edge cases from planning handled (noEmit was rejected by TS 6310 — referenced composite projects may not disable emit — so the outDir redirect was used instead; *.tsbuildinfo also covered)
- [x] Error handling in place (the CI guard prints the offending untracked files before failing)

## Test Quality

- [x] ~~Fixture/assertion items~~ (N/A: no test code in this change)

## Manual Verification

- [x] Feature manually tested end-to-end (`npm run build` and `npm run build:e2e` — the exact command that regenerated artifacts twice during this session — both leave the work tree clean; emitted files verified to land in node_modules/.cache/tsc-node)
- [x] Each acceptance criterion verified (artifacts gone from git; vite now resolves vite.config.ts since no .js exists; typecheck + 987 unit tests pass)
- [x] Edge cases manually verified (TS6310 path exercised and documented; build output internal/dataentry/static/v2 confirmed gitignored so the CI guard can't false-positive on the embed)

**Verification Evidence:** typecheck clean; 987 unit tests pass; `git status
--porcelain` after build + build:e2e shows only the intended change set. The
first attempted fix (`noEmit: true`) failed with TS6310 and is documented in the
bug analysis.

## Quality

- [x] Code follows project patterns (gitignore comment explains why; CI step comment references the bug)
- [x] Checked for DRY opportunities (N/A: 13-line diff)
- [x] No security issues introduced
- [x] No silent failures (guard echoes the untracked file list)
- [x] No debug code left behind
