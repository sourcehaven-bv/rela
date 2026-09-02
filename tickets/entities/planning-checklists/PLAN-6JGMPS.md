---
id: PLAN-6JGMPS
type: planning-checklist
title: 'Planning: Scope the timing claim in the ACL guide to entity-level filtering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: correct the timing claim in the ACL guide so it says what it actually
covers, and record why property-level redaction runs in Go.

OUT: moving redaction into the store. OUT: constant-time redaction — named as
REJECTED rather than left unconsidered, so a later reader knows it was weighed.

**Acceptance Criteria:**

1. The corrected passage distinguishes entity-level filtering (pushed into the
query, no observable cost) from property-level redaction (runs in Go, not
constant-time).
2. It does not overclaim in either direction. "Not constant-time" is accurate;
"vulnerable to timing attacks" would be alarmist for a signal of this size, and
leaving the original text would be an overclaim the other way.
3. It explains why the trade is right, not merely that it was made.
4. The claims about the code are TRUE — specifically that entity access has a
central enforcement point and that field values reach a caller only through the
redacting path.

AC4 matters because AC3's argument depends on it. "This is a strength, not a
shortcoming" is only honest if the central enforcement point actually exists.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a documentation correction.

**Existing Solutions:**

Verified rather than assumed:

- `internal/dataentry/visiblereader.go` exists specifically so gating is
STRUCTURAL rather than by convention. Its own doc says so: it holds the store
privately and exposes only gated reads, because "gate by convention" was the
read-ACL bug class (TKT-N26KLB, #1010).
- The one remaining raw `a.store.GetEntity` on the HTTP path
(`internal/dataentry/api_v1.go:2326`) reads an entity only to compare its TYPE
for a document-kind mismatch, returns no properties to the caller, and is
followed immediately by an ACL gate.

So field values reach a caller only through the redacting path. That is the
premise the whole "strength, not shortcoming" framing rests on, which is why it
was checked rather than asserted.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** amend the existing bullet in the ACL guide's search
section. Keep the original claim (it is correct about ROWS) and add the
distinction, the trade, and why the residual signal is a consequence of doing
field-level redaction at all.

**Alternatives considered:**

- *Push property redaction into the store.* Rejected: `visible:` grants are
per-principal and can be conditional on graph predicates (`has_relation`,
`count_relations`). Expressing that as a query means reimplementing the
affordance resolver in SQL for pgstore and again for fsstore and memstore — a
permanent, backend-multiplied correctness burden on the most security-sensitive
code in the tree, bought against microseconds.
- *Constant-time redaction (pad the loop).* Rejected: real permanent complexity
in the ACL path against a signal requiring an attacker who already holds a valid
principal, already knows which field to probe, and can distinguish microseconds
across a network.
- *Delete the sentence.* Rejected: the claim is TRUE about rows and worth
keeping. Deleting it would lose a real property to avoid stating a limit.

**Files to modify:** `docs-project/entities/guides/GUIDE-acl-security.md`
(source; `docs/` regenerates).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** unchanged — documentation only.

**Security-Sensitive Operations:**

The security-relevant act is stating a limit accurately. An overclaiming
security doc is worse than a silent one: a reader who trusts "no timing signal"
may build on that assumption, and the correction is cheap now and expensive
after someone has.

The corrected text must also not UNDERclaim. Describing this as a vulnerability
would misdirect effort toward a microsecond-scale signal and away from real
work, and would misrepresent a system that does more field-level enforcement
than most.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** none — no behaviour changes.

A timing test was considered and rejected: it would be inherently flaky
(microsecond differences on a shared CI runner) and would pin the CURRENT
performance characteristics as a requirement, so any optimisation to the
redaction loop would redden it. Measuring the signal is not the same as
defending against it, and the decision here is that it does not need defending.

What IS verified: AC4's claims about the code, by reading the call sites.
Recorded in the implementation checklist.

**Edge Cases / Negative Tests:** N/A for a documentation change. `just lint-md`
and the Docs CI check (generated-vs-source consistency) are the gates that
apply.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The correction reads as a concession.* The main risk. A security doc that
says "we are not constant-time" without context invites the next reviewer to
file the same finding, or worse, to treat a deliberate trade as a known
weakness. Mitigated by stating the trade and its reasoning inline.
- *The "strength, not shortcoming" framing is unearned.* Mitigated by verifying
the central-enforcement claim rather than asserting it.

**Effort:** xs

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/acl-security.md` — this IS the change. Edited at the
`docs-project/` source and regenerated with `just docs`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none. The judgement worth recording is that the
correction should read as a scoping of a true claim, not as an admission — the
original sentence is right about rows and stays.
