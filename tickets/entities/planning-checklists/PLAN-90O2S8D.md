---
id: PLAN-90O2S8D
type: planning-checklist
title: 'Planning: Narrow the list_entities Go path for filters that do not lower'
started: "2026-11-21"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

MCP `list_entities` falls back to `listPage.filterInGo` (whole-type read, Go
evaluation) in three cases: (a) the wiring has no `ConditionLowerer`, (b)
`queryplan.LowerCondition` declines the filter, (c) the page reader returns
`visibility.ErrPropsNotPushable` (a filtered property hidden on some rows, or a
traversal the gate refuses rather than denies). The goal is that the read always
costs a page, keeps the exact store-counted `total`, and leaks no hidden value.

**Scope:**

In:
- Remove `listPage.filterInGo` and `filterEntities`; remove the
`internal/mcp/tools_entity.go#listPage.filterInGo` allowlist entry.
- (b) becomes a tool error that names the unsupported construct.
- (c, hidden field) answered in the store: a leaf on a property the caller
does not see on every row is rewritten so it reads the stored value only where
the caller's `visible:` grants show it, and the absent-value answer elsewhere.
This is exactly the Go verdict on the redacted row.
- (c, refused traversal, unexpressible visibility) and (a) become tool errors.
- Tool description and `docs-project` MCP guide list the supported shapes.

Out:
- Date, datetime, decimal and boolean comparisons in the store (TKT-4Z2BIRG
canonical dates). They are refused for now.
- Folding `has_role` / `has_global_role`: they do not compile in the MCP
filter environment (`Evaluator.Compile`/`CompileWithCurrentUser` declare no such
function), so there is nothing to fold. `current_user.tool` is refused with a
named error rather than folded.
- Other Go-path consumers of `LowerCondition` (lists, nav rules) keep theirs.

**Acceptance Criteria:**
1. A lowerable filter on fully visible properties: unchanged result, one page.
2. A filter that does not lower (`entity.due < today()`, a date ordered
comparison, `entity.a == entity.b`, `current_user.tool == 'x'`, a rollup)
returns an MCP error that names the construct; no whole-type read.
3. A filter on a property hidden on some rows returns exactly the rows the old
Go path returned (equality, `~=`, ordered, `not`, `or`), with an exact total, on
memstore, fs, sqlite and postgres.
4. A traversal the gate refuses returns an error (it did before, from Go).
5. `listPage.filterInGo` and its allowlist entry are gone; archguard passes.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: direction agreed with the user; prior art is in-tree)
- [x] ~~Searched for existing libraries~~ (N/A: internal query lowering)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-tree design)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `search.FieldScope` / `affordances.PolicyResolver.SearchFieldScope`: one
type's `visible:` grants for the request, each a field list plus a
`store.GraphQuery` for the rows it applies to. Pinned to `FieldVerdicts` by
`TestSearchFieldScope_AgreesWithFieldVerdicts`.
- `visibility/rollup.go#visibleWhen` already turns a FieldScope into the row
condition under which a property shows (as `store.RowCondition`).
- `store.Cond` (TKT-8HY05WB) answers and/or/not over prop and relation
leaves on every backend.
- `graphquerynaive.MatchProps` is the reference store semantics of a leaf.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. predicate: `Program.ConditionTreeExplained(spec) (CondTree, string)`; the
string renders the first sub-expression outside the tree language (`today()`,
`entity.a == entity.b`). `ConditionTree` wraps it.
2. queryplan: `LowerConditionExplained(...) (ConditionLowering, error)`
returning `*UnsupportedConditionError{Construct, Reason}` (date ordered
comparison, decimal, rollup, list, current_user.tool, too large).
`LowerCondition` keeps its signature and wraps it.
3. appbuild `QueryConditions.Lower` returns `(visibility.PageQuery, error)`;
mcp `ConditionLowerer.Lower` follows (consumer-side interface).
4. store: `GraphQuery.RowCond() (Cond, error)`: Props as prop leaves,
Related as relation leaves (a negated one as `NotCond` over the un-negated
predicate), Where carried over. Any, Narrowing, HasInbound, HasOutbound, FaceIn
refuse. Pinned in storetest: `Where = q.RowCond()` returns the rows `q` returns,
on every backend (RR from design review). affordances `lowerWhen` normalizes
`FaceIn: [""]` with the metamodel: nil on a faceless type, grant dropped on a
faced type (DEC-NPZICR: no zero-face row), so search meaning is unchanged.
5. visibility: V_p built next to `visibleWhen` from the existing
`FieldScoper` (`SearchFieldScope`) the redactor already implements: OR of
`RowCond()` of each grant listing p (nil When true; ungated p true; gated with
no grant false). A grant FaceIn is compared with the read scope's FaceIn: scope
subset of grant faces -> true, disjoint -> false, else refuse.
`ScriptReader.ListEntityPage` no longer refuses props not visible on every row;
after gating, the largest subtree whose prop leaves all read the same hidden
property p and that has no relation leaf becomes `(V_p and S) or (not V_p and
S0)`, S0 the subtree's value on an absent key, folded to `V_p and S` or `not V_p
or S`. The absent-key value of a leaf comes from a small table over the ops
LowerCondition emits (scalar non-empty equality and membership false,
PropNotEqualOrEmpty true, ordered false); any other leaf refuses. Props on
hidden properties move into Where. The tree is checked with `store.ValidateCond`
(shape only, no probe query); over limits -> `ErrPropsNotPushable` wrapped with
a reason. The sentinel's doc changes: it is now a refusal the caller reports.
6. mcp: delete `filterInGo`/`filterEntities`; Lower error and
ErrPropsNotPushable become tool errors; nil Conditions -> error. A test-only Go
reference (evaluation over the gated, redacted rows) checks the tool output on
hidden-field filters.
7. archguard: remove the allowlist entry. Docs: tool description + guide.

Alternatives rejected:
- Superset pre-filter + Go remainder (ticket option 1): still reads the type.
- Header-only Go read (option 2): still O(type).
- Optional total: the user asked to keep the exact total.
- Refusing hidden-field filters outright: a principal with conditional
visibility could not filter on status at all.
- A face leaf in `store.Cond`: a 4-backend contract change for a rare grant
shape (global blockless role on a subset of faces); refused instead.

**Files to modify:** internal/predicate/condtree.go (+render),
internal/queryplan/conditionlower.go, internal/appbuild/conditionlower.go,
internal/store/graphquery.go, internal/store/storetest,
internal/affordances/searchscope.go, internal/visibility/entitypage.go (+ new
fieldcond.go), internal/mcp/tools_entity.go, server.go, tools.go,
internal/archguard/unpagedread_allowlist_test.go,
docs-project/entities/guides/GUIDE-mcp-server.md, tests in each package.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The filter expression comes from the MCP caller
(untrusted). It is compiled by the predicate engine and lowered by an allowlist
(`LowerCondition`): any shape not explicitly lowered is refused. Tree size is
bounded by `store.MaxCond*`, checked after the visibility rewrite too.

**Security-Sensitive Operations:**
- Field redaction: the rewrite must equal the redacted-row verdict. V_p is
derived from the same FieldScope search uses (pinned to FieldVerdicts). Every
unexpressible shape refuses; none approximates toward "visible".
- Grant relation leaves are ungated, as the redactor's own binder answers
them; the result set equals the Go verdict, so no new channel.
- Row gate unchanged (store read scope). Errors name filter constructs and
config (not secret), never values.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- predicate: explained construct for call, two-attr comparison, arithmetic.
- queryplan: error reasons per decline kind; LowerCondition unchanged.
- visibility: V_p agrees with FieldVerdicts on a fixture with
global conditional, local-role and blockless grants (row-by-row check via a page
read with the Cond as Where).
- visibility: page with hidden-field filter equals Go evaluation over
redacted rows (=, ~=, <, not, or), total exact; over-limit refuses.
- mcp: existing list_entities filter tests; new tests that non-lowering
filters error with the construct named, hidden-field filter returns the
visible-only matches.
- Backends: memstore default, sqlite tag, fs; postgres in CI.

**Edge Cases:**
- `~=` on a hidden field: hidden rows match (nil ~= v is true).
- `== ''` does not lower (prefilter refuses empty-literal equality): error.
- Integer `==` on a hidden property (two leaves, one subtree, one V).
- `~=` on a hidden property where V holds through a negated local-role leaf.
- Two local roles plus the open grant (limit exercise).
- Field ceiling hides the property everywhere (V=false).
- Property gated with no grant (V=false); grant with nil When (V=true).
- Faced type, blockless global role reading a face subset: intersection
with the read scope's faces decides; otherwise error.
- No identity bound -> error (fails closed).

**Negative Tests:** Non-lowering filters, refused traversal, too-large rewritten
tree, nil lowerer wiring: all return an error, never a Go scan.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Divergence between V_p and the redaction verdict -> leak. Mitigation:
derive from SearchFieldScope; agreement test against FieldVerdicts.
- Users lose date filters in MCP list_entities. Accepted direction; error
points to search or a string/integer property; follow-up TKT-4Z2BIRG.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- docs-project GUIDE-mcp-server (filter section), tool description.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** go-architect review: relation-leaf limit (V once per
same-property subtree), GraphQuery->Cond conversion placed in store with a
storetest differential, absent-key table instead of a graphquerynaive
dependency, `== ''` does not lower, face-subset rule against the read scope,
reasoned refusal errors, extra test fixtures. All folded in above; see the RR
entities linked to the ticket.
