---
id: IMPL-IWIXTF
type: implementation-checklist
title: 'Implementation: Apply field-level visible: redaction to the appbuild gated read paths'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Six tests in `internal/appbuild/fieldredaction_test.go`. The cascade test is a
genuine integration test: real `PatchEntity` → real automation trigger → real
inline-Lua action → store re-read, no doubles anywhere.

Error handling is the load-bearing part. `buildFieldRedactor` returns an
**error** when a policy declares affordance grants but fails to compile, and
both construction paths propagate it to abort startup. Degrading to
`NopRedactor` there would silently serve unredacted data to an operator who
had explicitly asked for redaction (RR-GKCZO5).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`writeRedactionProject` is the shared fixture. The metamodel declares exactly
two properties — one visible, one hidden — because the pair is the whole test:
asserting only on the hidden one would pass against a redactor that hid
everything.

**All four redaction tests are mutation-verified.** Reverting the redactor to
`nil` makes each fail on its own assertion; restoring makes them pass. Verified
again after rebasing onto develop (33 commits), since `appbuild.go` had been
refactored underneath.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Test | Result |
|---|---|---|
| MCP read redacts | `TestGatedReads_RedactsHiddenField` | PASS (fails without fix) |
| List path redacts | `TestGatedReads_RedactsOnListPath` | PASS (fails without fix) |
| Scheduler redacts (RR-7408F5) | `TestScheduledLuaWriteDeps_RedactsHiddenField` | PASS (fails without fix) |
| Cascade redacts | `TestCascadeReadDeps_RedactsHiddenField` | PASS (fails without fix) |
| No `acl.yaml` → byte-parity | `TestNoPolicy_RedactorHidesNothing` | PASS |
| Row-only policy → no redaction | `TestPolicyWithoutAffordanceGrants_HidesNothing` | PASS |
| Both KNOWN LIMITATION notes deleted | godoc diff | PASS |
| `just arch-lint` | full run | PASS |

The cascade case is the sharpest evidence: without the fix it writes
`leaked = "99000"` — the hidden salary laundered onto a field with no
restriction, where any reader sees it. That is the leak, demonstrated rather
than argued.

**A wrong diagnosis I had to correct.** I first reported the cascade path as
untestable because "inline-Lua cascade writes don't persist." That was wrong:
`rela.update_entity` takes the properties table **directly** as argument 2, and
I passed `{properties = {...}}`, which set a property literally named
`properties` — a successful write that changed nothing observable, with no
error. The tell was in my own probe output (`ok=true` plus a returned table),
which I read past. No rela bug; a wrong call signature in my harness.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The resolver is a **private field, not an accessor**: `Services` sits exactly at
its plimsoll `max-exported-methods=24` ceiling, so an accessor breaks CI — and
a field is the better design regardless, since it preserves the
construct-then-publish discipline `WithMachines` requires.

DRY: `storeRelationLookup` is knowingly duplicated (appbuild cannot import
dataentry). Not extracted here because the consolidation belongs in
[[TKT-0XL8MF]], which already reshapes this seam — see RR-4XPL8N. Both copies
carry a cross-reference so a fix to one is not missed in the other.
