---
id: PLAN-OJSS
type: planning-checklist
title: 'Planning: acl: Subject + Source + Request + resolver (declarative role-based authz)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In:**

- `internal/acl` package internals: `Subject` sealed sum, `Source`/
attribution, `Request`/`Globals`/`ForEntity`, store-backed `Graph`, declarative
role resolver (`belongs-to` + `inherits-roles-through`), single
`NewDeclarative(p, g)` constructor, `Policy.Validate()`, exported `DepthCap`,
`ReadQuery` scaffold composing `store.GraphQuery`, `WithRequest`/`FromContext`
ctx helpers.
- `internal/entitymanager` Subject wiring: every write path constructs
`Subject`; nil panics (no silent fallback); audit log records
`Subject.ID`/`Subject.FromID`.
- Unit + integration tests for all of the above.

**Out:**

- `appbuild` wiring (`WithACL` auto-detect, `Collaborators.Declarative`)
→ PR 4.
- `dataentry.attachACLRequest` middleware → PR 4 (only the ctx helpers
in `acl` land here).
- Affordance resolver migration to `*acl.Declarative` → PR 3.
- pgstore SQL-native GraphQuery → already shipped in PR 1 (TKT-ZYH3).

**Acceptance Criteria:**

1. `acl.Subject` sealed via unexported method; `EntitySubject` and
`RelationSubject` constructors exist, both implement the interface. *Test:*
`TestSubjectSum` constructs both, asserts they satisfy the sealed contract.
2. `Source` + `RoleAttribution`: deterministic `PrimarySource` selection
sorts by `(Kind, EntityID, RelationType)`. *Test:*
`TestPrimarySource_DeterministicTieBreak` shuffles inputs 50×, asserts same
primary.
3. `Request.ForEntity(id)` caches per-id within a request lifetime.
*Test:* `TestRequest_ForEntity_CachesPerID` records resolver hits.
4. `StoreGraph` adapter built on `store.GetRelation`; non-NotFound
errors surface (RR-K3OO). *Test:*
`TestStoreGraph_HasEdge_SurfacesUnexpectedError` with an injected store stub
returning a sentinel error.
5. `NewDeclarative(p, g) (*Declarative, error)` — single constructor;
`(*Declarative).Policy()` accessor; godoc declares Policy() return immutable
(RR-9GN3). *Test:* `TestNewDeclarative_RequiresGraph`,
`TestNewDeclarative_NilPolicyOK`.
6. `Policy.Validate()` rejects blank role / type / relation names
(RR-NIGK). *Test:* table-driven `TestPolicy_Validate_RejectsBlanks` covering
`""`, whitespace-only names.
7. Role expansion sorts iteration over `RoleRelations` (RR-MBK0).
*Test:* `TestResolver_RolesDeterministic` runs 50×, asserts identical ordering.
8. `acl.DepthCap` exported and equal to `graphquerynaive.DepthCap`
(RR-AROE acl side). *Test:* `TestDepthCap_LockstepWithGraphquerynaive` asserts
equality at compile time; CI catches drift.
9. `entitymanager` populates `Subject` on every write; `Subject == nil`
panics in `AuthorizeWrite` (RR-X1TE). *Tests:*
`TestAuthorizeWrite_NilSubject_Panics`,
`TestAuthorizeWrite_UnstampedPrincipal_Denies`.
10. Audit log records `Subject.ID` / `Subject.FromID` (RR-79HD).
*Test:* `TestEntityManager_DeniedWrite_RecordsSubjectAttribution`.
11. `Request_ForEntity_AttributionsDeterministic` 50-iteration test
pins ordering of `Decision.Attributions`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: prior art already on reference branch `feat/acl-v1-tkt-svxl` / TKT-SVXL)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: declarative role-based ACL is well-trodden; in-repo prior art is authoritative)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — reference branch `feat/acl-v1-tkt-svxl` is the research
artefact. TKT-SVXL is its planning ticket; PLAN-* / IMPL-* / REV-* on that
branch carry the design intent and 22 review-responses that this PR's code
already incorporates.

**Existing Solutions:**

- Casbin / OPA were considered on the reference branch and rejected:
external policy engines couple data-entry's per-entity verdicts to an
out-of-tree DSL evaluator; rela's needs are bounded enough (role grants +
transitive belongs-to/inherits-roles-through) that a declarative in-tree
resolver wins on legibility and test simplicity.
- In-codebase prior art: `internal/affordances` (current
`effective_roles.go`) walks roles per-request; this PR generalises that pattern
into `acl.Request.ForEntity`.
- Store integration: PR 1's `store.GraphQuery` provides the read-side
scaffolding for `ReadQuery` here. Inheritance traversal uses the same
`InheritThrough` / `EntityInheritThrough` shape.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Port code from `feat/acl-v1-tkt-svxl` with surgical adjustments for the PR slice
(affordance + appbuild references stripped). Concretely:

1. **`acl.Subject`** — sealed sum interface with `subjectSealed()`
unexported method. Two impls: `EntitySubject{ID, Type}` and
`RelationSubject{Type, FromID, ToID}`. No nil fallback anywhere downstream
(RR-X1TE).
2. **`acl.Source`** — `SourceKind` enum
(`SourceUnknown`/`SourceDirect`/`SourceInherited`/`SourceGroup`),
`RoleAttribution` carries source + role name. `lessSource` orders tuples for
`PrimarySource` deterministic tie-break (RR-MBK0).
3. **`acl.Graph`** — interface (`HasEdge`, `Out`, `In`) + `NullGraph`
for tests + `StoreGraph` adapter that calls `store.GetRelation`.
Non-`store.ErrNotFound` errors surface (RR-K3OO).
4. **`acl.Request`** — per-call ctx-scoped struct: `Globals` computed
once on construction (policy-globals: roles that everyone has); `ForEntity(id)`
lazily resolves and caches per-id within the request (RR-F9M9 keeps it on
entity-only). `WithRequest`/`FromContext` helpers thread it through the call
stack.
5. **`acl.Resolver`** — walks `member-of` ancestors (depth-bounded by
`DepthCap`); for each ancestor, looks up role grants and inherited roles
(`inherits-roles-through`). Sorted iteration over `RoleRelations` (RR-MBK0).
6. **`acl.Declarative`** — single `NewDeclarative(p, g) (*Declarative, error)`.
`Policy()` accessor returns the immutable policy (godoc warns callers not to
mutate, RR-9GN3). `AuthorizeWrite` constructs/uses a `Request`, dispatches on
`Subject` type. No nil-Subject fallback (RR-X1TE).
7. **`acl.ReadQuery`** — composes `store.GraphQuery` for read-side
enforcement scaffolding. No consumers in this PR (affordance migrates in PR 3);
covered by unit test confirming the composition.
8. **`acl.DepthCap`** — exported constant; init-time check (or
compile-time assertion) keeps it in lockstep with `graphquerynaive.DepthCap`
(RR-AROE).
9. **`entitymanager`** — every write path constructs a `Subject` from
the entity/relation being written, passes it into `AuthorizeWrite`. Audit log
writes `Subject.ID` for entities, `Subject.FromID` for relations (RR-79HD).
`RelationSubject.To*` not stored if unused (RR-F9M9).

**Files to modify:**

- New: `internal/acl/{subject,source,source_test,graph,storegraph,
storegraph_test,request,request_test,resolver,resolver_test,
authz_write,authz_write_test,internals,readquery,readquery_test,
features_test,doc_test,testutil_test}.go`
- Modified: `internal/acl/{acl,acl_test,declarative,declarative_test,
policy,policy_test}.go` — surface `Subject` and updated constructor.
- Modified: `internal/entitymanager/manager.go` — populate `Subject` on
every write; remove silent nil fallback; audit attribution.
- New: `internal/entitymanager/acl_test.go` — denied-write attribution
coverage.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Policy YAML** (`acl.yaml`) — operator-controlled, parsed at boot.
`Policy.Validate()` rejects blank role/type/relation names (RR-NIGK) and is the
boot-time gate. Malformed policy: PR 4 will fail boot loudly (RR-72OJ); this PR
adds the Validate primitive.
- **Subject** — constructed in-process from the requesting principal +
the entity being written. Never accepted from over-the-wire input. Nil Subject
is treated as a programmer error: `AuthorizeWrite` panics (RR-X1TE) so unsafe
call sites surface in tests rather than silently denying or silently allowing.
- **Principal identity** — `Subject.ID` traces back to the stamped
principal (per `internal/principal`). An unstamped (anonymous) write results in
`AuthorizeWrite` denying with a clear error message — does not panic, does not
silently allow.

**Security-Sensitive Operations:**

- `AuthorizeWrite` is the gate. Failure paths must (a) deny by default,
(b) record attribution in the audit log so a denied write is traceable to the
principal that attempted it (RR-79HD), (c) not leak which policy clauses
matched/missed.
- `StoreGraph.HasEdge` errors: `store.ErrNotFound` → `false, nil`
(expected); any other error → surfaces (RR-K3OO) so the caller doesn't silently
treat a transient store failure as "no role".

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

See Acceptance Criteria 1–11 above; each carries its specific test name.

**Edge Cases:**

- Empty `RoleRelations` (no grants) → empty `Attributions`, write
denied unless policy globals grant.
- Cyclic `member-of` (A → B → A) → `DepthCap` bounds recursion; test
asserts no infinite loop and deterministic result.
- Whitespace-only role name in policy → `Validate()` rejects (RR-ZB1V
whitespace test).
- Concurrent `Request` use: `Request` is per-call (ctx-scoped); test
documents that callers must not share across goroutines.
- `Subject == nil` at `AuthorizeWrite` → panic with clear message
(RR-X1TE).

**Negative Tests:**

- `TestPolicy_Validate_RejectsBlanks` — blank role name returns error.
- `TestAuthorizeWrite_NilSubject_Panics` — recovers, asserts panic
message naming the issue.
- `TestAuthorizeWrite_UnstampedPrincipal_Denies` — anonymous principal
is denied (not panic).
- `TestStoreGraph_HasEdge_SurfacesUnexpectedError` — store returns
sentinel; `HasEdge` returns the error wrapped.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Public API change**: `WriteRequest` shape changes (drops legacy
`EntityType`/`RelationType`, adds `Subject`). *Mitigation:* the acl package is
`internal/`; only entitymanager + (later) appbuild consume it. PR 3 + PR 4
update call sites; this PR's scope keeps the change bounded to entitymanager.
- **Subject==nil panic** could mask programmer errors as crashes in
production. *Mitigation:* every write call site has a test populating Subject;
CI coverage on entitymanager ensures no silent path skips Subject construction.
- **Lockstep DepthCap**: drift between `acl.DepthCap` and
`graphquerynaive.DepthCap` would silently change traversal depth. *Mitigation:*
RR-AROE test compares constants; CI fails on drift.

Effort: **l** (large) — ~1200 LOC across acl + entitymanager + tests.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md~~ (N/A: no metamodel changes; acl.yaml schema
lands in PR 4 with the boot integration)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI commands change)
- [x] ~~docs/data-entry.md~~ (N/A: UI consumption lands in PR 3 + PR 4)
- [x] ~~CLAUDE.md~~ (N/A: write-path rules already reference
entitymanager; Subject is an implementation detail at this layer)
- [x] ~~README.md~~ (N/A: internal refactor)
- [x] **Internal change, no user-facing docs needed** — godoc on
`acl.Subject`, `acl.NewDeclarative`, `acl.Request` is the public surface

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: reference branch already passed cranky-code-reviewer; 22 RR entities on TKT-SVXL document the audit. This PR ports already-reviewed code.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RRs from reference-branch review already
incorporated in the ported code per `.ignored/acl-v1-split-plan.md`: RR-X1TE,
RR-NIGK, RR-MBK0, RR-L3VO, RR-F9M9, RR-3D6Q, RR-2XZW, RR-K3OO, RR-ZB1V, RR-79HD,
RR-AROE (acl part), RR-JJYW (WithRequest part), RR-9GN3. PR 2's review-checklist
will audit these per the plan.
