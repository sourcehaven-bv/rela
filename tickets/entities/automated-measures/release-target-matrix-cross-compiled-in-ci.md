---
id: release-target-matrix-cross-compiled-in-ci
type: automated-measure
title: Release target matrix is cross-compiled in CI
kind: ci
location: .github/workflows/ci.yml (cross-compile job)
status: active
description: |
  Every (GOOS, build-tag) combination GoReleaser ships must compile in PR CI,
  not just the linux default build. A unix-only symbol (e.g.
  syscall.O_NOFOLLOW) used without a build-tagged fallback compiles on the
  linux runner but breaks GoReleaser's windows/darwin cross-compile at release
  time, aborting the whole release and producing zero assets. The cross-compile
  job mirrors .goreleaser.yaml's target matrix (linux/darwin/windows x
  default/postgres) so this fails at PR time. Keep the job's goos/tags matrix
  in sync with .goreleaser.yaml builds.
---

The `cross-compile` job in `.github/workflows/ci.yml` builds `./cmd/rela` and
`./cmd/rela-server` for every `(GOOS, build-tag)` combination GoReleaser ships
(`linux`, `darwin`, `windows` × default, `postgres`) and is gated into the
`build` job's `needs`. This catches platform-specific compile breaks — most
notably unix-only `syscall` symbols used without a build-tagged fallback — at
PR time rather than silently at release time, where a single failing target
aborts the entire GoReleaser run and publishes no archives.

When GoReleaser's `builds` matrix changes, update this job's `goos`/`tags`
matrix to match.
