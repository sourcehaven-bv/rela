---
id: TKT-QRTE6G
type: ticket
title: 'Align local lint tooling with CI: golangci-lint v2, pinned go-arch-lint'
kind: chore
priority: medium
effort: xs
status: done
---

## Problem

A test/tooling-quality review found two version drifts between local tooling and
CI:

1. `justfile` `install-tools` installs **golangci-lint v1.62.2** via the v1 install script, but `.golangci.yml` is `version: "2"` format and CI installs **v2.11.4** (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4`). A fresh local setup gets a linter that cannot read the project config.
2. CI installs `go-arch-lint@latest` — an upstream release can break CI unrelated to any change in this repo (go-test-coverage and golangci-lint are already pinned).

## Fix

- `justfile`: install golangci-lint v2.11.4 via the same `go install` path CI uses (single version variable, matching CI).
- `justfile` + `.github/workflows/ci.yml`: pin go-arch-lint to v1.15.0 (the current latest, i.e. what `@latest` resolves to today).

## Verification

- `just lint` and `just arch-lint` pass with the pinned versions.
- CI green on the PR.

## Why (5-whys, abbreviated)

- why1: `install-tools` still referenced the v1 version pin after the repo's `.golangci.yml` migrated to the v2 config format.
- why2: the config migration updated CI but not the local bootstrap recipe — two installation paths, no single source of truth.
- why3: nothing exercises `install-tools` in CI, so the drift was invisible until a fresh machine ran it.

## Prevention

Local bootstrap now uses the identical `go install` command as CI with one
pinned version per tool, so the next version bump is a single-line change
visible in both contexts.
