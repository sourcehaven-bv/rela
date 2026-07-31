---
id: release-embedded-spa-guard
type: automated-measure
title: 'Release guard: embedded SPA present in packaged binaries'
description: Steps in the Release workflow's GoReleaser job that (a) build the Vue SPA before GoReleaser runs and (b) assert the build produced internal/dataentry/static/v2/index.html plus a non-empty assets/ dir, then verify a packaged rela-server binary actually contains the vite asset markers. Fails the release rather than publishing an artifact whose //go:embed resolved to an empty tree (BUG-2YZ575, and BUG-W144 before it).
kind: ci
location: .github/workflows/release.yml
status: active
---

Closes the empty-`//go:embed` regression class at the release boundary.

The failure mode is silent by construction: `internal/dataentry/static/v2/` is
gitignored, so a checkout without a frontend build embeds an empty tree and the
Go build still succeeds. Only a startup check or an asset probe reveals it.

Two layers, both in the `release` job:

1. **Build** the SPA (`npm ci && npm run build`) before GoReleaser, mirroring
what the `desktop` job already does.
2. **Assert** — after the build, `index.html` and a non-empty `assets/` must
exist; after GoReleaser, a packaged `rela-server` must contain `assets/index-*`
markers.

Layer 2 is the load-bearing one: without it a future refactor that drops or
reorders the build step reverts to silently shipping a dead UI.

Related: [[ci-clean-worktree-guard]] covers generated-file churn in the PR
frontend job; this covers asset *absence* in the release job.
