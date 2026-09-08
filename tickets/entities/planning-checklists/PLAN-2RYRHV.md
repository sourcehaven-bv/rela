---
id: PLAN-2RYRHV
type: planning-checklist
title: 'Planning: Document the help endpoint as public by design'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: a godoc on `handleEntityHelp` recording that help is public by design, the
`_schema` parallel that supports it, and the condition that would invalidate it.

OUT: any authorization change to `/api/help/`.

OUT: changing `_schema`'s exposure. It is out of scope AND the argument depends
on it staying as it is — which is exactly why the invalidating condition has to
be written down rather than assumed permanent.

**Acceptance Criteria:**

1. A reader at `handleEntityHelp` learns why it is ungated without leaving the
file.
2. The comment names the already-public sibling (`/api/v1/_schema`) so the
reader can check the claim rather than take it on trust.
3. It states what would CHANGE the answer.
4. The `_schema` claim is TRUE — verified against the handler, not assumed.

AC4 is not ceremony. The whole argument rests on "the same data is already
served ungated one endpoint over". If that were wrong, the decision would be
wrong, and a comment asserting it confidently would be actively misleading.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a comment.

**Existing Solutions:**

Verified rather than assumed:

- `handleV1Schema` (`internal/dataentry/api_v1.go:1158`, registered at :84)
serves the complete entity/relation/custom-type model — every type name,
property and relation — with NO gate call in its body.
- It is registered in the SAME router as `/api/help/`
(`internal/dataentry/router.go:87`), so this is not a comparison across
differently-secured surfaces.

CLAUDE.md's rule that a settled decision should read as SETTLED rather than open
is the governing convention here. The current code neither gates nor explains,
and that silence is what left room for the finding.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** extend the godoc on `handleEntityHelp` with the
open-source argument, the `_schema` parallel, and the invalidating condition.

**Alternatives considered:**

- *Gate help on read authorization, as the issue proposes.* Rejected: it would
protect nothing, since the same information is served ungated by `_schema` and —
in an open-source deployment — published in the repository. Worse, a boundary
that looks deliberate while defending nothing invites the reader to assume the
model is private, which is a false sense of protection rather than none.
- *Gate BOTH help and `_schema`.* Out of scope, and a much larger decision: the
schema endpoint is what the SPA builds its forms from, so gating it is a product
change rather than a hardening. Not rejected on merit here — just not this
ticket, and the comment says the answer changes if it ever happens.

**Files to modify:** `internal/dataentry/handlers.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** unchanged — this adds a comment.

**Security-Sensitive Operations:**

The decision itself is the security-relevant artefact. What the endpoint
discloses is the entity TYPE model — type names, field descriptions, transition
prose. What it does NOT disclose is any entity instance, any property VALUE, or
anything ACL-governed; those come from the data endpoints, which are gated.

That distinction is the one to keep in view: the finding conflated
"documentation describing a type" with "data of that type". Only the latter is
what read authorization exists for.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** none. There is no behaviour change, and a test asserting
"help is reachable without authorization" would pin the current design as a
REQUIREMENT — making the decision harder to revisit than the code documenting
it. If `_schema` is ever gated, this decision should be re-opened, and a test
would fight that.

What IS verified, by reading rather than by test: AC4, that `handleV1Schema`
performs no gate check. Recorded in the implementation checklist.

**Edge Cases / Negative Tests:** N/A for a comment.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The `_schema` premise is wrong.* The only real risk — the whole argument
rests on it. Mitigated by checking the handler body for gate calls rather than
trusting the summary.
- *The decision outlives its premise.* If `_schema` is gated later, this
reasoning silently becomes wrong. Mitigated by naming that condition inside the
comment, so it fails loudly.

**Effort:** xs

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] To be settled in the docs-checklist. The godoc is certainly needed; whether
a user-facing note is warranted is a genuine question, since "the help endpoint
is unauthenticated" reads differently to an operator than to a maintainer.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none. The decision worth recording is that the
argument has two legs — open-source publication AND the ungated `_schema` — and
only the second can change. That is why the comment names it.
