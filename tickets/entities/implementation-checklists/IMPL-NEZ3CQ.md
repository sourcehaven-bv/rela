---
id: IMPL-NEZ3CQ
type: implementation-checklist
title: 'Implementation: MCP analyze_cardinality: delete the fifth copy, call the consolidated analysis service'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — the MCP tests drive the real handler through `setDeps` over a memstore-backed `appbuildtest` service
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed) — this is the ticket's whole point

## Test Quality

- [x] Using fixture builders or factories for test data — `makeTestFixture` / `newTestDeps`, plus nil-rejecting constructors for the two failing readers
- [x] No hardcoded values in assertions when object is in scope — e.g. the count-error test asserts against `countErr.Error()`, not a copied string
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `rela` and ran both surfaces against a scratch project seeded to violate
a bound in BOTH directions (`min_outgoing: 1` on `affects`, `min_incoming: 1`
with `inverse: {id: affected-by}`), with one ticket and one concept and no
relation between them.

`rela analyze cardinality`:

```
⚠ TKT-001 must have at least 1 'affects' relation(s), has 0
⚠ CON-001 must have at least 1 'affected-by' relation(s), has 0
⚠ Found 2 cardinality violations
```

MCP `analyze_cardinality`, driven over a real stdio JSON-RPC session:

```json
[
  {"entity_id":"TKT-001","relation":"affects",
   "message":"must have at least 1 'affects' relation(s), has 0"},
  {"entity_id":"CON-001","relation":"affected-by",
   "message":"must have at least 1 'affected-by' relation(s), has 0"}
]
```

Same two violations, same two sentences, same inverse label on the incoming
side — which is the parity this ticket exists to establish. Before the change
MCP rendered the second one as `incoming 'affects'`.

Also ran `rela analyze cardinality` against this repo's own `tickets/` project
(the dogfooding graph): "All cardinality constraints satisfied", matching the
MCP `analyze_cardinality` result used throughout this ticket's workflow.

Error paths are covered by test rather than by hand, because reproducing a
backend outage manually is less reliable than injecting one: the count-error
and truncated-scan cases each have a test verified to FAIL against the
pre-change implementation.

## Quality

- [x] Code follows project patterns (check similar code) — `CardinalityReader` mirrors `schema.RelationLister`, the existing call-site reader interface in the same package
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds). Two extractions, both earning their keep:
the CHECK itself (the ticket's purpose, 5 copies to 1) and the MESSAGE
rendering (3 copies to 1, via `CardinalityViolation.Message()`, found in
review). Deliberately NOT extracted: `collectCardinalitySubjects` still
duplicates `analysis.collectEntities`, and turning `Constraint` into a typed
enum would change the violation's JSON wire format — both out of scope here.
- [x] No security issues introduced — confirmed by a dedicated rela-security-reviewer pass; the gated reader is preserved end-to-end
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
