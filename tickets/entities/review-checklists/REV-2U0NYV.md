---
id: REV-2U0NYV
type: review-checklist
title: 'Review: cmdexec sets RLIMIT_NPROC, a per-UID ceiling, as if it were a per-command one'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./internal/cmdexec/` ok (macOS 8.4s; Debian 12 7.8s), `./internal/dataentry/` ok 35.0s, `./internal/attachment/` ok 0.8s
- [x] Lint clean — `gofmt -l internal/cmdexec/` clean; `GOOS=linux go vet ./internal/cmdexec/` clean
- [x] ~~Comment lint gate~~ (run as part of the pre-commit hook)
- [x] ~~Coverage maintained~~ (N/A: the change is a net deletion of production code; the new tests raise package coverage)

`rela validate --check cardinality --check properties --check validations`
passes, and `just docs` leaves `docs/` in sync with `docs-project/`.

## Code Review

- [x] Run `/code-review` command — cranky-code-reviewer
- [x] All critical review-responses addressed — 3 of 3
- [x] All significant review-responses addressed — 3 of 3
- [x] Self-reviewed the diff for unrelated changes

The reviewer **blocked the merge**, and was right to. It did not reason about
the code — it provisioned a Debian 12 VM and ran it. Five findings, all valid,
all fixed. Every one was then re-verified independently rather than accepted on
the reviewer's word.

### Critical — the regression test could never have passed

The first version lowered `RLIMIT_NPROC` to 1 in a subprocess and asserted a
command still ran. `setrlimit(2)` applies to the **calling** process, which then
has to `fork(2)` to launch anything, so the harness's own `exec` was refused
before reaching any code under test.

Reproduced here without rela in the picture at all:

```
BEFORE: /bin/true ran fine
RLIMIT_NPROC now 1 (for THIS process)
AFTER: plain /bin/true, NO rlimit applied by us: err=fork/exec /bin/true: resource temporarily unavailable
```

The test therefore failed identically with and without the fix — it pinned
nothing, and the mutation-check claim written into two ticket files was false.
Both files are corrected.

Two aggravating defects in the same file:

- the "did the helper run" guard matched bare `PASS`, which a **skipped** test
  also prints (confirmed: `--- SKIP: TestSkipper` is followed by `PASS`), so a
  broken env guard would have reported success;
- `t.Skipf` on `Setrlimit` failure would have hidden exactly the constrained
  environments worth a signal.

Rewritten to assert the property directly: start a child, run `applyRlimits`,
read the child's `RLIMIT_NPROC` back via `prlimit(2)`, assert it equals the
inherited value. Now genuinely mutation-verified on Linux — restoring
`set(unix.RLIMIT_NPROC, 256)` fails it:

```
applyRlimits changed the child's RLIMIT_NPROC to {cur=256 max=256},
want the inherited {cur=15383 max=15383}.
```

A companion test pins that `RLIMIT_AS`/`FSIZE`/`CPU` are still applied, so the
first cannot be satisfied by disabling `applyRlimits` altogether.

### Significant — a wrong number in the docs

The guide claimed `DefaultTasksMax` is "15% of `kernel.pid_max`, typically
~4900". It is 15% of the *minimum* of `threads-max`, `pid_max - 1` and the root
cgroup's `pids.max` — in practice `threads-max`, which is RAM-derived. Measured:

```
pid_max=4194304  threads-max=30767  DefaultTasksMax=4615  (15% of threads-max)
```

15% of `pid_max` would be ~629,000, three orders of magnitude out. The ~4900
figure assumes `pid_max = 32768`, stale since systemd v243 on 64-bit hosts.
Replaced with the real formula plus `systemctl show -p EffectiveTasksMax`.

### Significant — two stale pointers

- `limits_linux.go` cited `docs/deployment.md`, which does not exist. The
  content is in `docs/attachment-security.md`.
- `cmdexec.go:67` still described limits as bounding "memory, **processes**,
  file size, CPU" — precisely what no longer holds.

### Overstated containment claim

The new guide bullet said a PID namespace means "a fork bomb cannot escape the
command". Verified that it imposes no numeric ceiling — 300 processes fork
freely inside `bwrap --unshare-all` (`PROCS_IN_NS=305`). Reworded to
containment-and-reaping, with `TasksMax=` named as the actual ceiling.

Also confirmed, and load-bearing for the recommendation: cgroup accounting is
orthogonal to PID namespaces, so tasks inside bwrap still charge the unit's
`pids.max`. `TasksMax=` genuinely bounds the sandboxed converters.

### Accepted, not fixed

The reviewer flagged that the sandbox opt-out path
(`RELA_UNCONFINED_COMMANDS` / `WithSandboxDisabled`) has no PID namespace, so
after this change nothing bounds forks there. It agreed this is not a blocker:
`RLIMIT_NPROC` did not meaningfully protect that path either, since its
threshold was borrowed from unrelated UID-wide load and so fired arbitrarily or
not at all. Scoped the guide's claim to the sandboxed path and pointed
unconfined hosts at `TasksMax=` rather than silently widening this change.

Separately noted: `RELA_UNCONFINED_COMMANDS` is undocumented anywhere in
`docs/`. Pre-existing and left alone rather than folded into this fix.

### On why4

The reviewer judged "not expressible in RLIMIT_NPROC at any value" correct in
conclusion but imprecise: since Linux 5.14 the limit is enforced via
per-namespace `ucounts`, so what a ceiling permits depends on uid and
user-namespace provenance as well as UID-wide load. That strengthens the case —
unreliable as well as mis-scoped — and why4 is reworded to say so.

## Self-review

`git diff` shows the intended removal, its two doc-comment corrections, the
rewritten test, the guide edits with `docs/` regenerated from them, and the
ticket entities. No unrelated change.

Housekeeping from the review run: the `clamtest` Lima VM was used for
verification; `/tmp/relanp` inside it and `/tmp/relasrc.tgz` on the host are
scratch and can be removed.
