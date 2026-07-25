---
id: TKT-0AMKYB
type: ticket
title: CalVer releases (vYY.M.BUILD) with an automated tag cutter
kind: enhancement
priority: medium
effort: s
status: done
---

Replaces the manual `v0.<minor>` tag bump with a date-based scheme and a
workflow that computes and pushes the next tag.

## Scheme

`vYY.M.BUILD` — build counter resets each month.

```text
v26.7.0        first release of July 2026
v26.7.1        second that month
v26.7.2-alpha  a prerelease
v26.8.0        August — counter resets
```

Legacy `v0.1`…`v0.15` tags sort below every CalVer tag, so both schemes coexist.
The day is not encoded in the tag; `git log -1 <tag>` and the GitHub release
date carry it.

## Why not openvwr's `vYYYYMMDD` verbatim

This is adapted from openvwr, but that exact format could not be reused. It is
fine for GoReleaser — `v20260725` parses, orders correctly, and extracts
`-alpha` properly — but the `20260725` major exceeds the **MSI `ProductVersion`
255 cap**, so `wix build` fails.

openvwr never hits this: it ships a single PHP `tar.gz` where the version is
only "a package label baked into the archive", never a parsed version field.
rela ships `.msi` / `.dmg` / `.deb` / `.rpm`.

`vYY.M.BUILD` clears both constraints with no remapping — valid semver with all
three fields meaningful, and `YY <= 99` / `M <= 12` sit far inside the MSI
limits, so the tag is used verbatim in every artifact.

## Tag-push trigger

A tag pushed with the default `GITHUB_TOKEN` does not fire `on: push`, so
`Release` would never start. `Tag Release` pushes with a **GitHub App token**
minted from the existing `APP_ID` / `APP_PRIVATE_KEY` secrets — the same app
`security.yml` and `dependabot-auto-merge.yml` already use, and which already
pushes branches there, so it has the required `contents: write`. No new secret
is needed.

## Changes

- `scripts/generate-version-tag.sh` (new) — computes the next tag
- `.github/workflows/tag-release.yml` — calls the script; adds an `alpha`
input; keeps the existing `ref` / `dry_run` inputs and the tag-exists guard
- `.github/workflows/release.yml` — adds `workflow_dispatch` (no logic changes)
- `docs/releasing.md` — scheme, rationale, and the app-token note

## Verification

Tested in throwaway repos: increments `.0`→`.1`→`.2`; numeric sort so `.10`
follows `.9`; alpha and stable share one counter so they cannot collide; month
rollover resets to `.0`; prior-month, prior-year and legacy `v0.x` tags ignored;
parses as semver with `-alpha` extracted, and `v0.15` still sorts below
`v26.7.0`.

PR: https://github.com/sourcehaven-bv/rela/pull/1202
