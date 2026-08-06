---
id: BUGA-ZUYX1R
type: bug-analysis-checklist
title: 'Analysis: Release workflow ships rela-server with an empty embedded SPA (GoReleaser job never builds the frontend)'
status: done
---

## Reproduction

- [x] Reproduced against the shipped artifact, not a simulation: downloaded
`rela_0.14_darwin_arm64.tar.gz` from the GitHub release, extracted, ran
`rela-server` against a fixture project. Startup logged `embedded SPA check
failed ... open index.html: file does not exist`.
- [x] Established scope across releases: v0.7, v0.9, v0.10, v0.11, v0.14 all
contain zero vite entry-asset markers. Not a v0.14 regression.

## Root cause

- [x] Identified — see the bug's why1–why5. In short: `release.yml`'s GoReleaser
job never runs the Vite build; `internal/dataentry/static/v2/` is gitignored; an
empty `//go:embed` glob is not a Go build error, so the build stayed green and
shipped a dead UI.
- [x] Confirmed the misleading signal: `strings` on the binary *does* match
`static/v2`, but only from embed error-message literals — which is why a naive
grep looks reassuring and why the guard matches fingerprinted asset names
instead.

## Fix plan

- [x] Build the SPA in the release job (mirrors the `desktop` job).
- [x] Assert at two levels — build tree, then the actually-packaged binaries.
- [x] Cover every SPA-embedding binary across both archives.
- [x] Test the guard itself, so it cannot rot the way TKT-O03TB did.
