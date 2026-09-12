---
id: TKT-8LZGME
type: ticket
title: Fuzz sweep files one issue per failing target
kind: chore
priority: medium
effort: xs
status: done
---

## Problem

The weekly fuzz sweep filed a single GitHub issue and commented on it every week
thereafter, because dedup matched on the `fuzz-failure` **label** rather than on
the failing target. Issue #993 accumulated 10 comments covering 11 distinct
targets between 2026-06-15 and 2026-09-07.

Two consequences, both costly given the sweep is a productive bug finder
(BUG-J7PXAC, BUG-C9ZYPD, BUG-TOXQAA, BUG-RHFHTH and BUG-B1RA3J all came from
it):

1. **Unrelated bugs share one thread.** A sweep finding three crashes in
three packages is three bugs, each needing its own fix, assignee and close. One
issue cannot be closed until every finding in it is fixed, so the thread never
closes and new findings land in a backlog nobody can triage.
2. **A recurrence is indistinguishable from a new find.**
`FuzzGenerateShortID` failed five weeks running; it reads identically to a
first-time failure appended the same way.

## Approach

Loop over the rows of `fuzz-failures.txt` and file one issue per row. Identity
is the `(package, target)` pair, carried in the title as `Fuzz failure: <Target>
(<package>)` and matched **exactly** against open `fuzz-failure` issues on the
next sweep — a recurrence comments on that target's own issue, a newly broken
target opens a fresh one.

The pair, not the target name alone, is the identity:
`FuzzPropertyValuesTypeZoo` has failed independently on fsstore, memstore and
sqlitestore, and those are separate bugs in separate backends.

Each body carries that one failure's package, target and kind, with the
reproduction command filled in concretely (`go test -run='^FuzzParse$'
./internal/metamodel`) instead of the previous `<Target>`/`<pkg>` placeholders.

Robustness points beyond the rename, several added in response to code review:

- **Per-target failure isolation.** Under `set -e` a transient `gh` error on
the first of three crashes would abort the loop and drop the other two, with the
run already red and nothing retrying until the next Monday. Failures are counted
and reported; the step still exits non-zero.
- **A failed dedup query is not read as "no match"** (RR-LERUY1). Both are the
empty string, and guessing "no match" files a duplicate of an issue that already
exists. The step fails closed: warn, count, skip the target.
- **The summary's last row survives a missing trailing newline** (RR-EAW6EL).
`read` discards a final unterminated line, which would silently drop the
alphabetically-last target at exit 0.
- **`gh label create` cannot abort the step** (RR-513JAO) — a missing label is
cosmetic, an unfiled crash is not.
- **The `error` kind does not advertise crash files** (RR-EKH2XM) that a
non-crash failure never wrote.
- **The issue-list page cap is asserted** (RR-SWV734) rather than assumed
unreachable.
- **The title reaches jq through the environment** (`env.TITLE`), not the filter
string, so a target name cannot break the quoting.

Unchanged: the artifact upload, the `fuzz-failure` label, and the setup-error
path (script exit 2 writes no summary, so no issue is filed).

## Verification

- Step body extracted from the workflow and exercised against real sweep
summaries with `gh` stubbed: recurrence + new targets, the same target across
three packages (must not alias), both `fuzz-crash` and `error` kinds, the
setup-error path, and a mid-loop `gh` failure.
- The exact-title match checked against the live repo: returns `993` for the
exact title, empty for a non-match.
- Regression suite (10 scenarios) re-run after the review fixes: all green.
- actionlint clean (with shellcheck 0.11.0 present, so the `run:` block is
genuinely inspected — confirmed by an injected-fault canary).

Note: nothing in CI lints workflows. TKT-PCLGGL's verification notes claim
actionlint runs via CodeQL's "Analyze (actions)" job, but `codeql.yml` analyzes
only `go` and `javascript-typescript`. Adding an actionlint job is a follow-up,
not in this ticket.
