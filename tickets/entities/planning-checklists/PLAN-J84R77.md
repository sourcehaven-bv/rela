---
id: PLAN-J84R77
type: planning-checklist
title: 'Planning: Predicate language: current_user with is_current_user/has_current_user sugar, pushed into next-action queries'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `current_user` record + `is_current_user`/`has_current_user` host functions
in the predicate language (`internal/predicatefns`); fail-closed identity
binding from a ctx-stamped `QueryIdentity`; IR accessor for request-constant
equalities (`predicate.Program.ConstEqualities`); store pushdown + static-index
derivation for next-action `condition:` (`internal/queryplan`); exposure on ACL
affordance `when:` and next-action `condition:`; docs.

OUT: `where:` surfaces (views/feeds/kanbans/CalDAV) and CLI `--filter` →
[[TKT-ZQV9O5]]. Ordered-comparison pushdown (needs a store operator) → follow-up
per [[RES-05JD73]]. GIN index for list membership. Wizard-form identity.
State-machine/computed/validation/automation profiles deliberately do NOT
declare `current_user` (no request principal, or persisted results).

**Acceptance Criteria:**

1. All three spellings compile and evaluate in the two exposed profiles;
referencing them in a profile without a request is a compile error at load —
`TestCurrentUser_EqualityAndSugar`, `TestCurrentUser_UndeclaredInStdlibProfile`.
2. Absent/empty identity fails closed — `TestCurrentUser_BindFailsClosed`,
`TestMatchesAs_IdentityBoundOnlyWhenNeeded`,
`TestAffordances_CurrentUserSugar_UnknownPlaceholderNeverMatches`; empty
identity pushes nothing — `TestConditionPrefilters` ("an empty identity pushes
NOTHING").
3. Only top-level AND equalities/memberships are pushed — `TestConstEqualities`
(one case per refused shape, including the empty literal),
`TestConditionPrefilters`.
4. Pushdown and index inference agree — `TestConditionPrefilters_AgreesWithIndexProperties`.
5. Two principals, one source, different entities — `TestNextAction_CurrentUserConditionEndToEnd`.
6. Pre-filter actually reaches the store and the Go pass stays authoritative —
`TestNextAction_ConditionPrefiltersReachTheStore`,
`TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach settled in-session against existing research; see below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — [[RES-05JD73]] (pushing date conditions into the store)
already covers the pushdown seam and constant-folding; this ticket takes its
Step-1 shape for equalities only.

**Existing Solutions:**

- No library: the predicate language is rela's own sandboxed Lua-expression
subset (gopher-lua parser). `in` is a reserved Lua keyword, so membership must
be a host function, not an operator.
- `internal/affordances/env.go` already declared a `current_user` RecordType
`{id, tool}` for ACL `when:` — reused as the canonical type
(`predicatefns.CurrentUserType`) rather than introducing a second shape.
- `queryplan.PushdownPrefilters` + `dataentry.executeQuery` (helpers.go) are the
existing belt-and-braces pushdown seam; `ConditionPrefilters` is its
predicate-path twin with the identical contract.
- `store.PropPredicate` non-scalar `PropEqual` already means "any element
equals" for list values (pgstore `equalsCond`, `propmatch.Decide`, storetest
`Props_value_shapes`) — so membership needs no new store operator.
- Prior art on the "@me" idea elsewhere: Jira JQL `currentUser()`, GitHub
`assignee:@me`. Both resolve at query time from the session, never at config
load — same choice here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `predicatefns/currentuser.go`: `CurrentUserType`, `QueryIdentity` + ctx
stamp, `DeclareCurrentUser`/`DeclareCurrentUserFuncs`, `BindCurrentUser` (fail
closed), `CurrentUserFuncs`/`CurrentUserBindings` (empty identity never
matches), `ResolveQueryIdentity` (consumer-side `PrincipalResolver`),
`CurrentUserPrefilterSpec`, `RequiresCurrentUser`.
2. `predicatefns/evaluator.go`: `CompileWithCurrentUser`/`MatchesAs`; profile
in the cache key; identity bound only when required.
3. `predicate/program.go`: `References`, `Functions`; `inspect` refuses an
unhandled node. `predicate/prefilter.go`: `ConstEqualities(PrefilterSpec)` over
the top-level AND spine, recognising listed sugar functions (scalar → equality,
list → membership); empty literal refused.
4. `queryplan`: `ConditionPrefilters`, `ConditionIndexProperties`, shared
`conditionEqualities` core; `StaticIndexSpecs` includes next-action conditions;
`LoadStaticIndexSpecs` runs `conditionlint` so the CLI reconcile refuses what
the server refuses.
5. `conditionlint`: compile in the user profile, evaluate via `MatchesAs`,
expose `Program(type)` and `Types()`; refuse a condition on a free-text query.
6. `nextaction.CandidateFunc` gains the source id; `dataentry` declares
`ConditionPrefilterer` + `NextActionRequestScope` (consumer-side) and threads
prefilters through `executeQueryPrefiltered`; the handler applies the scope
binder once per request. `appbuild.NextActionMatchers` wraps the matcher and
supplies the scope binder (`nextActionRequestScope`, refusing a stamp that
disagrees with the principal); `ErrNoCurrentUser` →
`nextaction.ErrIdentityRequired` → HTTP `next_action_identity_required`.
7. `affordances`: alias the record type, declare the sugar funcs, bind per loop;
`passes` refuses a per-user clause for an unidentified caller.

**Alternatives rejected:**

- `@me` token in `internal/filter`: would give the filter DSL a second parser
concern and no IR to push from; predicate is the converging dialect.
- `current_user` as a bare string: breaks `has_role(current_user, ...)` in
operator acl.yaml; relaxing `checkRelational` without touching `valuesEqual`
would turn a loud compile error into a silent `false`.
- A `PropContains` store operator: unnecessary — non-scalar `PropEqual` already
has membership semantics on every backend.
- Stamping identity in `dataentry`'s router: dataentry may not import
`predicatefns` (arch-lint keeps the predicate engine above the app); the
composition root supplies a scope binder the handler applies once. Converging on
a router-boundary stamp is noted for [[TKT-ZQV9O5]].

**Files to modify:** see the commits on `current-user-predicate`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Operator config (`condition:`, `when:`): compiled at load against a typed Env;
unknown identifiers/functions are compile errors. Not a secret (config rule).
- Request principal: already resolved by `resolvePrincipalEntity`; the scope
binder reads it, never re-derives from headers. `unknown`/empty → no identity.
- Entity data compared against the identity: read through the existing ACL read
gate (`visibleListByTypes`), unchanged.

**Security-Sensitive Operations:**

- Identity binding fails closed (`ErrNoCurrentUser`); an empty identity is never
bound on the query path and never matches on the affordance path; an affordance
grant reading the current user is refused for an unidentified caller.
- Pushdown is narrowing-only (the store may only remove rows the Go pass would
remove); the authoritative Go pass still runs. Pre-filter and Go pass read one
identity stamped once per request, so they cannot select different users' rows.
- `current_user.tool` is never pushed and is documented as not an authorization
input.
- Error response names the misconfiguration (`next_action_identity_required`)
without echoing the identity or any entity data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria above (each names its test).

**Edge Cases:**

- Unset property with `is_current_user` → false, not error (Nil arg).
- Empty list / bare scalar in a list property → membership semantics agree
between `coerceList` and the store's non-scalar equality.
- Same attribute constrained twice → reported once (result is a SUBSET).
- Program compiled with `current_user` declared but not referenced → evaluates
without an identity (`RequiresCurrentUser` false).
- Stamped identity agreeing with the principal is honored; disagreeing is refused.
- Hidden (`visible:`) property: pre-filter keeps the row, Go pass rejects it.

**Negative Tests:**

- `entity.watchers == current_user.id` (list vs string) → compile error.
- `current_user` in a stdlib-only profile → compile error.
- OR / NOT / `~=` / ordered / typed / field-to-field / empty literal → nothing pushed.
- Free-text next-action query with a condition → load error.
- Unidentified request on a per-user source → HTTP 500 with a named code.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Pushdown widening results → mitigated by the never-widen contract, per-shape
refusal tests, and the authoritative Go pass.
- Index inference drifting from pushdown → mitigated by a shared eligibility
core and an agreement test.
- `CandidateFunc` / `NextActionMatcherFunc` signature changes → all
implementations updated; engine tests pass.
- Cache-key collision between profiles → profile is part of the key; tested.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md — "Per-user sources: `current_user`" under next-action
`condition`
- [x] CLAUDE.md — derived static-query index rule now covers conditions
- [x] Field comment on `NextActionSource.Condition`; `store.PropEqual` doc names
the list-membership reading
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-2ITZ84 (significant, addressed), RR-PDKNZG
(significant, addressed), RR-A856MZ (significant, addressed), RR-LD2B23,
RR-IZ9XNH, RR-JMHJJ5, RR-QBW3QO, RR-P1YQPB (minor, addressed), RR-3WKYQX,
RR-KUXDQO (nit, addressed), RR-NDER2B (nit, deferred with reason).
