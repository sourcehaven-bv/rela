---
id: PLAN-433DISX
type: planning-checklist
title: 'Planning: seqtrace: record the same-package functions between drawn calls'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In scope: an `in` field on a step in `steps.json`, set by
`diagram.Build`, and a README paragraph. Out of scope: the Mermaid and text
output, `seqtrace diff`, and raising the archplan exporter pin in the `justfile`
(that follows an archplan release).

**Acceptance Criteria:**

1. A step whose call was made by a same-package helper lists that helper in
`in`: `TestStepIn` default case, `Get` has `helper`.
2. With a root label, the chain starts with the traced root function:
`TestStepIn` root-label case, `Get` has `handle,helper`.
3. A step made by the function drawn above has no `in`: `TestStepIn`, `run`.
4. Comparison ignores `in`: the existing fold and diff tests pass unchanged.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one optional field)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: change inside rela's own tracer)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: change inside rela's own tracer)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: development tool, no product concept)

**Research Doc:** N/A for a small change.

**Existing Solutions:** `Step.Outcome` is an earlier optional field that only
`steps.json` readers use. `in` follows it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `builder.children` and `builder.call` carry the chain of
functions whose calls were lifted into the caller. A drawn step stores the
chain; its own children start with an empty chain. Alternatives: store only the
nearest function (rejected: cannot tell a shared helper below a wrapper from
copied wrappers), or draw same-package calls (rejected: changes every diagram).

**Files to modify:**

- `tools/seqtrace/diagram/steps.go`
- `tools/seqtrace/diagram/diagram_test.go`
- `tools/seqtrace/README.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new input)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: none added)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The trace of a local demo run, as before. `in`
holds function names only, never argument values.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: no input or error path added)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: `Build` is a pure function; the demo run exercises it end to end)

**Test Scenarios:** Table-driven `TestStepIn`, one case without and one with a
root label.

**Edge Cases:** A relabelled root; a helper below a helper; a step with no
helper in between.

**Negative Tests:** None.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** `steps.json` grows (1.2 MB to 2.1 MB on the route suite); it is a
gitignored build output. A reader that rejects unknown fields would break; the
archplan exporter ignores them. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: development tool; `tools/seqtrace/README.md` is updated)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, not an enhancement)

**Documentation Impact:** `tools/seqtrace/README.md` describes `in`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one optional output field; the user approved fixing it on both sides)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** None.
