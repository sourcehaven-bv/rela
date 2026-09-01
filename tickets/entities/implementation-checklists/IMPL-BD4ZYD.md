---
id: IMPL-BD4ZYD
type: implementation-checklist
title: 'Implementation: Operator-configured recipient allowlist for mail.send'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Four pieces, split across the arch-lint boundary rather than against it:

1. **`mail.RecipientConfig`** parses and validates the `recipients:` block.
Validation runs from `validateCommon`, so it applies to every transport — which
server carries the mail has no bearing on who may receive it.
2. **`mail.LuaSender` carries the policy** and satisfies
`lua.RecipientPolicyCarrier`. It already captures `from` from operator config
for the same reason: a script must not choose who may receive mail any more than
it chooses who it comes from.
3. **`lua.checkRecipients`** enforces, called from `luaMailSend` after the
message parses and before the transport sees it.
4. **`isDomainPattern`** accepts exactly a leading `*@`; anything else with a
`*` is refused at LOAD.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Tests drive `mail.send` rather than the checker, so the WIRING is covered along
with the logic — which mattered, see below.

The existing `mail_test.go` cases needed a change: every one of them sends
without a policy, so deny-by-default failed all seven. They now wrap their
sender in an explicit `allowAnySender`, with a comment saying why. That keeps
them testing the send path rather than silently testing the allowlist, and
`TestRecipients_UnconfiguredDenies` still fails if the default ever flips —
verified by mutation, so the accommodation did not weaken the guarantee.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| mutation | expected | observed |
| --- | --- | --- |
| unconfigured policy permits | AC1 reddens | `TestRecipients_UnconfiguredDenies` FAIL |
| `strings.HasSuffix` instead of domain comparison | AC4 reddens | both `..._IsNotASuffixMatch` subtests FAIL |
| non-carrier sender permits | AC8 reddens | `TestRecipients_SenderWithoutPolicyDenies` FAIL |
| `LuaSender` stops carrying the policy | AC9 reddens | `TestLuaSender_CarriesRecipientPolicy` FAIL |

The second row is the security-critical one: with a suffix test,
`attacker@evil-example.com` matches `*@example.com`. That is the classic
allowlist bypass, and the test catches it.

**A gap the mutations found.** Running the non-carrier mutation initially
reddened NOTHING — every test used a sender that did carry a policy, so making
the fallback permissive would have shipped unnoticed. That is precisely the
"transport written before this feature, or a test double that never thought
about it" case the design comment names.
`TestRecipients_SenderWithoutPolicyDenies` was added and the mutation now
reddens it.

**A worse gap found by inspection, not by tests.** After the enforcement was
complete and green, `grep` for implementations of `RecipientPolicyCarrier`
returned NOTHING: the check was correct and entirely inert, because no sender
carried a policy. Every test built one directly, so the suite was green and
production had no allowlist at all. Fixed by having `LuaSender` carry it, and
pinned by AC9 so it cannot come loose again.

Worth recording as the lesson: for a control split across two packages, "the
logic is tested" and "the control is in force" are different claims. The second
needs its own test at the seam.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The parse/enforce split respects `.go-arch-lint.yml` rather than working around
it: `internal/mail` may not depend on `filter` or `store`, deliberately, so a
send script has no graph access "by construction rather than by convention". The
rescope away from graph queries is downstream of that, and the boundary stayed
intact.

`permits` scans linearly over `AlsoAllow` rather than using a set. That is not
an oversight: the list is operator-written config — a handful of entries — and
half of them are patterns a map lookup could not answer anyway, so a set would
need the pattern scan beside it regardless.

The domain comparison takes the address's domain after the last `@` rather than
testing a suffix, and says why in a comment naming the bypass it prevents.
