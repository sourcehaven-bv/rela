---
id: TKT-X00CDI
type: ticket
title: Split docs build into a separate rela-docs binary (unlink chromedp from rela)
kind: refactor
priority: medium
effort: m
status: done
---

## Problem

The `screenshot{}` island links chromedp (+ cdproto/gobwas — 69 packages) into
the default `rela` binary via `internal/cli/docs_capturer_fs.go`, costing
**~14.7 MB / 31%** of binary size (47 MB → 62 MB). The common `rela
create/list/...` user never needs a browser linked in.

Measured: `go build ./cmd/rela` = 62 MB with the capturer seam, 47 MB with it
stubbed. `docscapture` also pulls `appbuild`/`store`/`search`, but those are the
shared base — chromedp is the only *extra* weight, and it rides in solely
through the CLI wiring.

## Change

Move `docs build` out of `rela` **entirely** into a new `cmd/rela-docs` binary:

- Extract `DocsCmd`/`DocsBuildCmd` + the `newDocsCapturer` seam (`docs_capturer_fs.go` / `docs_capturer_postgres.go`) out of `internal/cli` into a new home the rela-docs binary owns — this is what severs `internal/cli` → `docscapture` → `chromedp`.
- Remove the `Docs DocsCmd` field from the root `CLI` struct and `"docs"` from `requiresProject`.
- Give `cmd/rela-docs` its own small Kong root that reuses `appbuild.Discover` + the read bundle (metamodel + acl).
- Wire `justfile` (`build-docs`, add to `build-all`), `.goreleaser.yaml` (new `rela-docs` build, default tag only — it fails loud on postgres), and CI cross-compile.
- **Add a link-isolation assertion**: `rela` and `rela-server` must NOT link chromedp — mirroring the existing "no pgx / no bleve" greps in ci.yml so this can't regress.
- Keep `internal/docs` browser-free (consumer-side interface already holds); `.go-arch-lint.yml` allows only the new rela-docs home → `docscapture`.

## Not a breaking change

The docs feature (`rela docs build`) has never shipped — it lives only on the
rela-docs generator arc branches, unreleased. So this is a pure intra-branch
move: `docs build` is homed in `rela-docs` from its first release. No released
CLI surface changes; no user migration. The guide and example manual simply use
`rela-docs build` as the command from the outset.

## Acceptance criteria

- `go list -deps ./cmd/rela | grep chromedp` is empty; same for `./cmd/rela-server`.
- `go build ./cmd/rela` is back to ~47 MB.
- `rela-docs build <manual>` renders the manual (typeref/values/lifecycle/graph/roles_matrix/screenshot) — the doc-language behavior unchanged by the move.
- CI asserts the chromedp isolation; all build-tag combinations compile.

Part of the rela-docs generator arc (FEAT-G4VO53).
