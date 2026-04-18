---
id: TKT-CVPKG
type: ticket
title: Switch Go coverage to -coverpkg=./...
kind: chore
priority: medium
effort: s
status: done
---

## Description

Switch Go coverage measurement from per-package to cross-package by adding
`-coverpkg=./...` to the `go test` invocations used for coverage:

- `justfile` (`test-coverage` recipe)
- `.github/workflows/ci.yml` (Run tests step)
- `.github/workflows/post-merge-sync.yml` (Run tests with coverage step)

Regenerate `.coverage-baseline` with the new numbers so the ratchet has a
clean starting point. Utility packages like `internal/store/storeutil` and
the `internal/store/storetest` test kit will show their real coverage
(~98% and ~95% respectively) instead of 0%.

## Motivation

Discovered while investigating PR #403, where adding a few lines to
`storeutil.ValidateID` and `storetest` failed the ratchet with "251 lines
missing coverage" even though the new lines were fully exercised by
`fsstore` and `memstore` tests. Root cause: `go test ./...` without
`-coverpkg` only counts in-package coverage.

Fixing this globally means utility packages are tracked honestly and
future PRs touching them are not forced into either writing redundant
pass-through tests or padding exclusion lists.
