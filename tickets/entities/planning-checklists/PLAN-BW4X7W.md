---
id: PLAN-BW4X7W
type: planning-checklist
title: 'Planning: ACL: world-shaped read grants, state-shaped write grants (Step 3)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

Full planning survey: `.ignored/dn37j2-plan.md` (survey, decisions, PR
decomposition). Architect decisions recorded there in §8 — binding.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — three PRs:

1. **PR-A** grant syntax + all load validation + the membership refusal +
   world-name charset + client-ceiling worlds axis + the field-redaction
   permission constant. No new capability; parsing, validation, refusal only.
2. **PR-B** `aclaudit` cross-file (policy × metamodel) checks. Advisory only.
3. **PR-C** request-level world selection (`?world=`, `--world`) and the
   read-path grant check.

OUT, deliberately:

- **Closing RR-FOD7IB's sync-path redaction gap** (Q7 option (c)). §8.4 of
  the design doc is "flagged, not solved"; Step 3 must not silently become
  the ticket that solves it. Inherited gap gets a comment + follow-up ticket.
- **Per-tool MCP `world` parameter** (Q3). MCP read gating is itself still
  open (TKT-4QSZ8Y, a shipping gate); a world param on an ungated surface is
  the partial-feature shape Q10 rejected. MCP stays wiring-bound.
- **Per-world search indexing** — Step 5 (TKT-9KZGJO). Under a non-default
  world search REFUSES rather than degrading.
- **Copy kernel / promote** — Step 4 (TKT-C1XUA8).
- **UI/UX for world selection** — Ruling 7: discussed with Jeroen directly,
  never decided by agents.

**Acceptance Criteria:**

1. **The membership-gate load refusal ships with the syntax** (hard AC from
   TKT-T31NKT). `Policy.Validate` refuses a policy granting read on a
   non-default world while EITHER `MembershipSelfPromotionOpen()` OR the new
   `UngatedPrivilegedRoleRelationOpen()` is true. Test: both arms produce a
   load error naming the fix; a worldless policy with an open membership
   relation still boots warn-only (backward compat).
2. **Existing `acl.yaml` files keep their exact meaning.** Test: a policy
   with no `world:` token compiles to zero world grants and produces
   byte-identical read/write verdicts.
3. **A `world:` token never reaches the type-matching machinery.** Test:
   `roleGrantsRead`, `grantForRole`, `filterTypes` and `aclaudit`'s
   `verbLists` never observe a `world:`-prefixed string.
4. **State-shaped write grants load.** Test: `update: ["policy@draft"]` with
   `read: ["policy"]` loads clean (the F2 fix); `update: ["page@draft"]` does
   NOT grant `page@published` or the default state (no inheritance, fail
   closed).
5. **The grant check precedes resolver construction.** Test: a principal
   without the world grant gets an EMPTY result, not a 403, and the resolver
   is never constructed. Structural guard preferred over behavioural alone.
6. **List and GET do not diverge under a world** (design §8.1). Test: an
   ACL-gated principal (GraphQuery pushdown branch, not AllowAll) gets the
   same world-scoped answer from `ListEntities` and from a single-entity GET.
7. **Search refuses under a non-default world** rather than serving
   default-world hits.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: design of record already
      exists — `.ignored/pointer-design.md` §8, plus five prior review passes.
      A research doc would restate it.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — `.ignored/pointer-design.md` §8 is the design of
record; `.ignored/dn37j2-plan.md` is the implementation survey against the
current tree.

**Existing Solutions:**

Libraries: none applicable — this is policy semantics over an in-tree ACL.

Patterns reused (all verified against the tree):

- **Fail-closed + global override permission**: TKT-73C6B2's four-part shape
  — constant in `internal/acl` (`policy.go:58`), registered in
  `BuiltinPermissions()` (`policy.go:71-73`), fail-closed enforced
  structurally in `affordances` via a ctx marker + forced `optIn`
  (`resolver.go:392-394`), override consumed at the *handler* boundary
  selecting a wholly-bypassing path (`history_handler.go:196-227`). Note the
  comment there: merely unsetting the marker is NOT enough.
- **Shared predicate between linter and boot check**: TKT-T31NKT's
  `MembershipSelfPromotionOpen` (`policy.go:290-300`) called by both
  `aclaudit` A1 (`tier_a.go:56`) and the startup warning
  (`appbuild.go:966`), so the two can never disagree. Extended here with
  `UngatedPrivilegedRoleRelationOpen`.
- **Consumer-side narrow view across an arch-lint boundary**:
  `aclaudit.MetamodelReader` (`aclaudit.go:112-136`) with the concrete
  adapter in `internal/cli/acl.go:144`. Gains `HasWorld`/`HasPointer`.
- **Package-scan structural guard**: `acl/ceilingguard_test.go`,
  `worldreader/guard_test.go`, `acl/permguard_test.go` — all fail-closed
  exemption lists.
- **Compile-at-load-into-allowlists**: the client ceiling
  (`ceilingcompile.go`), whose discipline the new worlds axis must follow.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Read grants* — YAML spelling stays `read: [world:published]`, but a
`world:`-prefixed entry is **split out of `RoleDef.Read` into a new
`RoleDef.Worlds` at policy load** (`normalizeWorldGrants`, run at the top of
`Policy.Validate` so it cannot be bypassed — the argument `Validate` already
makes for `normalizeAssertedRoles`). This is forced, not cosmetic: left
inline, a world token is intersected with a client ceiling's TYPE allow/deny
by `filterTypes` (`ceilingcompile.go:277-300`), silently dropped under an
allowlist ceiling and silently kept under a deny ceiling. Splitting also
keeps `roleGrantsRead`, `grantForRole`, `verbLists` and B1 seeing a pure
type list. No `world:*` wildcard (design §4.5's auto-widening argument).

*Write grants* — `type@pointer` stays JOINED in `Create`/`Update`/`Delete`,
because `RoleDef.IsPrivileged` counts `len(r.Update) > 0`; splitting would
make a role holding only `update: ["page@draft"]` register as
non-privileged, **silently disabling A1/A2 and the membership warning for
exactly the policies this feature introduces**. Parsed by a local
`parseStateGrant` reusing `entity.ParsePointer` — NOT `entity.ParseStateRef`,
which validates its left side with `entity.ValidateID` (rejects internal
spaces that `metamodel.ValidateSchemaName` permits in type names; verified
by execution).

*Client ceiling* — `Restriction` gains `worlds:`/`deny_worlds:`, clamped in
`compiledCeiling.clamp`. (Q1 as overruled: the cheap "restricted clients read
the default world only" is fail-closed in form but fail-OPEN in outcome,
because the default world IS the draft face under the doc's own layout.)

*Selection* — grant check BEFORE resolver construction, in a new
`appbuild.WorldSurfaceFor` taking a consumer-side `WorldGate`. It cannot live
in `worldreader`, which is arch-lint- and package-scan-pinned to
`{entity, store}` (guard rule 1). `internal/acl` gains
`Request.PermitsWorld`, resolving roles through `roleFor` (THE clamp point).
A world-bound reader handle is constructed once per request in middleware and
carried in ctx under a typed unexported key — a constructed handle, not a
world value, so §4.4's "no ambient world" holds.

Alternatives rejected: A2-chain reachability search in `Policy.Validate`
(§4.2 — a graph walk with cycles in the most security-sensitive load path,
to close a hole whose necessary condition a ten-line predicate already
catches); explicit world threading through ~39 call sites (§3.2); world as a
plain ctx value (re-resolvable per call, the incoherence §4.4 names).

**Files to modify:**

PR-A: `internal/acl/policy.go` (RoleDef.Worlds, normalizeWorldGrants,
parseStateGrant, grantsVerbOnState, Validate + the F2 write⊆read fix, the
refusal, UngatedPrivilegedRoleRelationOpen, the new Perm constant +
BuiltinPermissions), `internal/acl/readquery.go` (roleGrantsWorldRead),
`internal/acl/request.go` (PermitsWorld), `internal/acl/ceiling.go` +
`ceilingcompile.go` (worlds axis), `internal/aclaudit/tier_a.go` (A2 rewired),
`internal/aclaudit/ceiling.go` (worlds row), `internal/metamodel/loader.go`
(world-name charset), `internal/affordances/resolver.go` (+ the state
fail-closed opt-in), `docs/acl-security.md`.

PR-B: `internal/aclaudit/{aclaudit,tier_b}.go`, `internal/cli/acl.go`
(adapter), `internal/metamodel` (exported pointer/world lookups — none exist
today; `anyTypeDeclaresPointer` is unexported and is "any type").

PR-C: `internal/appbuild/worldsurface.go`, `internal/dataentry/{router,
visiblereader,api_v1,entityreader}.go`, `internal/cli/kong.go` +
`cli_wiring.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| `read: [world:X]` | operator, `acl.yaml` | prefix split; non-empty; no `*` | hard load error |
| `update: ["T@p"]` | operator, `acl.yaml` | `entity.ParsePointer` on the pointer half | hard load error |
| world name | operator, `schema.yaml` | **ALLOWLIST** `[a-z][a-z0-9]*(-[a-z0-9]+)*`, matching the pointer grammar | load refusal |
| `?world=` | **untrusted, per request** | must match a declared world via `Compiled.Lookup` (already fail-closed) | named 400/404 |
| `--world` | operator shell | same lookup | error |

The world-name charset is the one place an allowlist replaces an existing
blocklist. `metamodel.ValidateSchemaName` is deliberately lenient (permits
internal spaces, `/`, `?`, `&`, `%`, `#`) because entity-type names need it;
worlds do not, and worlds now reach URLs, CLI flags, and `acl.yaml` grant
tokens. Zero deployed instances (worlds landed in PR-A of Step 2, fixtures
only), so tightening now costs nothing and later is breaking forever.

**Security-Sensitive Operations:**

- **The load refusal is the ticket's hard AC.** Fails closed; over-refuses
  only into policies already carrying a High audit finding.
- **Grant check strictly precedes resolver construction.** Enforced
  structurally: `worldreader` may not import `acl`/`principal`/`visibility`
  (arch-lint `:781` + `guard_test.go`), so the check cannot drift into it.
  That is guard rule 1 — world resolution is principal-independent, and if
  the gate ran first a denied prime would fall through to the next chain
  candidate, revealing what the ACL denied.
- **`roleFor` is THE clamp point.** `PermitsWorld` must resolve through it,
  never `policy.Roles[...]`; `ceilingguard_test.go` fails otherwise.
- **Error text asymmetry (Q4), deliberate and commented at both sites:**
  unknown world → named 400/404 (a config name is not a secret; CLAUDE.md is
  explicit that a 403 naming a config-declared capability is *more* useful);
  known world without grant → empty 200 (contents and existence are the
  secret; a 403 there is an existence oracle).
- **`internal/dataentry/entityreader.go:18` is deliberately UNGATED** and
  backs relations/serialization. Most likely Step-3 leak site: it must become
  world-bound or be explicitly documented default-world-only.
- **New permission constant MUST be registered** in
  `acl.BuiltinPermissions()` — two guards fire otherwise
  (`permguard_test.go` and `acl audit` A7 telling operators to delete a
  working grant, BUG-NRCJ9E).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 → load-error tests on both refusal arms + the
warn-only backward-compat case, named so the over-refusal reads as
deliberate. AC2 → a golden no-`world:` policy asserting identical verdicts.
AC3 → a scan/unit test that the split leaves `Read` world-free. AC4 → table
test over `grantsVerbOnState` covering the three-way meaning (bare grants the
default state only; `T@p` grants only `p`; `*` grants every type's default
state). AC5 → denial-is-empty behavioural test + a structural guard.
AC6 → the RR-GQWRLD shape: an ACL-gated principal (policy query present, so
the GraphQuery pushdown branch runs) compared against the GET path under the
same world. AC7 → `NewSurface` refusal already exists; add the handler-level
422.

Integration: dataentry handler tests through the real middleware chain, since
the grant check lives in middleware and unit tests of the seams would miss
ordering.

**Edge Cases:**

- `read: ["world:"]`, `["world:*"]`, `["world:default"]` (legal, = default
  world), a type literally named `default` (stays a type — no ambiguity).
- `update: ["page@"]`, `["page@Draft"]` (uppercase), `["page@a--b"]`.
- A type whose name contains a space (`"some property"`) — the
  `ParseStateRef` trap; must parse via `parseStateGrant`.
- `?world=` absent/empty → default world bound explicitly at the boundary
  (§4.4: the interior never sees "unspecified").
- `?world=` naming a declared world the principal cannot read → empty, and
  indistinguishable from a genuinely empty world.
- Ceiling with `worlds:` naming a world no role grants (inert — audit).
- Unicode/percent-encoded `?world=` values after the charset tightening.

**Negative Tests:** every "hard load error" row of the validation table
above; the pushdown/GET divergence case; a searcher present under a
non-default world (must refuse, `ErrSearcherCannotServeWorld`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **The ceiling worlds axis is new security-relevant compilation code.**
   Mitigation: direct unit tests on the compilation step (CLAUDE.md's
   standing rule — compilation is the one part that can fail toward more
   access), plus the existing `ceilingguard_test.go`.
2. **`Validate`'s stated contract changes.** Its godoc says it is a pure
   structural gate that does NOT flag escalation foot-guns (TKT-Z8A62F moved
   those to `aclaudit`). The refusal is a deliberate narrow exception and the
   godoc must say why, or the file contradicts itself.
3. **`rela acl audit` now refuses to load a policy the server rejects**
   (Q6, accepted). Mitigation: the load error carries the full A1 fix text
   and names `rela acl audit`.
4. **`entityReader` ungated path** — risk 1 for an actual leak; explicit AC.
5. **Survey method blind spot** (Ruling 5): this plan is grep-plus-read, not
   AST-parsed. `aclmap` returned zero grant-list hits — the right outcome,
   but evidence by absence. Flagged for `/design-review` and `/code-review`.

**Effort:** l (unchanged) — three PRs, one of which is validation-only.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/acl-security.md` — flip the "This becomes a hard requirement with
      content states" section (`:77-93`) from future to present tense. It
      already promises this refusal to operators **in writing**, so shipping
      without updating it leaves the docs lying in the other direction.
      Also: the new grant syntax, the override permission, the ceiling axis.
- [x] `docs/metamodel.md` — world-name charset.
- [x] `docs/cli-reference.md` — `--world`.
- [ ] `docs/data-entry.md` — deferred: any UI surfacing is Ruling 7 (Jeroen).
- [x] CLAUDE.md — the "bare grants mean the default world / no inheritance
      between states" rule is exactly the kind of invariant that gets
      re-litigated; it belongs there.

**Migration note owed:** an operator who adds `pointers:` to an existing type
finds their existing `update: ["page"]` now covers only the default face.
That is the correct fail-closed reading and it must be documented, not
discovered.

## Design Review

- [x] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** 10 findings, all VERIFIED against the tree before
filing (the reviewer was right on every one I checked; three landed exactly in
the blind spot §9 of the plan predicted).

CRITICAL — RR-WEKTVV, RR-Q1LI2Y, RR-H98VX0, RR-LWE222, RR-SU4T68
SIGNIFICANT — RR-YALMJ1, RR-TFATPO, RR-NFDPOA, RR-4TFZNL, RR-AC7GAH

**BLOCKED on the architect — two findings undercut binding §8 rulings:**

- **RR-WEKTVV vs Q5.** The approved refusal rests on "an ungated privileged
  role-relation is a NECESSARY condition of every chain". It is not, because
  `IsPrivileged` (policy.go:361-363) excludes `Read` by design (RR-LXI3NW).
  This ticket makes a read grant escalation-relevant for the first time, so
  `role_relations: {owns: {confers: viewer}}` ungated + `viewer: {read:
  [world:published]}` is a DIRECT one-write self-promotion into a world grant
  that BOTH proposed arms miss. Needs a third arm (a world-holding role counts
  as privileged for the refusal) — probably a separate `IsWorldPrivileged()`
  rather than widening `IsPrivileged`, which would shift A1/A2/A3 severity
  semantics tree-wide.
- **RR-SU4T68 vs Q2.** The "~3 seams, ~0 handlers" figure Q2 was approved on
  is wrong: `store.GetEntity` takes no query struct so it CANNOT carry a
  world (a `store.Store` interface question with storetest implications), and
  50+ raw read paths plus whole subsystems (views, document render + its
  world-blind cache key, commands, sidebar counts, search, tracer, caldav,
  sync, MCP, Lua) bypass any handle. PR-C's scope needs re-deriving.

Not blocked, foldable into PR-A once the above are settled: RR-Q1LI2Y
(export the canonical predicates, delete the `internal/docs` copy),
RR-H98VX0 (migration destroys grants), RR-LWE222 (normalize at
`NewDeclarative` too), RR-YALMJ1 (B1/B8 in PR-A, not PR-B; fix the §2.3
citation), RR-TFATPO (`deny_worlds` vs the implicit default world),
RR-NFDPOA (spell the guard signature), RR-4TFZNL (`(bool, error)`),
RR-AC7GAH (charset is a control; watch the metamodel→entity arch-lint edge).
