---
id: REV-ONBZF2
type: review-checklist
title: 'Review: FuzzParse fails a user''s own key back at itself when the key looks like a YAML tag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...`)
- [x] Lint clean (`golangci-lint run internal/metamodel/...`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: test-only change, no production code touched, so no package can lose coverage)

`go test ./...` green. `go build ./...` and `go build -tags sqlite ./...` both
green. `just arch-lint` reports no warnings. `just comment-lint` clean across
14026 comments. `just plimsoll` clean. `golangci-lint run
internal/metamodel/...` 0 issues.

**Full local fuzz sweep** (`FUZZTIME=25s scripts/fuzz-all.sh`, 52 targets) run
against this branch: 2 flagged.

1. `./internal/metamodel FuzzParse` — the finding this change fixes. Was run
before the fix was applied; re-verified after: the minimized input passes as a
committed seed, and a 90s dedicated fuzz run is clean (it previously failed in
2.4s).
2. `./internal/store/sqlitestore FuzzAttachmentKeyCollision` — **not a defect
and not related**. The failure is `context deadline exceeded` with no crashing
input written, produced while the machine was concurrently running `go test
./...` and two full builds. The target passes in isolation (`-fuzztime=30s`,
clean) and passed in CI's own sweep of the same run that filed this batch.

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: autonomous scheduled run; self-review recorded below)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Self-review.** Three things were worth a second look:

1. *Fix the oracle or fix the loader?* The reflex on a fuzz-crash is to change
production code. Wrong here: `checkUnknownKeys` formats the offending key with
`%q`, and naming the key is the whole value of the message — without it the
operator learns only that *some* top-level key is wrong. Suppressing the "leak"
would mean refusing to say which key. The four `!!`-shaped seeds already in the
corpus were fixed the other way (humanize the yaml error so no tag reaches the
prose), which is right when there is prose to fix; here there is none.

2. *Is this just disabling the check?* That is the real risk, and the reason
`TestStripQuotedEchoes` exists rather than trusting the fuzz target to notice.
It pins both directions across six cases, including the two that distinguish a
considered fix from a blanket one: `entity "task": cannot unmarshal` must still
be flagged (leak beside a quoted echo), and `property "cannot unmarshal" has
unknown type "string"` must not be (yaml language quoted as a property name).

3. *Length preservation.* `stripQuotedEchoes` replaces with spaces rather than
deleting, so any line/column offsets a failure message prints stay meaningful.
Cheap, and it keeps the failure output honest.

Scope check: the diff is one test file plus one seed corpus entry. No production
code changed, no behaviour change to `Parse`.

## Verification

- [x] Fix verified against the reported reproduction
- [x] Regression test added
- [x] No unrelated changes in the diff

The minimized input is committed as
`internal/metamodel/testdata/fuzz/FuzzParse/bug-c9zypd-tag-shaped-user-key`, so
it now runs as a plain regression test on every `go test ./internal/metamodel/`.
