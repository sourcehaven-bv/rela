---
id: IMPL-CY777P
type: implementation-checklist
title: 'Implementation: Restore the AC1.7 test: ACL deny returns a structured 403 body'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — this ticket IS a test; there is no
production change beyond one stale comment.
- [x] Integration tests written — `TestHandler_ACLDeny_Returns403Structured`
drives the real HTTP handler against a real `entitymanager` wired with a real
`acl.Declarative`, not a stubbed error. The denial comes from `AuthorizeWrite`
on the actual write path.
- [x] Happy path implemented
- [x] Edge cases from planning handled — the role grants **read** but not
**update**, which is the only configuration that reaches the code under test. A
role with no read grant 404s at the visibility gate (AC3) and never exercises
AuthorizeWrite.
- [x] ~~Error handling in place~~ (N/A: test-only)

## Test Quality

- [x] Using fixture builders or factories — `appbuildtest.New` with
`WithFS`/`WithStore`/`WithDeclarative`, plus the package's existing
`mustNewACL`, `patchEntityAs` and `newAppFromParts` helpers.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — the metamodel declares one type with
one property; the policy has one role and one assignment.
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object — the body is **decoded** and
compared per key rather than substring-matched, so a response that merely
*contains* the word "forbidden" still fails.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*The gap was established by mutation before writing anything*, because "there is
no test" is a claim worth proving rather than inferring from a missing file:

| Mutation to `writeForbiddenIfACLDenied` | Whole `internal/dataentry` suite |
|---|---|
| Never fire (fall through to generic 500) | **FAILS** — the 403 *status* was covered incidentally |
| `"error": "forbidden"` → `"error": "MUTATED"` | **PASSES** — the structured *body* was covered by nothing |

So the contract was half-covered, and the uncovered half is exactly what AC1.7
is about. That distinction matters: reporting "AC1.7 is untested" would have
been wrong, and reporting "it's fine, other tests cover 403" would have been
worse.

*After the new test*, both mutations are caught:

```
body["error"]: want "forbidden", got "MUTATED"
body["rule_kind"] is empty; AC1.7 requires it so an operator can tell which rule fired
```

**A stale comment was blocking this.** `acl_write_test.go` carried a note saying
a dataentry-level visible-but-write-denied test *"would require wiring the test
entitymanager with the same ACL, which `newTestAppV1` does not do"*. That was
true when written and is not now: `appbuildtest.WithDeclarative` wires the
Declarative as both the write-authz ACL and the affordance resolver. The comment
is updated to point at the new test and to state what the hidden-target tests
actually pin (the gate *ordering*), so the next person doesn't re-derive the
same dead end.

**Gates:** `go test ./...` exit 0; `just lint` 0 issues; comment-lint,
arch-lint, plimsoll all clean.

## Quality

- [x] Code follows project patterns — placed in its own
`acl_deny_403_test.go` alongside the sixteen other `acl_*_test.go` files, and
reuses their helpers rather than introducing a parallel fixture.
- [x] Checked for DRY opportunities — deliberately did **not** extend an
existing test. AC1.7 is a named contract from a security review; a dedicated
test with the AC in its doc comment is findable by someone auditing the control,
which an extra assertion inside an unrelated test would not be.
- [x] No security issues introduced — this restores a security-test control
(CONTROL-8-29), it does not change behaviour.
- [x] No silent failures — the point of the ticket.
- [x] No debug code left behind.
