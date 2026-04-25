---
id: TKT-O03TB
type: ticket
title: Packaged-binary smoke test (//go:embed assets non-empty + served)
kind: test
priority: medium
effort: s
status: backlog
---

## Problem

BUG-W144 shipped a desktop binary with no Vue SPA assets — the `//go:embed`
directive resolved to an empty directory because the build pipeline didn't
declare the SPA generator as a dependency. Caught manually after a release
candidate test. Nothing prevents recurrence.

A small smoke test that runs after `just build` and asserts that the packaged
binaries serve a non-empty asset set would close this regression class for
`rela-server`, `rela-desktop`, and any future packaged binary.

## Scope

**In scope**

- Add a smoke test (Go test or shell script under `scripts/`) that:
  1. Spawns the built `rela-server` binary against a fixture project.
  2. Issues a GET to `/index.html` and verifies non-empty body + status 200.
  3. Issues a GET to one bundled JS asset path and verifies non-empty body.
  4. Tears down.
- Wire the test into `just smoke` and into the CI release pipeline.
- Audit `//go:embed` declarations: each one whose target is generated
must have its generator declared as a `just` recipe dependency.

**Out of scope**

- Full e2e coverage (already in `/e2e/`).
- Wails/desktop-binary smoke (harder, separate ticket if needed).

## Acceptance criteria

- `just smoke` builds, runs the binary against a fixture, and exits 0.
- Smoke test fails (and PR is blocked) if the embedded SPA is empty.
- Documented in `justfile` and `CLAUDE.md` Commands section.
