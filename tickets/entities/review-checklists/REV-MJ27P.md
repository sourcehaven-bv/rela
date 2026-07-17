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
- [x] All CI checks pass — every job green.

**God-object lint — was a pre-existing `develop` failure; fixed here on request.**
Diagnosed, not assumed: checking out develop's tip (`40e94f44`) and running
plimsoll reproduced the identical error —

```
internal/cli/cli_wiring.go:43:6: type cliServices has 30 exported methods,
over the load line of 29
```

`40e94f44` (**TKT-BW6UUL / PR #1142**, operator version-purge) added
`cliServices.Audit()` for the purge commands' forensic trail — 29→30 exported
methods — but left the `//plimsoll:max-exported-methods=29` pin at 29. Every
branch cut from develop inherited the red.

**Fix (`internal/cli/cli_wiring.go`):** merged develop in and bumped the pin to
30. Weighed against the alternatives first:
- *Drop the method?* No — kong injects `*cliServices` into every `Run`, so
  `history_purge.go:82,144` can only reach the audit sink through it.
- *Decompose?* Not here — 38 binding sites / 113 usages. That is **TKT-N0IKN9**,
  which this type's TODO already names. Bumping a grandfathered pin is precisely
  the mechanism CLAUDE.md documents for offenders awaiting a ratchet.

Also fixed the **drift that caused it**: the count lived in two places (the
directive *and* the TODO text, both "29"), so a new method only tripped one.
Both now move together, with a note that the pin is a ceiling, not a budget —
a second bump means decompose instead.

**Verified by mutation:** a 31st method still fails (`exit status 3`), so the pin
is a real ceiling, not a silenced error. `go build`, `go vet`, `just arch-lint`,
`go test ./internal/cli/` all pass; frontend suite still 41/41 after the merge.

**Rela Tickets** — was failing on a self-inflicted bookkeeping loop: this
checklist is `done`, and the "all CI checks pass" item was legitimately unchecked
while CI was still running, which trips the "done review checklists cannot have
unchecked items" gate. Resolved once CI actually reported green. Now passing.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1147
