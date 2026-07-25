---
id: BUG-1VVXHZ
type: bug
title: 'CI Fuzz job flakes on slow runners: Go 1.25+ exits non-zero with ''context deadline exceeded'' when -fuzztime expires'
description: 'The Fuzz CI job fails intermittently on slow runners with ''context deadline exceeded'' — a -fuzztime budget expiring (Go 1.25+ exits non-zero for this), not a discovered crash. It failed twice in one afternoon on PRs touching no fuzzed code, and GitHub''s default bash -e also skipped every target after the first. Fixed by classifying the failure: only ''Failing input written to'' (reproducer evidence) fails the build; a deadline-exceeded run is tolerated with a notice; anything else still fails loudly.'
priority: medium
why1: The Fuzz CI job failed on PRs that touch no fuzzed code (twice in one afternoon, on FuzzParseEntityID then FuzzValidateID).
why2: '`go test -fuzz -fuzztime=10s` exited non-zero with ''context deadline exceeded'' — the time budget expiring while a worker was mid-execution, not a discovered crash.'
why3: Since Go 1.25 (repo is on 1.26) an expired -fuzztime budget can exit non-zero instead of stopping cleanly; GitHub's default `bash -e` then failed the step and skipped every remaining target.
why4: The step ran the three targets as bare sequential commands with no distinction between 'budget expired' and 'found a failing input' — both are simply a non-zero exit.
why5: CI treated an exit code as the signal of correctness without encoding what the tool actually means by it; a timeout and a security finding were indistinguishable to the pipeline.
prevention: 'The step now classifies the failure instead of trusting the exit code: a run is only failed when the output contains ''Failing input written to'' (reproducer evidence), a ''context deadline exceeded'' run is reported as a notice and tolerated, and anything else still fails loudly. Comments at the call site pin the non-obvious trap — the timeout output ALSO contains ''--- FAIL'', so matching on that generic marker would re-break the exact case being tolerated.'
status: done
---

## Symptom

The `Fuzz` CI job failed on PRs that changed nothing in the fuzzed packages:

- PR #1197 — `FuzzParseEntityID`, `internal/entity`, `context deadline exceeded` at 10.09s
- PR #1200 — `FuzzValidateID`, `internal/entity`, `context deadline exceeded` at 11.00s

`git diff origin/develop...HEAD` touched **zero** files under `internal/entity`
in either PR. Both cleared on re-run with no code change.

## Root cause

Since Go 1.25 (this repo builds on 1.26), a `-fuzztime` budget that expires
while a worker is mid-execution exits **non-zero** with `context deadline
exceeded` rather than stopping cleanly. That is the timer working as configured,
not a finding.

The runner speed makes it environment-dependent: the failing runs managed **~14k
execs/sec**, versus ~136–160k/sec locally. I could not reproduce it locally even
with all 10 cores saturated (still ~74k/sec) — it needs a genuinely slow shared
runner.

Because the step ran three bare `go test` commands under GitHub's default `bash
-e`, the first timeout failed the whole step and skipped the remaining targets.

## Fix

A `run_fuzz` shell function that classifies the failure rather than trusting the
exit code:

| Output | Outcome |
|---|---|
| exit 0 | pass |
| contains `Failing input written to` | **fail** (`::error::`) — a real finding, reproducer written |
| contains `context deadline exceeded` | pass (`::notice::`) — budget expired on a slow runner |
| anything else (build error, panic, …) | **fail** — never silently swallowed |

**The trap, pinned in a comment:** the timeout output contains `--- FAIL:
<target>` *and* `context deadline exceeded` together. An earlier draft grepped
`Failing input written to|--- FAIL` and would have re-failed the exact case it
was written to tolerate. The crash check must be a positive match on reproducer
evidence, not on a generic failure marker.

## Verification

Each branch exercised against realistic output:

- real captured CI timeout output → exit 0 (tolerated)
- real crash output with a reproducer path → exit 1 + `::error::`
- build-error output → exit 1 + `::error::` (unrecognised)
- a genuinely-failing fuzz target in a scratch module → exit 1 (the end-to-end proof that findings still break the build)
- all three production targets run end-to-end through the new helper → pass

YAML parses; the extracted shell passes `bash -n`.

## Notes

No `github.event.*` interpolation is introduced — the step uses only static
literals, so the workflow-injection class does not apply.
