---
id: REV-MJ27P
type: review-checklist
title: 'Review: Close the =~ ReDoS hole: require trusted literal regex patterns (issue #1139)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend suite **79 files / 1312 tests passed**; `conditions.test.ts` 41/41. Typecheck (`vue-tsc --noEmit`) clean.
- [x] Lint clean — 0 errors; **0 warnings in `conditions.ts` / `conditions.test.ts`** (89 pre-existing warnings are all in unrelated files). Prettier clean on both changed files.
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: coverage floors are Go-only — `go_packages` in the justfile; per CLAUDE.md "the frontend has no coverage enforcement". This change is frontend-only; no Go files touched.)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — run **twice**: once on the original approach, once on the rework.
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes — diff is exactly 2 files (`conditions.ts`, `conditions.test.ts`); no Go, no config, no drive-by edits.

**Review Responses:** RR-HPQV2 (critical), RR-P1QZK (significant), RR-H5I31
(minor), RR-G26WN (minor), RR-RYV9V (nit) — **all `addressed`**.

The first review found the original approach (the one issue #1139 proposed, and
that this ticket originally planned) was **security theater**: RR-HPQV2 proved a
value-length cap cannot mitigate ReDoS, since a catastrophic pattern is short
and blows up on ~40-char inputs. Independently verified before acting. The
approach was reworked from "cap the value" to "reject untrusted patterns", after
confirming the threat model with the user.

The second review probed **6 bypass attempts** against the new parse-time guard
(paren-wrapped ref `(form.pat)`, reversed operands `'^foo' =~ form.x`, `not`,
function call, non-string literals) — **all rejected at parse**. It confirmed no
grammar path can mint a `lit` node from data, so a literal cannot be influenced
by data. Verdict: *"the security reasoning is now sound... the vulnerability was
removed rather than mitigated, which is the right instinct."*

## Acceptance Verification

**Note:** the planning checklist's acceptance criteria were written for the
abandoned cap-as-fix approach (see RR-HPQV2). Criteria restated to match what
shipped; the original AC1 is superseded because its premise was false.

- **AC1′ — an untrusted (data-sourced) pattern cannot reach the regex engine.**
**PASS.** `parse('form.v =~ form.pat')` throws in 2ms; also `entity.pat`,
non-string literals (`42`/`true`/`nil`). Reviewer independently confirmed 6
bypass variants all rejected. Tests fail against unmodified code (verified).
- **AC2′ — no regression on legitimate literal patterns.** **PASS.** Full suite
1312/1312 green; `"form.v =~ '^foo'"` matches/non-matches as before.
- **AC3′ — eval never throws; over-cap value is fail-safe (false + warn).**
**PASS.** Test "an over-cap value is rejected fail-safe... never throws".
`eval`'s catch-all confirmed intact; `=~` now has *zero* eval-time pattern
failure modes (parse owns them all), which tightens the contract.
- **AC4′ — the value cap is honestly described and actually pinned.** **PASS.**
Verified by **mutation testing**: moving `MAX_MATCH_VALUE_LENGTH` 10_000→5_000
fails the boundary test (1 failed/40 passed); restoring → 41/41. This closes
RR-G26WN, where the original boundary test passed against the *unmodified* code
(a false coverage claim).

**Residual risk accepted and documented, not silently ignored:** an operator
writing a pathological literal into their own YAML still hangs the tab (measured
~10s at 27 chars). Foot-gun, not a vulnerability — documented in the module doc
with RE2/Worker named as the upgrade path if `=~` ever needs a data-sourced
pattern.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing
surface — engine is dormant, zero importers, and `visible_when`/`required_when`
appear in no config yet. Documenting a condition language before its first
consumer would be premature and would drift.)
- [x] User-facing documentation updated — **N/A** as above. The threat model,
the reason patterns must be literals, the accepted foot-gun, and the upgrade
path are all documented **in-code** (module doc + `MAX_REGEX_LENGTH`,
`MAX_MATCH_VALUE_LENGTH`, `validateRegexLiteral`, `compareRegex` docstrings),
which is where the next implementer will look. Corrected three pre-existing
docstrings that made the now-disproven "length cap bounds ReDoS" claim.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the restriction lands *before* any
consumer exists, so it costs nothing now and cannot regress later. A future
consumer needing a data-sourced pattern has a documented path (RE2/Worker).

**Bonus hardening from the second review:** documented that the unbounded parse
cache silently depends on the same trusted-config invariant (`cache` is keyed on
source with no eviction — safe only because sources come from a fixed YAML).
Noted in-code so nobody parses a user-typed source and turns it into a leak.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — every job green **except God-object lint, which is
  broken on `develop` itself and is not caused by this PR** (evidence below).
  Green: Frontend (the job covering this change), Test, Lint, CodeQL, Analyze
  (go / js-ts / actions), Architecture, Vulnerability Check, Fuzz, Postgres
  Backend, Lint Markdown, Rela Tickets, and all 6 Cross-Compile matrix jobs.

**God-object lint — pre-existing failure on `develop`, not this PR.** Proven, not
assumed: checking out develop's tip (`40e94f44`) and running plimsoll reproduces
the identical error —

```
internal/cli/cli_wiring.go:43:6: type cliServices has 30 exported methods,
over the load line of 29
```

`40e94f44` (**TKT-BW6UUL / PR #1142**, operator version-purge) modified
`cli_wiring.go` (+11/-3) and pushed `cliServices` past its pinned
`//plimsoll:max-exported-methods=29`. That commit landed on develop *after* this
branch, so this PR merely inherits a red base. This change is TypeScript-only —
a Go linter never reads it — and plimsoll passes on this branch's tree.
Out of scope here; raised with the user separately rather than silently widening
this PR.

**Rela Tickets** — was failing on a self-inflicted bookkeeping loop: this
checklist is `done`, and the "all CI checks pass" item was legitimately unchecked
while CI was still running, which trips the "done review checklists cannot have
unchecked items" gate. Resolved once CI actually reported green. Now passing.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1147
