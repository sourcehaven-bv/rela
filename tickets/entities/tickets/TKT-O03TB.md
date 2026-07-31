---
id: TKT-O03TB
type: ticket
title: Packaged-binary smoke test (//go:embed assets non-empty + served)
kind: test
priority: medium
effort: s
status: ready
---

## Problem

BUG-W144 shipped a desktop binary with no Vue SPA assets — the `//go:embed`
directive resolved to an empty directory because the build pipeline didn't
declare the SPA generator as a dependency. Caught manually after a release
candidate test. Nothing prevented recurrence — and it **did** recur:
**BUG-2YZ575** found every published release since v0.7 shipping `rela-server`
with an empty embedded SPA, for exactly this reason.

A small smoke test that runs after `just build` and asserts that the packaged
binaries serve a non-empty asset set would close this regression class for
`rela-server`, `rela-desktop`, and any future packaged binary.

## Status after BUG-2YZ575

BUG-2YZ575 delivered the **release-boundary** half of this guard
(`release-embedded-spa-guard`):

- `.github/workflows/release.yml` now builds the SPA before GoReleaser.
- `scripts/check-embedded-spa.sh` asserts the build tree, then asserts the
**actually-packaged** binaries embed the fingerprinted assets named by the built
`index.html` — covering `rela-server`, `rela-docs` and `rela-server-postgres`
across both archives, plus the `app_editor_dist` bundle.
- `scripts/check-embedded-spa-test.sh` pins 17 cases (mostly negative) so the
guard itself cannot silently rot.

## Remaining scope

**In scope**

- A smoke test that actually **spawns** the built binary against a fixture
project and issues real HTTP requests (`GET /index.html`, `GET` one bundled JS
asset; assert 200 + non-empty), rather than inspecting bytes. This catches
*serving* regressions — routing, SPA fallback, handler wiring — which the
byte-level guard cannot see.
- Wire it into `just smoke`.
- Audit remaining `//go:embed` declarations: each one whose target is generated
must have its generator declared as a `just` recipe dependency. (`static/v2` and
`app_editor_dist` are now covered at the release boundary; others are not
audited.)
- Consider running the tree-level check in `ci.yml` on PRs touching `frontend/`,
so an output-naming change is caught before a release rather than during one.

**Out of scope**

- Full e2e coverage (already in `/e2e/`).
- Wails/desktop-binary smoke (harder, separate ticket if needed).

## Acceptance criteria

- `just smoke` builds, runs the binary against a fixture, and exits 0.
- Smoke test fails (and PR is blocked) if the embedded SPA is empty **or** not
served.
- Documented in `justfile` and `CLAUDE.md` Commands section.
