---
id: IMPL-76TBSC
type: implementation-checklist
title: 'Implementation: ACL test coverage for per-recipient scheduled mail rendering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Test-only. No production code changed — the behaviour was already correct; it
had nothing pinning it.

Two tests in a new `internal/appbuild/scheduled_mail_acl_test.go`, both driving
the exported `RunScheduledTemplate` under a real `acl.Declarative` and
`visibility.PolicyRedactor` instead of the existing test's `NopRedactor`:

- `TestRunScheduledTemplate_RedactsPerRecipientACL` — a role denying `read: task`
loses the ROW; a wider role keeps both the row and the gated field.
- `TestRunScheduledTemplate_RedactsFieldOnAVisibleRow` — a role that CAN read the
row still loses a field its `visible:` block does not grant.

Deliberately two tests, not one. A denied row hides its fields for free, so a
single test cannot distinguish "the field was redacted" from "the whole row was
gone" — and the mutation results below show that split was load-bearing.

Asserted on the RENDERED TEXT rather than the model from `mailtemplate.Build`.
The claim is about what the recipient receives, so asserting a layer up would
leave a renderer that reintroduced a redacted value uncaught.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Reuses `staticConfig` and the `Services` fixture shape from the existing
`scheduled_mail_test.go`, so the new tests differ from it in exactly the
dimension under test. The ACL stack is built with the four-line recipe from
`internal/visibility/visibilitytest/suite.go:150-170` rather than a second way
of doing it.

Separate FILE rather than extending the existing test file: the policy YAML,
relation-lookup shim and redactor construction are substantial enough that
mixing them in would obscure what the original test is for.

Every test carries a positive control.
`TestRunScheduledTemplate_RedactsPerRecipientACL` asserts the `lead` recipient
DOES receive the row and the secret before asserting the `viewer` does not, and
the field test asserts the title survives. Absence assertions pass trivially
against a renderer that emits nothing, and "the redaction test quietly stopped
rendering" is the standard way this class of test dies without anyone noticing.

Each also asserts the `To` address, so a fan-out bug that renders the right
content for the wrong person cannot pass.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

A test-only ticket has one real hazard — that the new tests pass without
exercising the mechanism they claim to cover. So the mutations here are against
the PRODUCTION path and the POLICY, not the tests:

| mutation | expected | observed |
| --- | --- | --- |
| `scriptEntityReader` returns `visibility.Unrestricted` (ACL bypassed) | both redden | both FAIL |
| `NewPolicyReader` gets `NopRedactor`, row gate intact | only the FIELD test reddens | `..._RedactsFieldOnAVisibleRow` FAIL, alone |
| promote alice from `viewer` to `lead` in the policy | the row-denial test reddens | FAIL — so the per-recipient principal really reaches the ACL |
| grant `viewer` `read: task`, leaving `visible:` narrow | alice sees the title, NOT the secret | FAIL with `"title: Quarterly review\nsecret: "` — the row appears and the field renders EMPTY |
| `RenderedFor: ""` in the sender call | the attribution assertion reddens | FAIL |
| restored | green | ok |

Rows 3 and 4 are the ones that settle the question a redaction test usually
leaves open. Row 3 proves roles are applied PER RECIPIENT rather than every
principal resolving to the same thing — a test where they collapsed would pass
while proving nothing. Row 4 is the clearest evidence in the set: with `read:`
granted, the title appears and `secret:` renders empty in the same message. That
is `read:` gating the ROW and `visible:` redacting the FIELD, visibly
independent, which is exactly the pair PLAN-XMWT23 AC7 asked for.

Row 2 shows the two tests cover distinct mechanisms rather than one standing in
for both — had the field test also survived it, it would have been re-testing
row denial under another name.

Note the first attempts at the first two mutations silently failed to apply
(whitespace mismatch in the patch) and the suite stayed green. That green proved
nothing, and taking it at face value would have been the exact error this ticket
exists to correct. Re-ran both against the real code.

The code-review agent for this change died mid-run without producing findings,
so the verification above is my own rather than a second pair of eyes. Flagging
that rather than implying a review happened: rows 3-5 are the checks the review
would have been asked for, done by hand.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: `mustScheduledMailACL` factors out the four-step ACL construction for the
shared-policy test. The second test builds its policy inline, because its role
set is the POINT of that test (a role that reads the row but not the field) and
hiding it behind a helper would make the test unreadable. Two similar blocks is
the right call here over a parameterised helper that says less.

The `aclRelationLookup` shim mirrors the existing ones in
`affordances/features_test.go` and `visibilitytest/suite.go`. Not extracted to a
shared package: it is four lines, and the two existing copies already establish
that per-package test shims are this codebase's convention for it.

No production code changed, so no security surface moved. The security VALUE is
that a cross-recipient disclosure in scheduled mail — silent to everyone except
the person who should not have received it — now fails CI instead of shipping.
