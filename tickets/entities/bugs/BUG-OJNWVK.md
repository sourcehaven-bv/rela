---
id: BUG-OJNWVK
type: bug
title: Release workflow Test job never installs bubblewrap, so cmdexec/attachment tests fail and the release is blocked
description: 'release.yml''s Test job runs `go test -v -race ./...` without installing bubblewrap. internal/cmdexec fails closed when no sandbox mechanism exists, so ~20 cmdexec/attachment/transform tests fail, the Test job fails, and the Release job (needs: [test, security]) is skipped — no artifacts are ever published. ci.yml installs bubblewrap; release.yml never has.'
priority: high
effort: xs
why1: The Release workflow's Test job fails, so the Release job is skipped and no artifacts are published.
why2: '~20 tests in internal/cmdexec, internal/attachment and the transform/export paths fail with `cmdexec: start "bwrap": exec: "bwrap": executable file not found in $PATH`.'
why3: 'internal/cmdexec deliberately fails closed: on a host with no sandbox mechanism, commands REFUSE to run (documented in CLAUDE.md). Without bubblewrap installed, every command-running test fails by design.'
why4: release.yml's Test job is a bare `go test -v -race ./...` with no dependency install step. ci.yml grew an `Install bubblewrap (command sandbox)` step when the sandbox requirement landed; release.yml was never updated to match.
why5: release.yml only runs on tag pushes, so its jobs are never exercised by PR CI. Workflow drift between ci.yml and release.yml is therefore invisible until a release is attempted — the same blind spot that let BUG-2YZ575 ship a dead SPA across every release since v0.7.
prevention: 'Install bubblewrap in release.yml''s Test job, mirroring ci.yml. Longer term the two workflows should share the setup rather than re-declaring it (see BUG-2YZ575 why5: release.yml re-declaring what ci.yml/justfile already express is the recurring root cause).'
status: done
---

## Symptom

The `Release` workflow's `Test` job fails, so `release` (which declares `needs:
[test, security]`) is **skipped** and nothing is ever published.

```text
cmdexec: start "bwrap": exec: "bwrap": executable file not found in $PATH
```

~20 failures across `internal/cmdexec`, `internal/attachment` and the
transform/export paths — e.g. `TestRun_Stdout`, `TestRun_InOutFiles`,
`TestCmdRunner_ArrayArgsNoShellInjection`,
`TestAttachmentTransformBlocksEgress`, `TestExport_List_RenderOverride`.

## How it was found

While verifying the BUG-2YZ575 SPA guard, `release.yml` was temporarily given a
`pull_request` trigger (GoReleaser in `--snapshot`, publishing nothing) so the
new guard could be observed running. The guard never got to run: the `Test` gate
failed first and skipped the `release` job. Run 30611472878.

## Root cause

`internal/cmdexec` **fails closed** — on a host with no sandbox mechanism,
commands refuse to run rather than executing unconfined. That is correct and
deliberate (CLAUDE.md: "On a host with no mechanism, commands REFUSE to run").

`ci.yml` accounts for it:

```yaml
- name: Install bubblewrap (command sandbox)
  run: sudo apt-get update && sudo apt-get install -y bubblewrap
```

`release.yml`'s Test job is a bare `go test -v -race ./...` with no such step,
and never has been. Verified byte-identical to `develop` — this is pre-existing,
not a regression from the BUG-2YZ575 branch.

## Impact

**The next real release would fail at the Test gate before reaching
GoReleaser.** Combined with BUG-2YZ575 (which made every published `rela-server`
ship a dead web UI), the release path had two independent defects that PR CI
could not see.

## Fix

Add the bubblewrap install to `release.yml`'s Test job, mirroring `ci.yml`.

## Acceptance criteria

- `release.yml`'s Test job installs bubblewrap before running the suite.
- The `cmdexec` / `attachment` / transform tests pass in the Release workflow.
- The `release` job is reachable (not skipped by a failed Test gate).
