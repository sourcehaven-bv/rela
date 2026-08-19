---
id: REV-8HT4XN
type: review-checklist
title: 'Review: Postgres CI misses two tagged packages; the cross-process SSE test has never run'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — all four commands in the extended step green under `-race` against PostgreSQL 15; full default suite green
- [x] Lint clean — `golangci-lint run ./...` and `--build-tags postgres ./...` both 0 issues; `just arch-lint` OK
- [x] Coverage maintained — `just coverage-check` exit 0 (no Go source changed; workflow + tickets only)

## Code Review

- [x] Self-reviewed the diff — one workflow step extended by two lines plus explanatory comment; no Go source changes
- [x] ~~All critical review-responses addressed~~ (none raised)
- [x] ~~All significant review-responses addressed~~ (none raised)

**Checked for duplicate work before opening:** all 9 open PRs listed (none touch
CI or the postgres tag), the Postgres job on current `origin/develop` inspected
directly, and the ticket corpus grepped for the affected packages. `RR-9ZB7ZO`
matched on "postgres-tagged" but concerns benchmark comments — unrelated.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented

| Claim | Evidence |
|---|---|
| Two tagged packages were uncovered | `grep -rl '^//go:build postgres'` yields 6 packages; the job ran 4 |
| List is now complete | re-ran the grep against the patched step: every tagged package covered (`appbuild/...` subsumes `backendtest`; `pgstore` has its own step) |
| New coverage catches a real regression | removed `l.store.emit(fe.ev)` → `TestStoreEventBridgeCrossProcessSSE` FAILS in 5.2s |
| Nothing was actually broken | all four commands pass unmutated under `-race` |
| Cost is acceptable | ~14s added to the job |

**A correction worth recording.** My first mutation runs appeared to pass and I
initially read the test as vacuous. That was Go serving cached results: re-run
with `-count=1`, the emit-disabled mutation fails as it should. Two further
mutations (swapping `kind`/`op`; dropping the self-echo filter) genuinely do
pass, and correctly so — the field swap is symmetric across encoder and decoder,
and self-echo is *local* double-emission, which a two-store test cannot observe.
The test is sound; only its absence from CI was the defect.

## Documentation (enhancements only)

- [x] ~~User-facing documentation updated~~ (N/A: CI-only change, no user-visible surface)
- [x] ~~Docs-checklist created~~ (N/A: not an enhancement or docs ticket)

## Final Checks

- [x] Commit message explains the why (same systemic cause as BUG-3KQW7P: a hand-maintained package list with nothing tying it to the tagged-file set)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the enumerating grep is in the workflow comment

**Known limitation, stated rather than papered over:** completeness is enforced
by a documented invariant and an inline grep, not an executing check. A guard
test comparing the two sets is the stronger fix; it needs a home outside any
tagged package and must parse the workflow YAML, so it is recorded as a
follow-up on BUG-8HT4XN instead of being half-built here.

## Pull Request

- [x] PR created and CI monitored
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1383
