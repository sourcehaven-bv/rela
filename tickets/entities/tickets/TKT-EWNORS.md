---
id: TKT-EWNORS
type: ticket
title: 'ci: drop unused rela-linux binary artifact'
kind: enhancement
priority: medium
effort: xs
status: done
---

The CI `build` job uploaded the compiled `bin/rela` as an artifact named
`rela-linux` on every push and PR to main/develop. Nothing ever consumed it: no
`download-artifact` step in any workflow referenced it, and `release.yml` builds
its own binaries from scratch via GoReleaser.

## Impact

At the default 90-day retention, ~1100 copies of a ~22 MB binary had accumulated
— **24.6 GB**, which is essentially the whole of the `sourcehaven-bv` org's
GitHub Actions storage consumption (org total was 24.8 GB against a 2 GB
included quota).

## Fix

Remove the `Upload binary` step. The `build` job stays as a compile-only gate,
so CI still fails on a broken build; it just no longer writes a binary nobody
downloads.

Existing `rela-linux` artifacts are not deleted by this change — retention
settings are not retroactive — so the backlog needs a separate purge.
