---
id: FEAT-KJ7Z8D
type: feature
title: Staging build workflow for arbitrary refs
summary: Build the postgres server tarball from any branch or PR as an expiring artifact without cutting a release
description: staging-build.yml builds any ref with goreleaser --snapshot and uploads the rela-postgres tarball plus its sha256 as a 7-day artifact. No tag and no release is created, so the build counter shared by stable and -alpha tags stays untouched.
status: in-progress
---

The devops `roles/atlas` only installs a pinned, sha256-verified release, and
the only thing producing a `rela-postgres_*` tarball is `release.yml`, which
fires on a `v*` tag push. Trying a rela PR against real ISMS content therefore
meant cutting a release to production first.

`.github/workflows/staging-build.yml` closes that gap for the new
`staging-atlas` environment:

- `workflow_dispatch` with a `ref` input, plus an opt-in `pull_request` path
  gated on a `staging` label.
- GoReleaser runs `--snapshot`, which builds every archive and publishes
  nothing. That is what keeps `scripts/generate-version-tag.sh`'s shared build
  counter from being consumed by builds that were never released.
- Version is `<calver>-alpha-<short-sha>`, supplied through
  `snapshot.version_template` — in snapshot mode GoReleaser ignores
  `GORELEASER_CURRENT_TAG`, so the template is what drives the archive filename
  that `roles/atlas` reconstructs when it downloads a pin.
- Both SPA-embed guards from `release.yml` are kept, including the
  packaged-binary check. Upstream once shipped a correctly-checksummed tarball
  whose server binary had an empty embed and exited at startup (BUG-2YZ575);
  a staging artifact with that defect would wedge the deploy.

Retention is the whole cleanup policy: artifacts expire after 7 days and there
is no prune job. A pin older than that is re-created by re-dispatching on the
same ref.
