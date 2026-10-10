---
id: AM-lint-every-build-tag
type: automated-measure
title: golangci-lint runs under every backend build tag
description: CI lints each backend build tag (sqlite, postgres, memorybackend), so a finding in a tag-guarded file cannot reach develop unnoticed.
kind: ci
location: '.github/workflows/ci.yml (Lint job: default+maildemo,mailmanual, sqlite, postgres, memorybackend steps); justfile lint'
status: active
---

## What it prevents

[[BUG-RNM0MU]]: a testifylint finding in a `sqlite`-tagged test reached develop.
CI runs `golangci-lint run ./...` without build tags, so the linter never sees
files guarded by `sqlite` or `postgres`.

## Shape of the measure

Run golangci-lint once per backend tag (`sqlite`, `postgres`, `memorybackend`)
in addition to the default build.

At deb190a0 this would fail on existing findings: 2 under `sqlite` and 20 under
`postgres`. Those must be cleared before the step can gate.

## Status

Active since TKT-PHWFAI (GitHub #1775), which cleared the findings and added
the steps. The default step also passes `maildemo,mailmanual`, which only add
test files. OS-specific files (windows) are not covered.
