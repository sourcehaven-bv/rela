---
id: BUG-EA0E8M
type: bug
title: 'cmdexec sets RLIMIT_NPROC, a per-UID ceiling, as if it were a per-command one'
description: 'cmdexec applied RLIMIT_NPROC=256 to every external command to bound fork bombs. RLIMIT_NPROC is scoped to the real UID, not to a process tree: the kernel checks it in fork(2) against every process and thread the user owns. The value therefore never granted a command 256 forks — it made that command''s forks fail whenever the UID was already above 256, for reasons unrelated to the command. On a CI runner where a Playwright suite held a few hundred browser threads under the same user, every `command:`-backed document render failed with `bwrap: Creating new namespace failed: Resource temporarily unavailable` (EAGAIN).'
priority: high
effort: s
why1: 'Every `command:` document render returned HTTP 500 with `command failed: exit status 1: bwrap: Creating new namespace failed: Resource temporarily unavailable`. bwrap could not fork to create its namespaces.'
why2: 'cmdexec set RLIMIT_NPROC=256 on each command via prlimit(2). The UID already owned more than 256 processes and threads, so the kernel refused the fork with EAGAIN.'
why3: 'RLIMIT_NPROC is scoped to the real UID, not to a process or its descendants. The limit rela set was charged against a counter shared with every other process running as the same user — in CI, four Playwright-launched Chromium instances and four rela-server processes, none of them rela''s children.'
why4: 'The intent — "bound one command''s forks" — is not reliably expressible in RLIMIT_NPROC at any value: since Linux 5.14 it is enforced through per-namespace ucounts, so what a given ceiling actually permits depends on the uid and user-namespace the command runs under as well as on unrelated UID-wide load. The constant was chosen as though the limit were per-command, and nothing in the API surfaces the UID scoping, so the mismatch was invisible at the call site and in review.'
why5: 'The limit failed open on a quiet host and closed on a busy one, so it produced no signal while wrong. It was introduced alongside RLIMIT_AS/RLIMIT_FSIZE/RLIMIT_CPU, which ARE per-process, and inherited their assumed scope by association; no test exercised it under a UID that was already near the ceiling, which is the only condition that reveals the difference.'
prevention: 'Remove RLIMIT_NPROC (and the Limits.MaxProcesses field, so no caller can reintroduce it by setting a value). The goal it was reached for is already met by mechanisms correctly scoped to the command: its own PID namespace via --unshare-pid, the process-group kill on timeout, per-process RLIMIT_AS and RLIMIT_CPU, and Runner.WithMaxConcurrent for aggregate load. Operators wanting a hard per-service ceiling get systemd TasksMax=, which is cgroup-scoped and therefore bounds the service and its children only — documented in the attachment-security guide beside the shipped unit. Pinned by AM-nproc-is-not-a-per-command-limit, which starts a child, runs applyRlimits against it, and reads the child''s RLIMIT_NPROC back via prlimit to assert rela left it at the inherited value; restoring the old line fails it (observed: the child dropped from the inherited 15383 to 256). Generalizable lesson: before bounding a resource, check what the limit is scoped TO — process, process tree, UID, or cgroup — because a limit applied at the wrong scope silently borrows its threshold from unrelated work and fails on load rather than on logic.'
status: done
---

## Symptom

Every document configured with `command:` returns HTTP 500:

```json
{"type":"https://rela.dev/errors/render_failed","title":"Document rendering failed",
 "status":500,
 "detail":"command failed: exit status 1: bwrap: Creating new namespace failed: Resource temporarily unavailable"}
```

The page renders its header, title and action buttons (these come from config),
then shows the empty state — "No document content available" — because only the
render failed.

## Expected

A `command:` document renders regardless of how many unrelated processes the
server's user happens to own.

## Reproduction

On Linux, with the server's UID already above the ceiling:

1. Configure a document with `command: ["cat", "{in}"]`.
2. Ensure the UID owns more than 256 processes/threads. In CI this happened on
   its own: a Playwright suite with 4 workers holds 4 Chromium instances plus 4
   `rela-server` processes under the runner's user.
3. Open the document. The render fails with the EAGAIN above.

Deterministically, without a busy host: lower `RLIMIT_NPROC` to 1 in a
subprocess and run any command through `cmdexec` — see
`internal/cmdexec/limits_nproc_linux_test.go`.

## Root cause

`RLIMIT_NPROC` is per-real-UID. From `getrlimit(2)`: "the maximum number of
simultaneous processes for this user id". Linux counts threads toward it.

`internal/cmdexec/limits_linux.go` applied it per command:

```go
set(unix.RLIMIT_NPROC, l.MaxProcesses) // MaxProcesses = 256
```

This does not mean "this command may create 256 processes". It means "this
command's forks fail unless the UID's total is below 256". rela sized a number
for its own behaviour against a counter it does not own, and lowered that
ceiling on a resource shared with every other process under the same user.

## Why the other limits were fine

`RLIMIT_AS`, `RLIMIT_FSIZE` and `RLIMIT_CPU` are per-process. Only
`RLIMIT_NPROC` is per-UID, which is why it was the only one that misfired.

## Affected code

- `internal/cmdexec/limits_linux.go` — the `RLIMIT_NPROC` application.
- `internal/cmdexec/limits.go` — `Limits.MaxProcesses` and its default of 256.
- `internal/cmdexec/sandbox_options.go` — the startup line claiming "PID" limits.
