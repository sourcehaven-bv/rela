---
id: PLAN-8SOH31
type: planning-checklist
title: 'Planning: ACL test coverage for per-recipient scheduled mail rendering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: tests driving `RunScheduledTemplate` under a real `acl.Declarative` and
`visibility.PolicyRedactor`, asserting that a denied ROW and a redacted FIELD
are both absent from the rendered mail.

OUT: implementation changes. The behaviour is correct; it simply had nothing
pinning it. If the tests had found a real leak this would have become a
different ticket, which is worth saying out loud — a test-only ticket is a claim
about the code, and the claim has to be checked before it is asserted.

**Acceptance Criteria:**

1. A recipient whose role denies read on the section's entity type gets NO row.
*Test:* `viewer` (no `read: task`) receives a digest over tasks; assert the task
title is absent.
2. A recipient whose role grants the row but not a field gets the row WITHOUT
the field. *Test:* `reporter` (reads task, `visible:` grants only `title`);
assert the title is present and the secret is absent.
3. A recipient with the wider role still sees both.
*Test:* `lead` receives the row AND the secret field.
4. The message under test is addressed to the recipient it claims to be.
*Test:* assert the `To` address, so a fan-out bug that renders the right content
for the wrong person cannot pass.

AC3 is what makes AC1 and AC2 mean anything. Absence assertions pass trivially
against a renderer that emits nothing, and "the redaction test stopped rendering
at all" is the standard way this class of test silently dies.

AC2 must be a SEPARATE test from AC1, not another assertion in it: a denied row
hides its fields for free, so one test cannot distinguish "the field was
redacted" from "the whole row was gone".

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — test-only.

**Existing Solutions:**

- `internal/visibility/visibilitytest/suite.go:150-170` is the in-repo recipe
for building a real policy stack in a test: unmarshal the YAML,
`acl.NewDeclarative`, `affordances.New`, `visibility.NewPolicyRedactor`. This
ticket follows it rather than inventing a second way.
- `internal/affordances/features_test.go:479` has the `RelationLookup` shim over
a store; the same two methods are needed here.
- The existing `TestRunScheduledTemplateSendsRenderedRecipientMessage` supplies
the `Services` fixture shape (cfgLoader, mailRuntime, memory sender), so the new
tests differ from it in exactly the dimension under test.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Two tests in a new `scheduled_mail_acl_test.go`, each building a `Services` with
a real `aclDeclarative` + `PolicyRedactor` instead of the existing test's
`NopRedactor`, then calling `RunScheduledTemplate` under a principal-stamped ctx
and asserting on the rendered text.

Separate file rather than extending `scheduled_mail_test.go`: the ACL
scaffolding (policy YAML, relation lookup, redactor construction) is substantial
enough that mixing it in would obscure what the original test is for.

**Alternatives considered:**

- *Assert on the model from `mailtemplate.Build` rather than the rendered text.*
Rejected: the claim is about what the RECIPIENT receives. Asserting one layer up
would leave a renderer that reintroduced a redacted value uncaught.
- *One test with both a denied row and a redacted field.* Rejected — see AC2.
- *Reuse `visibilitytest`'s fixture wholesale.* Rejected: it carries a large
seeded graph shaped for a different suite, so the policy under test would be
hard to read against it. Copying the four-line construction recipe is clearer
than importing an unrelated world.

**Files to modify:**

- `internal/appbuild/scheduled_mail_acl_test.go` (new)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** none new — this adds tests.

**Security-Sensitive Operations:**

The operation being COVERED is security-sensitive: per-recipient ACL scoping on
a fan-out that mails data to multiple parties. The failure mode is a
cross-recipient disclosure that is silent to everyone except the person who
should not have received it.

That is why the mutation checks matter more than usual here: a test that asserts
absence and no longer exercises the mechanism looks identical to a passing one.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one per acceptance criterion, all through the exported
`RunScheduledTemplate` so the whole path (build → render → send) is covered.

**Edge Cases:**

- a recipient who may read the row but not one field (AC2) — the case a
row-denial test hides.
- a recipient with the wider role (AC3) — the positive control.

**Negative Tests:** AC1 and AC2 are the negatives; AC3 keeps them honest.

**Mutation plan** — each must redden only the expected test:

1. make `scriptEntityReader` return `visibility.Unrestricted` → both redden.
2. pass `NopRedactor` into `NewPolicyReader`, keeping the row gate → only the
FIELD test reddens.

The second is the important one: it proves the two tests cover distinct
mechanisms rather than one test standing in for both.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The test passes without exercising ACL.* The main risk for a redaction test,
and the reason for both the positive control and the mutation checks.
- *The test asserts current behaviour rather than intended behaviour.* Mitigated
by deriving the expectations from PLAN-XMWT23 AC7, which predates and is
independent of the implementation.

**Effort:** s

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — no behaviour, config or interface changes. The documentation that
matters is in the test comments: what each test pins and why the two are
separate.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none. The one decision worth recording is splitting
row-denial and field-redaction into separate tests, and the reasoning is under
AC2.
