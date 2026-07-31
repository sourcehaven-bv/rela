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
candidate test. Nothing prevented recurrence — and it **did** recur:
**BUG-2YZ575** found every published release since v0.7 shipping `rela-server`
with an empty embedded SPA, for exactly this reason.

## Why this needs to be a real e2e, not just byte checks

Temporarily running `release.yml` on a PR (BUG-2YZ575) surfaced a **second,
independent** release blocker within minutes: **BUG-OJNWVK** — the Test job
never installed bubblewrap, so `internal/cmdexec` failed closed, ~20 tests
failed, and the `release` job was skipped entirely.

Two defects, two different failure modes, both invisible to PR CI:

| Defect | Symptom | Would a byte-level guard catch it? |
| --- | --- | --- |
| BUG-2YZ575 | binary embeds an empty SPA | yes |
| BUG-OJNWVK | release job never runs at all | **no** |

The lesson: inspecting *bytes in an artifact* only proves the artifact was built
right. It cannot prove the release **path** works. A small e2e that actually
runs the packaged binary and talks to it over HTTP is the only check that covers
both classes — and it is cheap.

Placement note: this must live in the `release` job **after** GoReleaser (where
`dist/` exists), not in the `test` job, which builds no artifact.

## Status after BUG-2YZ575

Delivered at the release boundary (`release-embedded-spa-guard`):

- `release.yml` builds the SPA before GoReleaser.
- `scripts/check-embedded-spa.sh` asserts the build tree, then asserts the
**actually-packaged** binaries embed the fingerprinted assets named by the built
`index.html` — covering `rela-server`, `rela-docs` and `rela-server-postgres`
across both archives, plus `app_editor_dist`.
- `scripts/check-embedded-spa-test.sh` pins 17 cases so the guard cannot rot.

## Remaining scope

**In scope**

- **Spawn-and-serve smoke test in the `release` job**: unpack an archive, run
`rela-server` against a fixture project, `GET /` and one hashed bundle asset,
assert 200 + non-empty, tear down. Catches serving regressions (routing, SPA
fallback, handler wiring) *and* proves the release path itself executes.
- Wire the same check into `just smoke` for local use.
- Audit remaining `//go:embed` declarations: each generated target must have its
generator declared as a `just` recipe dependency. (`static/v2` and
`app_editor_dist` are covered at the release boundary; others are not.)
- **Reduce ci.yml/release.yml drift** — the shared root cause of BUG-2YZ575 and
BUG-OJNWVK is that `release.yml` re-declares setup (`bubblewrap`, runner image,
frontend build) that `ci.yml` and the justfile already express, with nothing
keeping the copies in sync. Consider a composite action or reusable workflow for
the common setup.

**Out of scope**

- Full e2e coverage (already in `/e2e/`).
- Wails/desktop-binary smoke (separate ticket if needed).

## Acceptance criteria

- `just smoke` builds, runs the binary against a fixture, and exits 0.
- The release job fails if the embedded SPA is empty **or** not served.
- Documented in `justfile` and `CLAUDE.md` Commands section.
