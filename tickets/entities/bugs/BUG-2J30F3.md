---
id: BUG-2J30F3
type: bug
title: Release test gate lacks bubblewrap so releases fail on commits that pass CI
description: The test job in release.yml runs on ubuntu-latest without bubblewrap while ci.yml uses ubuntu-26.04 with it installed. Confined commands fail closed with no sandbox so 35 attachment/cmdexec/transform/export tests fail on a commit that passed CI. This gated the Release workflow and left v26.7.1 a tag with no release object.
priority: high
why1: The Release workflow test gate failed on the exact commit that had just passed CI on develop, so release, desktop and homebrew were skipped and v26.7.1 got a tag with no release object.
why2: The runner had no bwrap binary, and confined external commands fail closed, so every attachment/cmdexec/transform/export test errored.
why3: release.yml's test job ran on ubuntu-latest with no bubblewrap install step, while ci.yml's runs on ubuntu-26.04 and both installs and verifies it.
why4: release.yml duplicates CI's test gate rather than reusing it, so the sandbox requirements added to ci.yml were never mirrored into release.yml.
why5: Nothing enforces that the two gates stay equivalent, and the release gate only executes on a tag push — so drift stays invisible until a release is cut and cannot be caught at PR time.
prevention: The release test gate now pins ubuntu-26.04, installs bubblewrap, and keeps ci.yml's `bwrap --unshare-all --ro-bind / / /bin/true` verification step so an absent sandbox is a hard failure rather than a silent mass-skip. docs/releasing.md documents runner drift from CI as a named release failure mode. The structural fix — a workflow_call reusable gate shared by both workflows, which would make this drift impossible — is deliberately deferred as a larger refactor.
status: review
---

## Summary

The `test` job in `.github/workflows/release.yml` is a second, independent copy
of CI's test gate, and it had drifted from `.github/workflows/ci.yml`:

|                | `ci.yml` test        | `release.yml` test (before) |
| -------------- | -------------------- | --------------------------- |
| Runner         | `ubuntu-26.04`       | `ubuntu-latest` (24.04)     |
| bubblewrap     | installed + verified | **not installed**           |

External commands (attachment scan/transform, view export) run confined via
`internal/cmdexec` and **fail closed** when no sandbox is available. With no
`bwrap` on the runner, every command-running test fails.

## Impact

`v26.7.1` was tagged successfully by `Tag Release`, but the `Release` workflow's
`test` gate failed, so `release` / `desktop` / `homebrew` were all skipped.

The result is the inverse of the `v0.8` / `v0.12` failure mode the release docs
warn about: not an empty published release, but **a tag with no release object
at all**. Consumers see no `v26.7.1` on the releases page and no assets.

35 tests failed, all from the one cause:

```
sandbox_test.go:71: transform: cmdexec: start "bwrap":
    exec: "bwrap": executable file not found in $PATH
```

Spanning `internal/attachment`, `internal/cli`, `internal/cmdexec`,
`internal/dataentry` (export 500s — export shells out through `cmdexec`), and
`internal/transform`. The identical commit passed CI on `develop` minutes
earlier, which is the signature of runner drift rather than a code defect.

## Fix

Bring the release test gate back in line with CI: pin `ubuntu-26.04`, install
bubblewrap, and keep the `bwrap --unshare-all --ro-bind / / /bin/true`
verification step so an absent sandbox is a hard failure rather than a silent
mass-skip.

`docs/releasing.md` gains this as a named entry under "Why a release can
silently produce no assets", since that list is where the release failure modes
are enumerated.

## Note on the duplicated gate

The deeper issue is that the release test gate is a copy of CI's rather than a
reuse of it. A `workflow_call` reusable workflow shared by both would make this
class of drift impossible. Not done here — that is a larger refactor of both
workflows and this needed to unblock the release.
