---
id: TKT-PHWFAI
type: ticket
title: Lint (incl. gosec) every backend build tag in CI
kind: chore
priority: medium
effort: s
started: "2026-10-09"
completed: "2026-10-09"
status: done
description: 'CI runs golangci-lint without build tags, so files behind sqlite/postgres/memorybackend are neither linted nor scanned by gosec (GitHub #1775). Clear the tagged findings and lint each tag in CI and just lint.'
---

## Description

GitHub #1775 (finding B1 from the security review on #1756). `golangci-lint run
./...` in CI and `just lint` use the default build only. Files behind
`//go:build sqlite`, `postgres` or `memorybackend` are not linted, and gosec
(enabled in `.golangci.yml`) does not scan them. That covers the
`rela-server-postgres` build: `internal/tenant/dsn_postgres.go`,
`internal/jobs/pgqueue.go`, `internal/appbuild/*_postgres.go` and others.

BUG-RNM0MU proposed AM-lint-every-build-tag for the lint side. This ticket
implements it, which also covers SAST.

## Acceptance criteria

1. The existing findings under every backend tag are fixed.
2. CI runs golangci-lint for the default build and for `sqlite`, `postgres` and
`memorybackend`, and fails on any finding.
3. `just lint` runs the same set locally.
4. AM-lint-every-build-tag is active.
