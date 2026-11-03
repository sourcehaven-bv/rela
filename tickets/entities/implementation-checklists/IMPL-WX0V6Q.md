---
id: IMPL-WX0V6Q
type: implementation-checklist
title: 'Implementation: Trigger and reminder literals through MessageType.NormalizeProps'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestParse_ReminderLiteralPolicy`, `TestParse_TriggerLiteralOrder`, `TestNormalizeProps_PropError`, updated cases in `TestParse_SessionRemindersInvalid` and `TestParse_Invalid`
- [x] Integration tests written (test full flow, not just units): `TestLiteralLoadPassesFireTime` checks that a literal that loads also passes the fire-time checks for reminders and triggers
- [x] Happy path implemented: trigger and reminder literals go through one `validator.literalPost`, which calls `MessageType.NormalizeProps`; `literalProps` and `literalMessage` are gone
- [x] Edge cases from planning handled: `{{` refused; only string, markdown and enum literals; required, enum, max, size and count rules from `NormalizeProps`; `today()` reported first; `when:` checked on the normalized fields; each problem reported once
- [x] Error handling in place (errors surfaced, not swallowed): load errors point at `...post.props.<prop>` through the unexported `propError`; fire-time text unchanged

## Test Quality

- [x] Using fixture builders or factories for test data: realtime config fixtures and `assertOnlyProblemAt`
- [x] No hardcoded values in assertions when object is in scope: the count case is built from `realtimelimits.MaxMessageProperties`
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: load-time config check, covered by parse and load-versus-fire tests)
- [x] Each acceptance criterion verified with test scenario from planning: AC table in the notes maps 1 to 8 to tests
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build on four tags; vet; golangci-lint (0 issues on
all tag sets), arch-lint, plimsoll, comment-lint; `go test -race` on realtimecfg
and realtime; tests of the 16 packages that import realtimecfg; texthazard and
corpus tests; `just docs`; markdownlint; `just build-frontend`; archplan ci;
coverage check.

**Deviations:** the property cap is 32, not 20, and the test reads the constant;
no coverage-ignore comments were needed on the nil guards.

## Quality

- [x] Code follows project patterns (check similar code): button presets already report the first `NormalizeProps` refusal the same way
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): this ticket removes the second copy of the
prop rules
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
