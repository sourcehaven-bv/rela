---
id: IMPL-4CKV2T
type: implementation-checklist
title: 'Implementation: cmdexec sets RLIMIT_NPROC, a per-UID ceiling, as if it were a per-command one'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: the change is a deletion; the
edge case IS the happy path — a command running while the UID is busy)
- [x] ~~Error handling in place~~ (N/A: removes a limit application, adds no
error path)

The change is a removal, in four places:

- `internal/cmdexec/limits_linux.go` — drop the `RLIMIT_NPROC` application, with
  a comment recording why the mechanism is wrong rather than the value.
- `internal/cmdexec/limits.go` — drop `Limits.MaxProcesses` (default 256). The
  field is deleted rather than defaulted to zero so a caller cannot reintroduce
  the limit by passing a value.
- `internal/cmdexec/sandbox_options.go` — the startup line advertised
  "memory/PID/file-size/CPU limits"; the PID claim is now false.
- `docs-project/entities/guides/GUIDE-attachment-security.md` — the confinement
  bullet list made the same claim. Replaced with the PID namespace, which is
  what actually bounds forks, plus a "Bounding process count" section pointing
  at systemd `TasksMax=`.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: the assertions are
on a command's stdout and on whether it ran at all)
- [x] Property comparisons use original object, not hardcoded strings

`internal/cmdexec/limits_nproc_linux_test.go` starts a child, runs
`applyRlimits` against it with the production `DefaultLimits()`, and reads the
child's `RLIMIT_NPROC` back via `prlimit(2)`, asserting rela left it at the
inherited value. A companion test asserts `RLIMIT_AS`/`RLIMIT_FSIZE`/
`RLIMIT_CPU` are still applied, so the first cannot be satisfied by disabling
`applyRlimits` wholesale.

The first version of this test was wrong and was caught in review. It lowered
`RLIMIT_NPROC` to 1 in a subprocess and expected a command to run anyway — but
`setrlimit(2)` applies to the calling process, which must then `fork(2)`, so the
harness's own `exec` was refused before reaching any code under test. It failed
identically with and without the fix, and a second defect (matching bare `PASS`,
which a *skipped* test also prints) would have masked that. Both confirmed
empirically rather than argued.

## Verification

| Check | Result |
| ----- | ------ |
| `go build ./...` | pass |
| `GOOS=linux go build ./internal/cmdexec/` | pass |
| `GOOS=linux go vet ./internal/cmdexec/` | pass |
| `go test ./internal/cmdexec/` (macOS) | pass, 8.4s |
| `go test ./internal/dataentry/` | pass, 35.0s |
| `go test ./internal/attachment/` | pass, 0.8s |
| `gofmt -l internal/cmdexec/` | clean |
| `rela validate --check cardinality --check properties --check validations` | pass |
| `just docs` + `git diff docs/` | regenerated, in sync |

## Verified on Linux

Run in a Debian 12 aarch64 VM (Go 1.26.6, bwrap present), since `applyRlimits`
is a no-op on this macOS dev machine:

| Check | Result |
| ----- | ------ |
| `go test ./internal/cmdexec/ -run TestApplyRlimits -v` | both PASS |
| `go test ./internal/cmdexec/` (full package) | ok, 7.8s |
| Mutation: restore `set(unix.RLIMIT_NPROC, 256)` | FAILS — child dropped to `cur=256` from an inherited `cur=15383` |

That inherited 15383 is the point of the bug in one number: the host's own
ceiling was 60x what rela was narrowing it to.

Two supporting facts were measured rather than assumed:

- `setrlimit(RLIMIT_NPROC, 1)` breaks the *calling* process's `fork/exec` — a
  plain `/bin/true` with no rela code involved fails — which is why the first
  version of the test could not have worked.
- A PID namespace imposes no numeric ceiling: 300 processes fork without
  complaint inside `bwrap --unshare-all` (`PROCS_IN_NS=305`). The guide's
  wording was corrected from "cannot escape" to containment-and-reaping
  accordingly.
- `DefaultTasksMax` is 15% of `kernel.threads-max`, not `kernel.pid_max`:
  measured `threads-max=30767`, `pid_max=4194304`, `DefaultTasksMax=4615`. The
  guide's original "~4900 / 15% of pid_max" was wrong and is fixed.

## Diagnosis provenance

Not inferred. The 500's response body was recovered from the failing run's
Playwright trace artifact:

```json
{"detail":"command failed: exit status 1: bwrap: Creating new namespace failed: Resource temporarily unavailable"}
```

`Resource temporarily unavailable` is `EAGAIN`, which is what `fork(2)` returns
when `RLIMIT_NPROC` is exceeded.
