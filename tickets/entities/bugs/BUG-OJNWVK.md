---
id: BUG-OJNWVK
type: bug
title: Release workflow Test job never installs bubblewrap, so cmdexec/attachment tests fail and the release is blocked
description: 'DUPLICATE of BUG-2J30F3, which diagnosed the same defect independently and landed the fix on develop first (PR #1256). Kept as a record of the parallel discovery; no separate fix shipped. See BUG-2J30F3 for the authoritative analysis.'
priority: high
effort: xs
why1: The Release workflow's Test job fails, so the Release job is skipped and no artifacts are published.
why2: '~20 tests in internal/cmdexec, internal/attachment and the transform/export paths fail with `cmdexec: start "bwrap": exec: "bwrap": executable file not found in $PATH`.'
why3: 'internal/cmdexec deliberately fails closed: on a host with no sandbox mechanism, commands REFUSE to run (documented in CLAUDE.md). Without bubblewrap installed, every command-running test fails by design.'
why4: release.yml's Test job is a bare `go test -v -race ./...` with no dependency install step. ci.yml grew an `Install bubblewrap (command sandbox)` step when the sandbox requirement landed; release.yml was never updated to match.
why5: release.yml only runs on tag pushes, so its jobs are never exercised by PR CI. Workflow drift between ci.yml and release.yml is therefore invisible until a release is attempted — the same blind spot that let BUG-2YZ575 ship a dead SPA across every release since v0.7.
prevention: 'Install bubblewrap in release.yml''s Test job, mirroring ci.yml. Longer term the two workflows should share the setup rather than re-declaring it (see BUG-2YZ575 why5: release.yml re-declaring what ci.yml/justfile already express is the recurring root cause).'
status: wont-fix
---

## Duplicate of BUG-2J30F3 — closed unfixed

Same defect, found independently and concurrently. **BUG-2J30F3 landed the fix
on `develop` first** (PR #1256), so this branch's duplicate commit was dropped
when merging `develop` in; the surviving implementation is BUG-2J30F3's.

The two diagnoses agree completely — release.yml's test gate ran on
`ubuntu-latest` without bubblewrap while ci.yml used `ubuntu-26.04` with it
installed and verified, and `internal/cmdexec` fails closed, so every
command-running test errored. Both fixes were byte-equivalent in effect (runner
pin + install + `bwrap --unshare-all` probe).

BUG-2J30F3 is the better record and should be treated as authoritative:

- It counted the blast radius precisely (35 tests across `internal/attachment`,
`internal/cli`, `internal/cmdexec`, `internal/dataentry`, `internal/transform`).
- It identified the real-world consequence — **`v26.7.1` was tagged but produced
no release object at all**, the inverse of the empty-release failure mode.
- It also documented the fix in `docs/releasing.md` under the enumerated release
failure modes.

## What this entity still contributes

The discovery path differed and is worth keeping: this one surfaced while
temporarily giving `release.yml` a `pull_request` trigger to exercise the
BUG-2YZ575 SPA guard. That is direct evidence for the shared why5 both bugs
reached — the release gate only runs on tag pushes, so drift is invisible at PR
time.

Both bugs, and [[BUG-2YZ575]], share one root cause: `release.yml` duplicates
setup that `ci.yml` and the justfile already express, with nothing keeping the
copies in sync. BUG-2J30F3 names the structural fix (a `workflow_call` reusable
gate) and defers it; that deferral is tracked as remaining scope on
[[TKT-O03TB]], alongside the case for a spawn-and-serve release e2e.
