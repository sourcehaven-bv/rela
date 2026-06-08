---
id: PLAN-EUTI
type: planning-checklist
title: 'Planning: affordances: migrate resolver to *acl.Declarative; has_role consults ancestor-conferred roles'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In:**

- `internal/affordances/resolver.go`: `New(meta, lookup, *acl.Declarative)` —
drops the policy first arg, reads it via `declarative.Policy()`.
`resolveViaDeclarative` consults `acl.FromContext(ctx)` and reuses the upstream
Request when present, builds a fresh one otherwise.
- `internal/affordances/bindings.go`: `bindingContext.entityRoles`
field carrying the per-entity attribution set the resolver computed.
- `internal/affordances/hostfuncs.go`: `hasRole` consults
`entityRoles` — sees ancestor-conferred roles. `holdsLocalRole` deleted.
- `internal/affordances/effective_roles.go`: deleted.
- Tests: `features_test.go` (UC10/UC11 + new ancestor-conferred test,
AC8 rewritten discriminating), `resolver_test.go` + `hostfuncs_test.go`
call-site updates, new `TestResolver_ReusesRequestFromContext`.
- `internal/dataentry/affordances_policy_test.go`,
`internal/dataentry/affordances_stub.go`: update call site to the new signature;
build a Declarative there.

**Out:**

- `dataentry.attachACLRequest` middleware → PR 4. PR 3 just makes the
resolver *able* to reuse a Request when one is on ctx.
- `appbuild.Collaborators.Declarative` field, `WithACL` auto-detect →
PR 4.

**Acceptance Criteria:**

1. `affordances.New(meta, lookup, *acl.Declarative)` compiles; all
call sites updated. *Test:* tree builds clean.
2. `effective_roles.go` no longer exists in tree. *Test:* `ls` /
`go build` would fail if there were stale callers.
3. `has_role(roleName)` returns true when the role was conferred via
`inherit_roles_through` from an ancestor. *Test:*
`TestFeature_HasRole_AncestorConferred` (belongs-to chain).
4. Resolver reuses `acl.FromContext(ctx)` when present, zero re-walk.
*Test:* `TestResolver_ReusesRequestFromContext` with a counting graph stub;
fatal on premise failure (RR-K7CT).
5. AC8 parity discriminating — fails if resolver and
`Declarative.AuthorizeWrite` disagree. *Test:* rewritten
`TestFeature_AC8_WriteAffordanceParity`.
6. Full tree green; race-clean for affordances + dataentry.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: prior art on reference branch `feat/acl-v1-tkt-svxl` / TKT-SVXL, already vetted)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: this is internal-only resolver glue)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — `.ignored/acl-v1-split-plan.md` PR-3 section is the
authoritative plan; the source branch carries the original review- response
findings (RR-WTLD, RR-JRPZ, RR-Y6Y9, RR-K7CT).

**Existing Solutions:**

- In-codebase prior art: `internal/affordances/effective_roles.go`
(deleted by this PR) implemented a flat role walk. PR 2 generalised that into
`acl.Request.ForEntity` with four-corner Source attribution; PR 3 is the
consumer-side payoff.
- Reusing an upstream Request via ctx is the same pattern documented
in PR 2's `WithRequest`/`FromContext` godoc.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Port the affordance changes from `feat/acl-v1-tkt-svxl`, then patch dataentry
call sites for the new signature. The migration is mechanical:

1. **`affordances.New` signature change** — replace
`New(policy, meta, lookup)` with `New(meta, lookup, declarative)`. Internally
store the `*acl.Declarative`; expose `policy` via `declarative.Policy()` when
needed.
2. **Resolver reuses Request** — `resolveViaDeclarative` calls
`acl.FromContext(ctx)`. If non-nil, reuse it (no fresh `ForPrincipal` call, no
re-walk). If nil, build one with the ctx's principal.
3. **`bindingContext.entityRoles`** — populated from
`request.ForEntity(ctx, entityType, entityID)` once per binding evaluation.
4. **`hasRole`** — looks up the role name in `entityRoles` (where
ancestor-conferred grants land). `holdsLocalRole` deleted because the
`entityRoles` path subsumes it.
5. **Tests rewritten** — AC8 parity (discriminating), UC10/UC11 callsite
updates, new ancestor-conferred test, new request-reuse test.
6. **dataentry call sites** — `affordances_policy_test.go` builds a
Declarative (via `acl.NewDeclarative(&policy, acl.NewStoreGraph(app.store))`)
and passes it. `affordances_stub.go` updated similarly.

**Files to modify:**

- New/changed: `internal/affordances/{resolver,bindings,hostfuncs}.go`
- Deleted: `internal/affordances/effective_roles.go`
- Test updates: `internal/affordances/{features_test,resolver_test,hostfuncs_test}.go`
- dataentry call sites: `internal/dataentry/{affordances_policy_test,affordances_stub}.go`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`acl.FromContext(ctx)`** — if the ctx carries an attached Request,
we trust the resolver bound to it (constructed from the same Declarative we
already hold). Risk: a caller could attach a Request built from a *different*
Declarative. Mitigation: this is an internal-package contract; PR 4's middleware
is the canonical attachment site, and `WithRequest` only accepts `*acl.Request`.
The affordance resolver does not look inside the Request to validate provenance
— same trust model as PR 2.
- **Resolver invocation count** — reusing a Request avoids redundant
graph walks (a perf concern, not security). Building a fresh Request when none
is attached is the safe fallback.

**Security-Sensitive Operations:**

- `has_role` widening is intentional: ancestor-conferred grants are
by-design how `inherit_roles_through` is supposed to work. Without this
widening, the affordance UI would *under-report* what the user can actually do
(write succeeds but the button is hidden), which is a usability bug, not a
security one. The write path still re-authorizes server-side (per
`dataentry/CLAUDE.md`).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Each AC maps to the named test as listed above.

**Edge Cases:**

- ctx without a stamped principal → `acl.NewDeclarative.ForPrincipal`
returns `ErrUnstampedPrincipal`; affordance resolver returns "no visible
affordances" (today's behavior preserved).
- ctx with a Request whose principal differs from `principal.From(ctx)`
→ we trust the attached Request (the upstream attached it deliberately); test
pins this.
- Ancestor chain longer than `acl.DepthCap` → bounded by the cap;
test pins truncation.

**Negative Tests:**

- AC8 parity: write-allowed but resolver-hides → test FAILS (the whole
point of the discriminating rewrite).
- `TestResolver_ReusesRequestFromContext`: graph-stub counts calls;
asserts zero increment when reuse is active.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Signature break** of `affordances.New`. *Mitigation:* internal
package; only dataentry callers + tests. Both updated in this PR.
- **`hasRole` widening could surface latent test fixture assumptions**
that "no ancestor → no role." *Mitigation:* full suite run + race; any failure
is a bug the test was hiding.
- **Request reuse without provenance check** could allow a misbehaving
middleware to inject a Request with the wrong policy. *Mitigation:* same trust
model as PR 2 (`internal/` scoped); PR 4 middleware is the canonical site.

Effort: **m** — ~600 LOC across affordances + dataentry + tests.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md~~ (N/A: no metamodel changes)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI changes)
- [x] ~~docs/data-entry.md~~ (N/A: SPA shape unchanged — UI verdicts
get *more correct* (ancestor-conferred grants now visible) but the response
schema doesn't change)
- [x] ~~CLAUDE.md~~ (N/A: `internal/dataentry/CLAUDE.md` already
documents the affordance contract; the resolver shape change is an
implementation detail)
- [x] ~~README.md~~ (N/A: internal refactor)
- [x] **Internal change** — godoc on `affordances.New` updated to
describe the Declarative parameter

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: reference branch already passed cranky-code-reviewer; RR-WTLD, RR-JRPZ, RR-Y6Y9, RR-K7CT findings already encoded in the ported code per `.ignored/acl-v1-split-plan.md`)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RRs from reference-branch review already
incorporated in the ported code: RR-WTLD (significant), RR-JRPZ (significant),
RR-Y6Y9 (minor), RR-K7CT (minor). PR-3's review checklist audits each.
