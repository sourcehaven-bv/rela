---
id: PLAN-FHTWBQ
type: planning-checklist
title: 'Planning: related() in views, next-action, CLI filter, validation, automation, state machine and ACL when:'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `related()` evaluates on view `condition:`, next-action `condition:`,
CLI `--filter`, validation `when_condition`/`then_condition`, automation
`on.condition`, state-machine `when:` and ACL `when:` (field, visible, option
and relation grants). Every newly enabled surface validates traversals against
the metamodel at load. Form conditions keep refusing `related()`, with a clearer
message. Derived indexes cover the new condition traversals.

Out of scope, each filed as a follow-up:
- Moving the state-machine `when:` onto `predicatefns.Evaluator`. Its env exposes `entity.value` and `count_relations`, which the Evaluator env lacks, so the move is a breaking change. This ticket binds the traversal into the existing env.
- Page priming of ACL `when:` traversals on caldav, feeds, the tracer decorator and the streaming `PolicyReader`. These stay correct but make one query per row.
- Changing the general rule that an error in a validation `when` skips the rule. Only traversal failures change here.

**Acceptance Criteria:**
1. Each surface answers `related()` with results equal to the query-scope semantics. Tested per surface.
2. Traversals are batched. A list or validation run makes one `MatchingIDs` per distinct traversal, whatever the row count. `storetest.Counting` budget tests compare 10 and 50 rows.
3. A condition without `related()` makes no extra store call. Pinned by a budget test.
4. Principal-facing surfaces (views, next actions, validation under a principal, `_transitions`) do not count entities the principal cannot read.
5. A traversal error never widens access. An ACL grant is denied, a validation rule reports a load error rather than skipping, an automation does not fire, and a transition is blocked.
6. An invalid traversal (unknown relation, wrong type) fails at load on every surface.
7. Form conditions refuse `related()` at load.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the traversal engine and gate exist from TKT-CXQEV0; this wires them into more surfaces)
- [x] Searched for existing libraries that solve this problem (none apply; internal engine)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects (query scopes in TKT-CXQEV0/TKT-XKCNCL are the reference)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `internal/appbuild/queryscopetraversal.go`: `answerTraversals`, `traversalHop`, `traversalAnswers.traversalFunc`. Moved, not rewritten.
- `acl.Request.GateTraversal`, `acl.UngatedTraversal`, `acl.TraversalQuery`.
- `predicatefns.Evaluator.MatchesWithTraversals`, `predicatefns.ValidateTraversals`.
- `store.GraphQueryer.MatchingIDs`, which every backend implements.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

New package `internal/relresolve` (depends on acl, metamodel, predicate,
predicatefns, store):
- `Gate` and `Match` are type aliases for the func types, so consumer-side interfaces match across packages.
- It holds `Ungated`, `Hop`, `Answers` and `Answer`, moved from appbuild.
- `Binder{meta, gate, match}` comes from `NewBinder`, which rejects nil.
- `Bind(ctx, entityType, ids, progs...)` returns `func(rowID) predicate.TraversalFunc`. It dedupes specs across programs. With no specs it makes no call.

Each wiring site builds a Binder with its own gate and match, because match
stamps world and faces per surface. Consumers declare `TraversalBinder` locally.
dataentry and nextaction cannot import predicate, so they pass gate and match
funcs to appbuild adapters, as query scopes already do.

Per surface (details in `.ignored/TKT-205V2N-plan.md`):

| Surface | Gate | Batching | On error |
| --- | --- | --- | --- |
| View condition | Request read gate | One Bind per candidate page, via a new `MatchPage` seam | Propagate (422 when unsupported) |
| Next-action condition | Request read gate, in the source world | One Bind per source and type | Skip the source and warn |
| CLI `--filter` | Ungated (operator trust) | One Bind over the listed ids | Report an invalid filter |
| Validation | Ungated in CLI/system; `DeclarativeGate.GateTraversal` under a principal (MCP, data entry) | One Bind per rule and type, over when+then | LoadError; never a skip or a violation |
| Automation | Ungated (system policy) | List of one | Does not fire; warn |
| State-machine write (`EnforceUpdate`) | Ungated: a hidden blocker still blocks | List of one, over every edge's program | Precondition failure |
| State-machine read (`Performable`) | Raw, like the write side (design review) | One Bind over all out-edges | Blocked |
| ACL `when:` | Raw (the gate cannot gate itself) | `PrimeTraversals` per page on list, view, next-action, export and search; one row otherwise | Deny the grant and warn |

The state-machine traversal reaches through `GraphLookup.BindTraversals`.
`visibility.DeclarativeGate` gains `GateTraversal` to give a principal-bound
gate off the dataentry ctx. The load-time `ValidateTraversals` check is added to
conditionlint (views and next actions), CLI, validation (a new boot check),
automation, transitions and affordances compile. queryplan derives indexes from
the new condition traversals.

Alternatives rejected:
- A single long-lived Resolver. The match must stamp the world and faces per request, and meta reloads.
- Answering one row at a time. That is the per-row lookup the collection-read rule forbids.
- Gating automations by the writer. Stored data would then depend on who wrote it.

**Files to modify:**
- New: `internal/relresolve/*`.
- `.go-arch-lint.yml`.
- `internal/appbuild/{queryscopetraversal,queryscopes,viewconditions,nextaction_matchers,transitions,appbuild}.go`.
- `internal/visibility/adapters.go`.
- `internal/cli/list.go`.
- `internal/conditionlint/{conditionlint,viewcondition,nextaction}.go`.
- `internal/dataentry/{viewcondition,api_v1,export_list,nextaction_handler,affordances,scopedread,app}.go`.
- `internal/nextaction/nextaction.go`.
- `internal/validation/validation.go`, `internal/validator/validator.go`, analysis deps.
- `internal/automation/engine.go`.
- `internal/statemachine/{predicate,enforce,statemachine}.go`.
- `internal/affordances/{resolver,bindings}.go`.
- `internal/queryplan`.
- Guides and CLAUDE.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Conditions are operator config. They are compiled and validated against the metamodel at load, with relation names and types checked by `ValidateTraversals`.
- `--filter` is operator shell input, validated the same way.
- Traversal property filters are string equalities only; `Hop` refuses anything else.

**Security-Sensitive Operations:**
- Principal-facing traversals go through `acl.Request.GateTraversal` (row gate, field gate, client ceiling).
- `ErrTraversalDenied` means no match. `ErrTraversalUnsupported` is an error, so `not related` cannot widen.
- ACL `when:` and the state-machine write side read raw. Each discloses one bit to the requester (a shown or writable field, a 422). The operator authors that policy; it is documented in acl-security.
- Answers are per request and never cached across principals.
- Error messages name the relation path (config), never related entity data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1:
  - `TestListFilter_Related`, `TestViewCondition_Related*`, `TestNextAction_Traversal*`.
  - `TestCheckRule_Traversal*`, `TestProcess_Traversal*`.
  - `TestEnforceUpdate_RelatedPrecondition`, `TestFieldVerdicts_RelatedGrant`.
- AC2:
  - `TestQueryBudget_ViewConditionTraversalIsSizeIndependent`, `TestNextAction_ConditionTraversalBudget`.
  - `TestCheckRule_TraversalBudgetIsRowIndependent`, `TestListFilter_RelatedBudget`.
  - `TestQueryBudget_ACLWhenTraversalIsSizeIndependent`, `TestPerformable_OneBindAcrossEdges`.
- AC3: `TestBinder_NoTraversalIsFree`, `TestProcess_NoTraversalNoStoreCalls`, existing budgets unchanged.
- AC4: `TestViewCondition_HiddenNeighbourDoesNotCount`, `TestNextAction_HiddenNeighbourDoesNotCount`, `TestGatedValidator_TraversalIgnoresHiddenEntity`, `TestTransitionVerdicts_HiddenEntityNotCounted`.
- AC5: `TestFieldVerdicts_TraversalErrorDeniesEvenUnderNot`, `TestCheckRule_TraversalErrorIsLoadErrorNotSkip`, `TestProcess_TraversalErrorDoesNotFire`, `TestTransitionVerdicts_GateErrorBlocks`.
- AC6: `*_InvalidRelatedPath*` / `*RelatedValidated` tests on every surface.
- AC7: `TestLint_FormConditionRefusesRelated`.

**Edge Cases:**
- Empty candidate set: no store call.
- Duplicate ids.
- A spec repeated across programs: one query.
- Chained and incoming hops.
- `not related(...)` under denial and error.
- A historical (versioned) subject in ACL `when:` fails closed.
- A validation rule with no `entity_type` using `related()` is refused at load.

**Negative Tests:** unknown relation, wrong target type, non-string property
constraint, missing binder (load error), gate unsupported (422, skip or deny per
surface).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Per-row query blowup on ACL redaction. Mitigated by page priming at the main list sites; the remaining sites are filed as a follow-up.
- A widening error path. Every surface pins its error to the narrowing outcome with a test.
- A large diff. Done in 11 small commits, starting with a pure-refactor move.
- Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md: `related()` surface table, state machine, validation, automation.
- [x] docs/cli-reference.md: `--filter` supports `related()`.
- [x] docs/data-entry.md: view and next-action conditions.
- [x] CLAUDE.md: `internal/relresolve`, surfaces list.
- [x] docs/acl-security.md: ACL `when:` traversal one-bit disclosure.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-2TWSJP, RR-35CXMY, RR-3P7VS5, RR-44DP60, RR-5EPYZ0, RR-5UBURU, RR-7X06OE, RR-AUQGSV, RR-EE47LA, RR-G3XZ6Z, RR-JNZEET, RR-LMHRWB, RR-M9WSM0, RR-W2UM25, RR-ZIB0M1 (all addressed in the plan; see "Design-review changes" in `.ignored/TKT-205V2N-plan.md`). Changes: `Answers` refuses unasked and id-less rows; state machine reads raw on both sides; raw surfaces refuse a named-face subject; the data-entry validator gate is late-bound; `computed:` refuses `related()`; ACL traversals use a per-operation memo primed per page; automation refuses `related()` on created triggers; indexes only for view and next-action conditions. Follow-ups: TKT-BZBN2O, TKT-7SI6QA, BUG-Q7YO23.
