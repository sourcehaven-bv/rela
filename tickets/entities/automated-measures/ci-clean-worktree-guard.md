---
id: ci-clean-worktree-guard
type: automated-measure
title: 'CI guard: work tree clean after frontend build'
description: 'Step in the CI frontend job (.github/workflows/ci.yml, ''Work tree clean after build'') that fails the PR when any step of the job modified tracked files or left untracked files behind — `git status --porcelain --untracked-files=all` must be empty. Catches the generated-file-churn class (BUG-P0DZA5: vue-tsc artifacts shadowing vite.config.ts).'
kind: ci
location: .github/workflows/ci.yml
status: active
---
