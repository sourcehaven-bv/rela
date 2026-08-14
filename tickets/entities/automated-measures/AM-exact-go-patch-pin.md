---
id: AM-exact-go-patch-pin
type: automated-measure
title: "Exact Go patch-version pin in CI plus a matching go.mod toolchain directive"
kind: ci
location: .github/workflows/*.yml (go-version) + go.mod (toolchain)
status: active
description: >-
  Every `go-version` entry across the workflow files pins an exact patch
  release (e.g. 1.26.6), not a floating minor (1.26), and go.mod carries a
  matching `toolchain` directive. Together these make the toolchain the
  vulnerability gate runs against an explicit, reviewable value rather than
  whatever patch release happens to sit in the runner's tool cache.
---

## What it prevents

BUG-219PVU: `Vulnerability Check` went red on `develop` because the workflows
pinned `go-version: '1.26'`. `actions/setup-go` satisfied that from the
runner's tool cache, which held **1.26.5**, while 7 of the 8 reported findings
were stdlib issues fixed in **1.26.6**. The build was silently one patch
release behind the fix the gate was asking for.

The failure mode is nastier than a plain stale dependency: govulncheck reports
the findings as `stdlib`, which reads like an unfixable platform issue, and the
tempting response is to add them to `IGNORED_OSVS`. That would have suppressed
eight genuinely-fixed vulnerabilities.

## Why BOTH halves are required

- **Workflow pin alone** — local builds stay on whatever the developer has
  installed, so `just govulncheck` can disagree with CI.
- **`toolchain` directive alone** — `actions/setup-go` exports
  `GOTOOLCHAIN=local`, which makes Go *ignore* the directive rather than fetch
  the named toolchain. CI would keep using the cached release.

With both, CI and local agree, and the version is visible in a diff.

## Maintenance contract

When a stdlib CVE lands, bump the workflow pins and the `toolchain` directive
**together**. A govulncheck failure naming `stdlib` should be read as *"bump
the pin"*, never as an unfixable finding to append to `IGNORED_OSVS` — that
list is only for findings with no upstream fix (currently GO-2026-4923, bbolt
via blevesearch).
