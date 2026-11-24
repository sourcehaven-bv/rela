---
id: PLAN-AO6I3WT
type: planning-checklist
title: 'Planning: Scoped relations: within and on_change'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: `within:` and `on_change:` on relation types; load-time checks in
internal/metamodel; enforcement on the entitymanager write path (CreateRelation,
DeleteRelation, ReplaceRelations, PatchWithRelations, CreateLinked, the cascade
host's relation create); data-entry form picker filtering and dependent-field
handling; wire exposure in `/_schema`; docs in docs-project.

Out: entity deletes (not checked, as for relation rules); copy and conversion
edge rewrites (they rewrite edges of an entity being copied or converted and run
their own owning checks; a follow-up ticket covers scope there); a `rela
analyze` report for stale scoped edges in imported data (follow-up).

**Acceptance Criteria:**

1. A schema with a valid `within:` loads; every invalid combination is refused
with a message naming the relation (unknown scope, not single-valued, from not
covered, no owner, two owners, missing/invalid on_change, owning, symmetric,
cycles of length 1, 2, 3, scope relation scoped itself).
2. Creating a scoped edge to a target not owned by the source's scope target
is refused (422), whatever on_change says. Same with an empty scope.
3. Changing the scope edge (replace in_project) clears stale scoped edges with
`clear` in the same transaction, audited; refuses with `reject`.
4. Removing the scope edge clears or refuses the same way.
5. Re-parenting the target (changing its owning edge) clears or refuses
incoming scoped edges.
6. Refusal messages name the relation and its `within`, never an entity id or
title.
7. Cyclic ownership data (an entity owning itself) does not loop.
8. A flows.yaml relation trigger fires once per cleared edge, and a clear does
not re-enter the scope check.
9. Form: the picker offers only targets owned by the selected scope value, is
disabled while it is empty, and a scope change clears the dependent field
(clear) or flags it (reject).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the design was worked out up front by the requester; the
codebase survey below replaces a research entity.

**Existing Solutions:**

- `validateRelationOwning` (internal/metamodel/loader.go) for load-time
relation checks; the new checks sit next to it in within.go.
- `checkRelationRules` / `pendingEdges` (internal/entitymanager/relationrules.go):
in-Tx, check-before-write derivation of after-edges. The scope check follows the
same shape: adds and removes of the write, judged before any write.
- `checkOwningEdgeLeaving` (owning.go) for reading owning edges in the Tx.
- `ownerResolver` (internal/dataentry/owner.go) is the read-side owner; the
form uses the gated relations endpoint of the scope entity instead because
`_owner` is omitted for nested owners.
- No external library applies; this is a schema rule over the store.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Metamodel: `RelationDef.Within`, `RelationDef.OnChange` (`ScopeChange`), and
`ScopeOwner(m, rel, targetType)` returning the one owning relation that owns a
target type from a scope target type.

Write path: a new `scopeIndex` compiled in `entitymanager.New` (scoped relations
by name, by scope relation, by owning relation). A function `checkScopes(ctx,
deps, view, adds, removes) (clears, error)` runs inside the write's Tx, before
any write:

- affected sources: from of every added/removed scoped or scope edge; plus the
sources of incoming scoped edges of every target whose owning edge is added or
removed;
- for each source: after-scope = stored scope edges + adds - removes (at most
one by `max_outgoing: 1`); after-scoped edges likewise;
- for each after-scoped edge, the target's after-owner is read from its
incoming owning edge (stored + adds - removes), one hop;
- an out-of-scope edge added by this write is refused; a stored one is a
clear (on_change: clear) or a refusal (reject).

The clears join the write's removes: they go through the relation-rule check,
are deleted in the same Tx, audited in the Tx, and versioned after commit (the
lineage id is read before the Tx from a non-authoritative pre-pass, as
ReplaceRelations does). One level only means a clear never makes another scoped
edge stale, so there is no second pass.

The path is chosen statically: a write whose edge types include a scoped, scope
or scope-owning relation runs the check and applies its clears in its own Tx
(CreateRelation, deleteRelationIfVersion, ReplaceRelations, PatchWithRelations,
CreateLinked). Each distinct (scoped relation, source type) of a clear is
authorized as the caller with RelationDeleteRequest; a denied clear refuses the
write. Lineage ids come from a pre-pass of the same check; a clear the pre-pass
missed fails the write with a conflict. The cascade host refuses any scope
violation, including one that would need a clear. Ownership is judged by
membership (the scope target is among the target's owners), by id.

Form: `/_schema` relation types carry `within`, `on_change` and `scope_owners`
(target type → owning relation). The picker gets the selected scope value from
the form, loads the scope entity's owning edges once through the gated relations
endpoint, and filters its candidates to those targets. DynamicForm clears or
flags the dependent field on scope change.

**Alternatives rejected:**

- A write-enforced validation rule with `related(...)`: cannot express "same
entity as", and cannot clear.
- Multi-hop owner traversal: unbounded and loops on cyclic data.
- Reading `_owner` in the picker: omitted for nested owners and when two
owners are readable, so the picker would wrongly offer nothing.

**Files to modify:**

internal/metamodel/{types.go,loader.go,within.go}, internal/entitymanager/
{scoped.go,manager.go,cardinality.go,linked.go,joinedreplace.go,
ifmatch_delete.go,cascadehost.go}, internal/apiwire/v1/responses.go,
internal/openapi/schemas.go, the schema response builder in dataentry,
internal/cli/schema_json.go, frontend RelationPicker/FormFieldList/
DynamicForm/types, docs-project GUIDE-metamodel.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Threat model:**

Assets: entity existence and content (secret); ownership edges; the write path
invariants (audit, ACL, transactions). The schema is not secret.

Actors: an authenticated user with partial read access; an operator writing
schema.yaml (trusted).

Threats and mitigations:

1. Existence oracle. A refusal could say which entity owns a hidden target.
Mitigation: the message names the relation and `within` only. The verdict itself
is one bit about a target the caller already names: whether it is owned by the
caller's scope target. The caller must be able to read the target
(requireVisibleTarget runs first), so the bit concerns the owner edge of a
readable entity. This is the accepted relation-rule channel
(docs/acl-security.md), and counting raw edges keeps the verdict the same for
every principal.
2. Clear as an unauthorized delete. A clear deletes an edge the caller did not
name. Mitigation: the cleared edge's source is the same entity whose scope edge
the caller is authorized to write, or (re-parenting) the edge became invalid by
an owning change the caller is authorized to make. The clear is recorded in the
audit log under the caller's principal with reason "cleared out of scope", and
versioned, so it is visible and reversible. An operator who does not want a
write to have this effect declares `reject`.
3. Unbounded work (DoS). Mitigation: one hop, one level, one pass. Per write:
one scope-edge query and one scoped-edge query per affected source, one owner
query per affected target. No traversal, so cyclic ownership data cannot loop.
4. TOCTOU between check and write. Mitigation: the check runs in the write's
Tx on its view (postgres: transaction plus advisory lock; fs/mem: write mutex).
5. Picker as an oracle. The picker reads the scope entity's owning edges
through the ACL-gated relations endpoint; it sees nothing the user could not
already read.

Input validation: `within` must name a declared relation (allowlist of relation
names); `on_change` is an enum allowlist.

**Input Sources & Validation:**

schema.yaml (operator): load-time checks refuse invalid shapes. Relation writes
(user, every transport): checked in the manager's Tx.

**Security-Sensitive Operations:**

Edge deletes on clear: in Tx, audited, versioned, attributed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. internal/metamodel/within_test.go: table of load cases.
2-5. internal/entitymanager/scoped_test.go: create in/out of scope, empty scope,
replace scope (clear/reject), delete scope (clear/reject), re-parent target
(clear/reject), PatchWithRelations joined replace, audit records of clears,
refusal message names no entity.
2. Seeded cyclic ownership (an entity owning itself, two owning each other).
3. Trigger on relation delete fires once per cleared edge.
4. Frontend unit tests for the scoped-relation helper and the picker.

**Edge Cases:**

Empty scope; scope target deleted; target with no owner; target owned by itself;
two scoped relations within the same scope; the scoped edge and the scope change
in one replace (new heading in the new project: accepted).

**Negative Tests:**

Out-of-scope create, reject on change, invalid schema.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

Many write paths: mitigated by routing every check through one function and
testing each entry point. Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- docs-project GUIDE-metamodel (relation options, new "Scoped relations"
section); docs/ is regenerated from it.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review-response entities linked to
TKT-5GK80Q9.
