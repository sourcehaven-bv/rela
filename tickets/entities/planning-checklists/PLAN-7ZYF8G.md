---
id: PLAN-7ZYF8G
type: planning-checklist
title: 'Planning: Cardinality analysis: batch relation counts and count visible edges only'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: `schema.CheckCardinality` (CLI analyze and validate, MCP) and the
data-entry `_analyze` cardinality section. Out: the write path
(entitymanager), which does not check cardinality counts.

**Acceptance Criteria:**

1. Edges are read with one relation query per relation, never one count
   per subject: a `storetest.Counting` test reads the same number of times
   at 10 and 50 subjects.
2. On a gated path an edge counts only when the reader can read both
   endpoints and its tail face: a principal who can read the subject but
   not a neighbour sees the count without that neighbour (data-entry and
   MCP tests).
3. The CLI keeps raw counts through the same code path, with its raw
   reader acting as the allow-all gate.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach set in the ticket)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal read path)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal read path)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `visibility.PolicyReader.FilterRelations` already
gates a relation batch through `Resolver.EndpointsReadable`, and the gated
readers (`ScriptReader`, MCP's `gatedGraphReader`) route `ListRelations`
through it. `VisibleTracer.FindOrphans` is the gate-before-fold precedent.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `schema.CardinalityReader` swaps `CountRelations`
for `ListRelationsStrict` (`schema.Ungated` adapts a raw store). Per relation with an active bound, one
`ListRelations{Type}` query; edges are counted in memory per (From,
FromFace) for a per-face outgoing bound, per From or To otherwise. The
reader is the gate: a gated reader filters through `EndpointsReadable`, a
raw store or `AllowAllReader` admits every edge. Subjects are listed as
headers. Data-entry drops its own cardinality loop and calls
`schema.CheckCardinality` with its gated reader.

Rejected: a per-subject `EntityIDs` query (one query per direction, but an
IN list the size of the subject set); a separate gate argument (every
gated reader already applies the same gate, so a second one could only
disagree).

**Files to modify:** `internal/schema/cardinality.go`,
`internal/dataentry/analyze.go`, `internal/dataentry/app.go`,
`.go-arch-lint.yml` (dataentry may use schema), tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new input)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** none new; the metamodel bounds and the
store.

**Security-Sensitive Operations:** the count in a violation message. It is
folded from gated edges only (gate before fold), so it cannot reveal a
hidden neighbour.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** 1: schema budget test over `storetest.Counting`. 2:
data-entry `_analyze` and MCP `analyze cardinality` with a principal who
cannot read the neighbour type. 3: existing CLI and schema tests unchanged.

**Edge Cases:** per-face outgoing bound on a faceless type; max bound 0;
an edge whose endpoint is not a subject; scope filter.

**Negative Tests:** a relation-list error fails the check rather than
reporting count 0.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** the tolerant gated relation read hides edges on a failed header
read, which would invent min violations. Mitigation: cardinality reads
through a strict `ListRelationsStrict` that returns the gate fault as an
error. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/acl-security.md if it describes raw
cardinality counts; otherwise godoc only.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: approach and the open CLI question were ruled by the coordinator for Stage 2)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none raised.
