---
id: BUG-2YZ575
type: bug
title: Release workflow ships rela-server with an empty embedded SPA (GoReleaser job never builds the frontend)
description: 'Every published rela_* archive ships a rela-server whose embedded Vue SPA is empty: the GoReleaser release job never runs the Vite build, and internal/dataentry/static/v2/ is gitignored, so //go:embed resolves to a tree with no index.html.'
priority: high
effort: s
why1: The release archives ship a rela-server whose embedded Vue SPA has no index.html, so the web UI is dead on arrival.
why2: The GoReleaser `release` job in .github/workflows/release.yml never runs the Vite build, so //go:embed all:static/* resolves to a tree containing only the committed favicon.
why3: internal/dataentry/static/v2/ is gitignored (generated output), so a clean CI checkout has no SPA unless a build step creates it — and an empty //go:embed glob is not a Go build error, so the build stayed green.
why4: 'The frontend-build dependency is expressed only in the justfile (`build-server: build-frontend`), not in the release workflow; the `desktop` job re-declared it by hand and the `release` job simply never did.'
why5: Generated-but-embedded assets have no enforced producer→consumer dependency at the release boundary, and nothing verified the published artifact. TKT-O03TB specified exactly this guard after BUG-W144 but was left in backlog, so the identical failure recurred undetected across every release since v0.7.
prevention: 'Build the SPA in the release job AND assert the packaged binary actually contains vite assets (release-embedded-spa-guard). The artifact-level assertion is the load-bearing half: it fails the release if a future edit drops or reorders the build step, rather than silently publishing a dead UI.'
status: done
---

## Symptom

Every published `rela_*` archive ships a `rela-server` whose embedded Vue SPA is
empty. The binary's own startup check reports it:

```text
level=ERROR msg="embedded SPA check failed"
error="embedded SPA is missing index.html (run `just build-frontend`): open index.html: file does not exist"
```

Reproduced against the shipped v0.14 darwin/arm64 artifact — downloaded,
extracted, run against a fixture project. The web UI is unusable.

## Root cause

`internal/dataentry/static/v2/` is gitignored (`.gitignore:52`); only
`static/favicon.svg` is committed. A clean CI checkout therefore has no SPA, and
`//go:embed all:static/*` (`internal/dataentry/static.go`) resolves to a tree
with no `index.html` unless the workflow runs the Vite build first.

In `.github/workflows/release.yml` the two build jobs are asymmetric:

| Job | Produces | Builds frontend? |
| --- | --- | --- |
| `release` (GoReleaser) | `rela`, `rela-server`, `rela-docs`, `*-postgres` | **no** |
| `desktop` | `rela-desktop` | yes (`npm ci && npm run build`) |

GoReleaser's only `before.hooks` entry is `go mod tidy`. Nothing generates
`frontend/dist` → `internal/dataentry/static/v2` in that job. The `justfile`
gets this right (`build-server: build-frontend`), and `ci.yml` builds the
frontend too — the release path is the sole place that does not.

## Scope

- **Affected:** `rela-server` and `rela-server-postgres` in every `rela_*` archive.
- **Not affected:** `rela-desktop` (DMG/MSI/deb/rpm) — that job builds the frontend.
- **Not a v0.14 regression:** v0.7, v0.9, v0.10, v0.11 and v0.14 all contain zero
vite `assets/index-*` markers. Broken across every release checked.

Note: `strings` on the binary does match `static/v2`, but only as error-message
literals (`"mount embedded SPA filesystem (static/v2): %w"`), never embedded
content — which is why a naive grep looks reassuring.

## Recurrence

This is the second occurrence of the class. BUG-W144 shipped a desktop binary
with no SPA for the same reason, and produced TKT-O03TB (packaged-binary smoke
test), which has sat in `backlog` since. The guard proposed then would have
caught this.

## Fix

Add Node setup + frontend build to the `release` job before GoReleaser runs,
mirroring what `desktop` already does, and pair it with an assertion so a green
release can never again ship a dead UI (see TKT-O03TB).

## Acceptance criteria

- The `release` job builds the Vue SPA before GoReleaser runs.
- A packaged `rela-server` from the release job starts without the
`embedded SPA check failed` error and serves a non-empty `index.html`.
- The release job **fails** if the embedded SPA is missing or empty, rather than
publishing a broken artifact.
