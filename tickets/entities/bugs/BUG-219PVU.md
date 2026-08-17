---
id: BUG-219PVU
type: bug
title: "Vulnerability Check red on develop — CI go-version pin resolves to an unpatched 1.26.5"
description: "The Vulnerability Check job fails on develop itself, not just on feature branches. 7 of the 8 called findings are Go stdlib issues fixed in 1.26.6, but the workflows pin a minor version, which actions/setup-go resolves to whatever 1.26.x is cached on the runner (1.26.5), so the patched toolchain was never used. The 8th is golang.org/x/image, fixed in 0.45.0."
priority: high
effort: s
status: done
why1: "govulncheck reports 8 called vulnerabilities, so the CI job exits non-zero."
why2: "7 are Go stdlib issues whose fix shipped in go1.26.6, but CI was building with go1.26.5."
why3: "The workflows pin `go-version: '1.26'`, a minor-version spec. actions/setup-go satisfies it from the runner's tool cache, which held 1.26.5 — so a patched patch-release is picked up only if and when the cache happens to refresh."
why4: "A floating minor pin was chosen for low maintenance, on the implicit assumption that 'latest 1.26.x' would track security patches. For a *vulnerability* gate that assumption is exactly backwards: the gate's whole job is to detect an unpatched toolchain, and the pin let one in."
why5: "The security gate's own input (the toolchain version) was not itself pinned to something the gate could vouch for. Nothing tied 'the version CI builds with' to 'the version we assert is patched', so the two could drift silently — and the failure surfaced as an apparently-unfixable vulnerability report rather than as a stale-toolchain signal."
prevention: "Pin the exact patch version (1.26.6) in all 20 go-version entries AND add a matching `toolchain` directive to go.mod, so CI and local builds agree. Both are required: actions/setup-go sets GOTOOLCHAIN=local, which makes Go ignore the go.mod directive, while the workflow pin alone leaves local builds unconstrained. When a stdlib CVE lands, bump both together — a govulncheck failure naming `stdlib` should now be read as 'bump the pin', not as an unfixable finding to add to IGNORED_OSVS."
---

## Symptom

`Vulnerability Check` fails on `develop` (verified against a clean worktree, so
it is not introduced by any in-flight branch). `scripts/govulncheck-filtered.sh`
exits 1 with 8 OSV ids:

```text
GO-2026-5026  GO-2026-5972  GO-2026-6088  GO-2026-6089
GO-2026-6090  GO-2026-6091  GO-2026-6218  GO-2026-6222
```

All 8 are *called* findings (trace length > 1), so the filter script correctly
refuses to ignore them.

## Root cause

Seven are Go stdlib, all fixed in **1.26.6**. The workflows pinned a minor
version, and the runner served a cached patch release:

```text
Setup go version spec 1.26
Found in cache @ /opt/hostedtoolcache/go/1.26.5/x64
go version go1.26.5 linux/amd64
```

The eighth (`GO-2026-6222`) is `golang.org/x/image`, fixed in **0.45.0**; the
module was at 0.44.0.

None of the 8 required the `IGNORED_OSVS` list — they were all fixable, and
reading them as unfixable would have been the wrong response.

## Fix

- All 20 `go-version` entries across the 6 workflow files pinned to `1.26.6`.
- Matching `toolchain go1.26.6` directive added to `go.mod`.
- `golang.org/x/image` 0.44.0 → 0.45.0 (pulls `x/text` 0.41.0).

Both toolchain changes are load-bearing. `actions/setup-go` sets
`GOTOOLCHAIN=local`, so the `go.mod` directive alone is ignored in CI; the
workflow pin alone would leave local builds on whatever the developer happens to
have installed.

## Verification

```text
govulncheck: no actionable vulnerabilities found.
Ignored (no upstream fix): GO-2026-4923
```

`GO-2026-4923` remains on the documented ignore list — still unfixed upstream
(bbolt, reached transitively via blevesearch). Full `go test ./...` green on
go1.26.6; `golangci-lint` clean; `just arch-lint` OK.
