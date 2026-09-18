---
id: AM-nproc-is-not-a-per-command-limit
type: automated-measure
title: applyRlimits leaves RLIMIT_NPROC at the inherited value
description: |-
    Guards against BUG-EA0E8M, where cmdexec set RLIMIT_NPROC — a per-real-UID ceiling — as though it bounded one command, so a busy host refused every `command:` render with `bwrap: Creating new namespace failed: Resource temporarily unavailable`.

    `TestApplyRlimitsLeavesNPROCAlone` starts a child, runs `applyRlimits` against it with the production `DefaultLimits()`, then reads the child's RLIMIT_NPROC back via `prlimit(2)` and asserts it still equals the inherited value. That states the property directly: whatever the UID is doing, rela does not narrow this command's process ceiling.

    `TestApplyRlimitsStillAppliesPerProcessCeilings` is the companion guard. Without it, the first test could be satisfied by disabling `applyRlimits` altogether, silently dropping the memory, file-size and CPU ceilings that ARE correctly scoped.

    Mutation-verified on Linux: restoring `set(unix.RLIMIT_NPROC, 256)` fails the first test, reporting the child dropped from an inherited 15383 to 256. Both tests pass with the fix in place.
kind: test
location: internal/cmdexec/limits_nproc_linux_test.go
status: active
---

## Why it reads the limit instead of provoking the failure

The obvious test — lower `RLIMIT_NPROC` and watch a command fail — cannot work,
and the first version of this measure was written that way and was wrong.

`setrlimit(2)` applies to the **calling** process, which then has to `fork(2)`
to launch anything. Lowering the ceiling below the UID's current usage therefore
breaks the test harness's own `exec` before any code under test is reached.
Verified on Debian 12: after dropping `RLIMIT_NPROC` to 1, a plain
`exec.Command("/bin/true")` — with no rela code involved at all — fails with
`resource temporarily unavailable`. The test would have failed identically with
and without the fix, pinning nothing.

Reading the child's limit back avoids the problem entirely and asserts the
actual invariant.

## Why Linux-only

`applyRlimits` is a no-op on macOS and BSD (`prlimit(2)` is Linux-specific), so
there is nothing to regress. The file carries a `_linux` suffix rather than a
runtime skip.

## What this does not cover

These tests pin that rela imposes no per-command NPROC ceiling. They do not
verify a fork bomb is *contained* — that is the PID namespace
(`--unshare-pid`), the process-group kill, and `WithMaxConcurrent`, which have
their own tests. Note the PID namespace bounds a fork bomb's lifetime and
blast radius, not its process count: 300 processes fork without complaint
inside `bwrap --unshare-all`. A numeric ceiling is an operator concern, served
by systemd `TasksMax=`.
