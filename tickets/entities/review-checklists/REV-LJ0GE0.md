---
id: REV-LJ0GE0
type: review-checklist
title: 'Review: classify the unreached set with reasoned coverage-ignore directives'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Architecture clean (`just arch-lint`)
- [x] Reachability pipeline runs clean (`./scripts/reachability.sh`)

**Evidence.** `just test` exit 0, 107 packages ok, 0 FAIL. `just lint` exit 0,
"0 issues". `just arch-lint` exit 0, "OK - No warnings found". `go build ./...`
clean. `./scripts/reachability.sh` exit 0: 79.1% reached (46,156/58,326), 1,187
statements dismissed.

**Lint note.** The directives initially produced 196 `lll` violations because the
reasons are long. Reasons were wrapped onto continuation comment lines rather
than shortened — the justification is the point of `--require-reason`, so
truncating it to satisfy a line limit would defeat the gate. `lll` uses
`tab-width: 4`, so wrapping had to be measured in display columns, not bytes.

## Manual Review

- [x] Every directive reattached to the construct it classified
- [x] Every directive validated against a real coverage profile
- [x] Stale directives dropped rather than reinterpreted
- [x] Self-reviewed the diff for unrelated changes

**Reattachment.** The branch forked from a 2026-07-22 merge base, 316 commits
behind; 84 of 109 annotated files had changed upstream. 32 files conflicted.
Resolution took upstream code verbatim and reattached each directive to its
construct. 19 directives were dropped because the construct is gone (8 from the
Wails v3 `runtime.*` API removal in `cmd/rela-desktop`, the rest from upstream
restructuring of the annotated guard). None was moved to a neighbouring
construct. With the 3 stale ones below, 22 of the branch's 282 directives were
dropped and 260 survive.

**Profile validation.** Directives were checked against the merged profile, not
accepted on trust:

- 106 single-line dismissals: 0 sit on a reached statement. The check is
  column-aware; a naive line-level check reports 37 false positives because a
  dismissal on an `if err != nil {` line shares that line with the reached
  condition block while dismissing the unreached body.
- 161 `-start`/`-end` blocks: 112 fully unreached, 27 partially reached, 19
  statement-free.
- 3 blocks were fully reached and were **dropped as stale**:
  `internal/config/config.go`, `internal/predicate/eval.go`,
  `internal/store/fsstore/watcher.go`.

**Dialect.** Verified empirically on a throwaway module that `-d coverage-ignore`
does not honour `//scupper:ignore` and vice versa, so the branch's spelling
would have been inert under the landed pipeline. Conversion preserves each
reason verbatim. This matches the decision TKT-DWO4ZB recorded: one vocabulary,
shared with `go-test-coverage`.

**Comment findings.** During wrapping, 4 directives had run into a pre-existing
doc comment below them, making the two read as one sentence
(`cmd/rela-server/main.go`, `internal/canonical/canonical.go`,
`internal/lua/json.go`, `internal/store/pgstore/feed.go`). A `//` separator was
restored in each. Both texts are intact and no prose was deleted.

**Review Responses:** None recorded. Points examined:

- *Is dropping `scripts/coverage-generous.sh` in scope?* Yes — it is the
  predecessor of the landed `scripts/reachability.sh` and would otherwise be a
  second script claiming the same job, unwired to CI.
- *Does the report overstate what was measured?* It now states the e2e and
  postgres legs did not run and marks the affected packages as understated, so
  the figures read as a lower bound rather than a verdict.

## Sign-off

- [x] Ready to merge

Report-only; no threshold is enforced, so this cannot break CI on coverage.
