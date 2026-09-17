---
id: IMPL-XDZC30
type: implementation-checklist
title: 'Implementation: Named query scopes declared per entity type in schema.yaml, referenced by data-entry views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`TestQueryScopes_EndToEnd` is the integration test: schema.yaml → the real
compiler → the appbuild bridge → the dataentry seam → the HTTP list endpoint.
The unit tests stub the evaluator; this one does not, which is what proves the
pieces fit. `TestQueryScopes_IdentityScope` extends it to the identity case that
review showed was entirely unwired.

Errors are surfaced at every layer: a scope that does not compile fails the BOOT
(not the first page that uses it), an undeclared name is refused at config load
AND at request time, and a nil evaluator beside a non-nil scope errors rather
than skipping the filter. Each alternative serves the unscoped set.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The AC6 tests each seed one archived and one live entity and assert both come
back; the scope uses `~=`, which does not lower to a store predicate, so the
archived row can only vanish if a surface deliberately filters in Go. That makes
the assertion about scope application rather than about query shape.

The frontend tests follow `EntityList.world.test.ts`'s anti-vacuity discipline:
each absence assertion ("no query_scope is sent") is paired with a
`rendersProof` guard, because an absence proves nothing against a component that
threw during setup.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verified by | Result |
| --- | --- | --- |
| AC1 declaration and load | `scopes` suite + boot gate in `appbuild.prepare` | PASS |
| AC2 loader validation | `metamodel/queryscope_test.go` (9 reject, 8 accept, collect-all) | PASS |
| AC3 unknown fails closed | config validation + HTTP 400 naming the scope | PASS |
| AC4 default applies | `TestQueryScopes_EndToEnd`, no-param case | PASS |
| AC5 `all` withdraws | `TestQueryScopes_EndToEnd`, `query_scope=all` | PASS |
| AC6 non-SPA unaffected | one test per surface in mcp/lua/cli/analysis/tracer | PASS |
| AC7 Lua/MCP opt-in | SPLIT to TKT-LYLO6P | N/A |
| AC8 pushdown superset | `TestScopedHeaders_ScopePropsAreASuperset` | PASS |
| AC9 visible: overlap warns | `appbuild/queryscopevisibility_test.go` | PASS |
| AC10 no lifecycle concept | the archived story runs end to end on an operator enum | PASS |
| AC11 index derivation | `queryplan/queryscopeindex_test.go` + EXPLAIN on real postgres | PASS |

**AC11's EXPLAIN evidence**, run against PostgreSQL 16 in Docker with 5000
seeded rows. With the derivation:

```
Limit  -> Index Scan using rela_derived_list__57f5db26... on entities e
            Index Cond: ((type = 'taak') AND ((properties ->> 'status') = 'open'))
```

With the scope's contribution removed, the same query degrades to `Seq Scan on
entities` with the condition as a post-scan `Filter`, plus a `Sort` — correct
rows, silently slow, which is exactly the failure AC11 describes. The full
pgstore suite also passes against that database (90s).

**Mutation testing.** Each authoritative check was verified to bite by reverting
it: the Go-side scope filter (leaks a row), AC9's closed-world complement (a
denylist reading reports nothing), AC6's surface assertions (teaching `rela
list` to honor the default fails its test), AC11's derivation (the EXPLAIN plan
degrades), and the SPA attachment (two frontend tests fail).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed rather than invented: `internal/worlds` for the compile-at-
assembly boundary, `conditionlint.NextActionMatchers` + `SetNextActionMatchers`
for the consumer-side seam, `queryplan.ConditionPrefilters` reused as-is for
lowering (no new lowering was written), and `?world=` for the parameter's shape
including its repeated-value defense.

Security: reviewed by both `cranky-code-reviewer` and `rela-security-reviewer`.
Eight findings recorded; the two critical and two significant ones are fixed.
The security reviewer confirmed the scope cannot widen past the ACL, is not an
existence oracle, and that `internal/scopes` reaches `internal/acl` by no route.
