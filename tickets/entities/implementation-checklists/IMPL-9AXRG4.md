---
id: IMPL-9AXRG4
type: implementation-checklist
title: 'Implementation: flatten interpolated values rather than the operator''s template'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Both tests drive the real router through `app.NewRouter().ServeHTTP`, so they
exercise the full pipeline (find-or-create, interpolate, append, patch) rather
than calling `flattenToLine` directly. `flattenToLine` returns no error by
design: a value that cannot be represented on one line is flattened, not
rejected, because refusing would discard an alert whose producer will not resend
it.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Uses the existing `newHookTestApp` / `listTickets` helpers, matching the
neighbouring webhook tests.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Acceptance criteria, each pinned by a test:

1. Operator newlines survive — `TestWebhookRoutes_AppendSectionKeepsOperatorNewlines`
asserts `#### T\n` appears as its own line from a `content:` of `"####
{{body.title}}\n\n- detail: {{body.msg}}"`. PASS.
2. Value newlines still flattened — the same test sends `msg: "one\ntwo"` and
asserts `- detail: one two`, plus a loop over every line asserting none is bare
`two`. PASS.
3. Injection unchanged — `TestWebhookRoutes_AppendSectionFlattensNewlines` is
byte-identical to before and still asserts a payload carrying `"first\n\n##
Injected\n\n<script>alert(1)</script>"` plants no sibling heading. PASS.
4. Content flattened, never dropped — the same test asserts both `Injected` and
`first` are still present in the body. PASS.

Mutation check (the tests are not vacuous): removing the `flattenToLine` call
from `interpolate` and re-running makes
`TestWebhookRoutes_AppendSectionKeepsOperatorNewlines` fail on both the
flattening assertion and the surviving-newline loop, and makes
`TestWebhookRoutes_AppendSectionFlattensNewlines` fail with `## Injected`
rendered as a real heading. Restoring the call makes all three tests in the
`TestWebhookRoutes_AppendSection` group pass again.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The change is a move, not new machinery: `flattenToLine` keeps its body and
gains one caller in `interpolate`, losing the one in `applySteps`. Nothing to
extract.
