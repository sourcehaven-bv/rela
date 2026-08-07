---
id: REV-0QK7Y7
type: review-checklist
title: 'Review: Release workflow Test job never installs bubblewrap'
status: done
---

## Automated checks

- [x] Verified in CI, not locally simulated: Release run **30611964082** (PR
  #1257, snapshot mode) — `Test: success`, and the previously-skipped
`Release` job ran to `success`.
- [x] Before/after is unambiguous: run 30611472878 (same branch, pre-fix)
failed `Test` with ~20 `bwrap: executable file not found` errors and **skipped**
`Release`. Run 30611964082 (post-fix) passes both.
- [x] Workflow YAML parses.

## Code review

- [x] Mirrors `ci.yml` rather than inventing a variant — same install step, same
`bwrap --unshare-all --ro-bind / / /bin/true` sanity probe.
- [x] **`ubuntu-26.04`, not just the install.** `ci.yml` documents that 24.04
restricts unprivileged user namespaces via AppArmor, so `bwrap` fails even when
installed. Installing the package alone would have looked correct and still
failed — caught by reading `ci.yml`'s comment rather than copying only its
install line.
- [x] The sanity probe is retained deliberately: without it, a sandbox that is
present-but-broken lets the confinement tests skip silently, which is the
failure mode the probe exists to prevent.

## Acceptance verification

| Criterion | Result |
| --- | --- |
| Test job installs bubblewrap | **PASS** |
| cmdexec/attachment/transform tests pass in Release workflow | **PASS** — Test: success |
| `release` job reachable, not skipped | **PASS** — Release: success |

## Notes

Found by temporarily giving `release.yml` a `pull_request` trigger to exercise
the BUG-2YZ575 SPA guard. That temporary commit has been dropped from the branch
(`git rebase --onto`); the final diff contains only the two permanent fixes.

This bug and BUG-2YZ575 share a root cause — `release.yml` re-declares setup
that `ci.yml` and the justfile already express, with nothing keeping the copies
in sync. Recorded as remaining scope on [[TKT-O03TB]], along with the argument
that a spawn-and-serve e2e (not a byte-level check) is what covers both failure
classes.
