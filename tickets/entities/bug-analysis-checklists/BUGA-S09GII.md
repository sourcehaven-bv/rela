---
id: BUGA-S09GII
type: bug-analysis-checklist
title: 'Analysis: Tracked vite.config.js shadows vite.config.ts — fresh-clone dev server breaks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (during this session's PR work, `npm run build:e2e` regenerated the tracked vite.config.js into two unrelated commits; the shadowing itself verified by reading Vite's config-resolution order vs the tracked file's content, which lacked the `__E2E_TEST_HOOKS__` define present in vite.config.ts)
- [x] Minimal reproduction steps documented (fresh clone at the affected revisions → `npm run dev` → open any form with a MarkdownEditor → ReferenceError: **E2E_TEST_HOOKS** is not defined)
- [x] Environment/conditions noted (any environment; bites on fresh clones and whenever the .js drifts behind the .ts)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (delete artifacts; `noEmit: true` in tsconfig.node.json — TS ≥5.6 permits it in composite projects; gitignore the names; CI clean-worktree guard after the frontend build)
- [x] Regression test planned (the CI guard IS the regression test: `git diff --exit-code` after build fails the job if any build step mutates the repo — catches this whole class, not just these two files)
- [x] Related areas checked for similar issues (searched for other tracked build outputs: `*.tsbuildinfo` not present/tracked; `internal/dataentry/static/v2` is the intended embed target, not churn; no other composite tsconfigs emit into the tree)
