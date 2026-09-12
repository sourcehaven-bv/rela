---
id: REV-T2JNGC
type: review-checklist
title: 'Review: Compile build-tag-gated test files in CI — they rot silently today'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test -race -cover -shuffle=on ./...` — 107 packages ok, 0 failures.
`golangci-lint run` — 0 issues. `just comment-lint` — gate clean, no
unresolvable doc links across 14032 comments. `just coverage-check` — package
and total floors PASS, total 79.4%. `internal/dataentry` is 81.8% against a 55
floor; deleting a tag-gated file cannot move it, since coverage is measured
from the untagged run where that file never compiled.

Also run, being the point of the change: `just tagged-tests` (5 tags, clean) and
`just tagged-tests-test` (30/30). `shellcheck` clean but for one SC2001 style
note on a line copied verbatim from scripts/check-embedded-spa-test.sh;
`gofmt -l` and `go vet` clean on scripts/tagged_build_tags.go.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The review was worth having: it found the guard reproducing its own failure
mode. A constraint written `//go:build<TAB>e2e` was not matched by the sed
patterns, so a rotted file produced "nothing to compile" and exit 0 — the exact
silent pass the guard exists to prevent. Rather than patch the patterns, which
would have left the next gap, discovery moved to Go's own
`go/build/constraint` parser.

**Review Responses:** RR-GKZELE (critical, addressed), RR-R7BL0Z (critical,
addressed), RR-MEWZO5 (critical, addressed), RR-8J2JTV (significant,
addressed), RR-FKC4QB (significant, addressed), RR-RNF9DA (significant,
addressed), RR-0603SI (minor, addressed), RR-Y4GMZS (minor, addressed),
RR-CKLDFT (minor, addressed), RR-POACUV (minor, addressed), RR-P054UY (minor,
deferred — cross-GOOS vetting wants its own ticket), RR-VVUH9Z (minor,
wont-fix — reasoned).

No critical or significant finding remains open. The diff contains no unrelated
changes: it is the guard, its test, the discovery tool, the CI wiring, two
justfile recipes, the deleted test, and the ticket entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all six PASS. Against PLAN-FFV3WC:

1. A non-compiling tagged file fails CI — **PASS**. "tagged file with stale call
   signature FAILS", plus the real article: restoring the historical
   e2e_test.go makes the guard discover `e2e` and fail on the original NewApp
   error (exit 1).
2. The same tree passes `go build ./...` and untagged `go vet ./...` — **PASS**.
   Both asserted in the suite, so the guard is demonstrably not duplicating an
   existing signal.
3. Tag list derived, not hardcoded — **PASS**. Discovery cases for plain tags,
   dedup/sort, or-expressions, parens, both syntaxes and their precedence.
4. Negated and platform tags are not opt-in — **PASS**, including the
   `!(a || b)` scope case the first draft got wrong.
5. Empty match exits 0 — **PASS**, and `tags` prints no lines rather than a
   blank one.
6. e2e_test.go no longer rots — **PASS**. Deleted; the tag disappears from
   discovery and the guard stays green.

Scope grew during review: the guard now covers tagged PRODUCTION files too
(`sqlite`, `memorybackend`), which were uncovered for the same structural
reason.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: internal CI guard)
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

**Docs Checklist:** N/A. This adds a CI step and two `just` recipes; it changes
no command, flag, schema or UI. The guard and the discovery tool carry their
rationale in their own headers, and `just --list` surfaces both recipes. No
`docs/` page enumerates CI jobs, so there is nothing to keep in sync.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

No TODO or FIXME introduced. A developer hitting this in CI gets the failing
command, the reason no other step would have caught it, and `just tagged-tests`
to reproduce locally; `just tagged-tests-test` covers changing the parser.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
