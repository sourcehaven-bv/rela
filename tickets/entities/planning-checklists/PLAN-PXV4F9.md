---
id: PLAN-PXV4F9
type: planning-checklist
title: 'Planning: Client attenuation: principal_type baselines + scope re-openings as an ACL ceiling below the acting user'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `principal_type`/`scope` claim extraction through to `principal.Principal`;
`client_baselines` + `scope_grants` policy keys; load-time compilation to plain
allowlists; the clamp after role resolution; enforcement on read gate,
`visible:` redaction, write authorize and named permissions; a distinguishable
`SourceKind`; `rela acl audit` rules; `rela acl map --as`; docs.

OUT: MCP `list_tools` filtering and MCP read gating (both TKT-G3PPD — this
ticket makes the *policy* expressible, it does not wire `internal/mcp` into
`internal/visibility`); org enforcement (separate, named in TKT-RP3X3Q);
`client_id` keying (mechanism supports adding it later at no semantic cost);
unifying verbs and permissions (IDEA-HUWQ); `syncContext` claim-dropping
(BUG-0Q8MCZ).

**Acceptance Criteria:** 13 criteria on TKT-IAC8TX, each mapped to a test
scenario under Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design was settled through direct discussion with
the user (see the "Why this shape" section on TKT-IAC8TX, which records the
rejected alternatives and the reasoning). A `/research` survey of token
attenuation prior art (OAuth downscoping, Macaroons/caveats, Biscuit, GCP
credential attenuation) was offered and declined in favour of proceeding.

**Existing Solutions:**

Codebase prior art, all verified by direct read:

- `asserted_role_assignments` (`internal/acl/policy.go:112`, resolver at
`resolver.go:57-65`) — the precedent for claim → policy mapping. Establishes
that a claim value is never a role name; it only selects an operator-authored
entry, so an IdP cannot name a rela role the deployment did not choose. This
ticket follows the identical discipline for `principal_type`/`scope`.
- Closed-world `visible:` (`internal/affordances/resolver.go:370`,
`compileFieldBlock` at `:256`) — a role declaring a `visible:` block for a type
asserts a *complete* list; unnamed fields redact. This is the existing machinery
the allowlist form reuses, and the source of the fail-closed property. Already
conformance-tested by `internal/visibility/visibilitytest/suite.go`.
- `EveryoneRole` (`internal/acl/policy.go:34`, `resolver.go:63`) — precedent for
a role entering the effective set with no graph walk.
- `store.Formatter` / `HistoryReader` / `VersionWriter` — the optional-capability
type-assertion idiom, if the clamp needs to be optional per backend.
- `PermitsRead`/`ReadQuery` compiling to `store.GraphQuery`
(`internal/acl/readquery.go:16-27`) — the reason a *runtime* denial primitive
was rejected: read gating pushes down into SQL, so a runtime deny would have to
become a SQL predicate. Load-time compilation sidesteps this entirely.

External: the shape is OAuth token downscoping / capability attenuation
(Macaroon caveats, Biscuit, GCP credential attenuation). No library is
applicable — the ceiling must compile against rela's own `RoleDef` vocabulary.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

The key insight that makes this tractable: **compile the ceiling at load time,
not at evaluation time.** A baseline/scope block is static config, so `redact:
{person: [salary]}` is resolvable against the metamodel when the policy loads —
enumerate `person`'s declared fields, subtract `salary`, store the result as an
ordinary allowlist. The runtime evaluator never learns the word "deny";
`decideFromAttrs`, `readQuery`, `grantsPermission` and `FieldVerdicts` keep
seeing plain allowlists. DEC-RG878's additive union semantics are untouched.

Five steps:

1. **Claim extraction.** Add `PrincipalType`/`Scope` to
`jwtauth.AssertionClaims` (`verifier.go:157-163`) and project them in
`VerifyAssertion` (`:186-204`), bounded the way `roles` is (`maxRoles=32`,
`maxRoleRunes=256`, `:171-174`). Thread through `dataentry.AssertedIdentity`
(`router.go:471-476`) → `verifiedPrincipal` (`:554-579`) → `principal.Verified`
(`principal.go:84`). Preserve the unexported-field +
`Verified()`-only-constructor shape so the compiler keeps enforcing the trust
boundary.

2. **Policy types.** `Policy.ClientBaselines map[string]ClientBaseline` and
`Policy.ScopeGrants map[string]ScopeGrant`, plus entries in `knownPolicyKeys`
(`policy.go:413-424` — the reflection parity test `policy_parity_test.go:15-30`
fails CI without them). A shared `restrictionBlock` type carries both spellings
per axis (`Read`/`DenyRead`, `Update`/`DenyUpdate`, `Visible`/`Redact`, …).

3. **Load-time compilation.** In `Validate` / `ValidateAgainstMetamodel`:
reject overlapping `applies_to` sets; reject both spellings for one type in one
block; expand `"*"`; expand `deny_write` to create+update+delete; resolve
denylist forms to allowlists against the metamodel. Output is a compiled
`ceiling` keyed by principal_type, with per-scope deltas.

4. **The clamp.** After role resolution produces effective grants, intersect
with `baseline ∪ matched scope_grants`. Applied at the point where
`computeGlobals`/`ForEntity` results feed `decideFromAttrs`, `readQuery`,
`grantsPermission` and `FieldVerdicts` — one place per axis, not per call site.
A new `SourceKind` (`SourceCeiling`) so a clamped denial is attributable and
distinguishable from a role-grant denial.

5. **Tooling + docs.** `rela acl audit` rules; `rela acl map --as <profile>`;
`docs/acl-security.md` trust-boundary section; `docs/acl-overview.md`.

**Alternatives rejected** (full reasoning on TKT-IAC8TX): runtime deny rules
(would force re-derivation of the whole evaluation core including SQL pushdown);
intersecting denial sets across profiles (denials cancel — `{salary} ∩ {bsn} =
{}` — forcing every profile to restate every denial); flat per-profile
allowlists restating everything (effort O(schema size) per profile, drifts as
the metamodel grows). `Tool` as a selector was dropped: it is self-asserted, and
mixing a spoofable key with signed claims invites operators to lean on the weak
one.

**Files to modify:**

- `internal/jwtauth/verifier.go` — `AssertionClaims`, `VerifyAssertion`
- `internal/principal/principal.go` — new fields + the nine methods that touch
them (`Verified`, `Sanitized`, `Equal`, `IsZero`, `Clone`, `principalJSON`,
`MarshalJSON`, `UnmarshalJSON`)
- `internal/dataentry/router.go` — `AssertedIdentity`, `verifiedPrincipal`,
`resolvePrincipalEntity` (must forward new fields or they drop silently)
- `cmd/rela-server/main.go` — the adapter
- `internal/acl/policy.go` — `Policy` fields, `knownPolicyKeys`, `Validate`,
`ValidateAgainstMetamodel`, the compilation
- `internal/acl/resolver.go`, `request.go`, `readquery.go`, `authz_write.go` —
the clamp
- `internal/acl/source.go` — new `SourceKind` (note: adding one is
compiler-silent; `sourceKindPriority`, both `String()` methods, `lessSource` and
five tables in `source_test.go` need entries)
- `internal/affordances/resolver.go` — field-verdict clamp
- `internal/aclaudit/tier_a.go`, `tier_b.go` — new rules
- `internal/aclmap/` — `--as` support
- `docs/acl-security.md`, `docs/acl-overview.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Validation | On invalid |
|---|---|---|
| `principal_type` claim (JWT) | Only after ES256 signature verification. Used solely as a **lookup key** into operator-authored `client_baselines` — never interpreted as a capability name. Length/count bounded like `roles`. | No matching baseline → unrestricted (gated by the user's own roles). AC11 audit rule surfaces uncovered types. |
| `scope` claim (JWT) | Same: verified-only, lookup key into `scope_grants`, bounded. | Unknown scope value → contributes nothing (silently dropped, mirroring the `Assignments` guard at `resolver.go:36-38`). |
| `acl.yaml` blocks | Structural validation at load: disjoint `applies_to`; not-both-spellings-per-type; types/fields cross-checked against the metamodel. | Hard startup error for the security-critical invariants (overlap, double-spelling); `slog.Warn` for drift, matching the tolerant-by-design convention. |

**Security-Sensitive Operations:**

- **Trust boundary (the critical one).** `internal/acl` verifies nothing — a
Principal is trusted absolutely. `principal_type`/`scope` MUST be populated only
after signature verification. The existing unexported-field +
`Verified()`-sole-constructor shape (`principal.go:58-95`) enforces this at
compile time; the new fields must keep it. A spoofable-header path setting them
would be a full authorization bypass.
- **Direction of failure.** Because a ceiling only ever *narrows*, a bug in the
clamp fails toward less access, not more. The one exception is the compilation
step: a `"*"` expansion or denylist→allowlist resolution that produces too large
a set would over-grant. That step needs direct tests, not just end-to-end ones.
- **Row-level denial is a secrecy boundary.** `deny_read` must make entities
*nonexistent* (404 indistinguishable from a real 404, pruned lists, no count
leak) per the row-level rule in CLAUDE.md — not merely redacted.
- **Config is not secret.** Per CLAUDE.md, baseline/scope names and the types
they mention are operator-authored config; a 403 naming the ceiling is correct
and useful. Do NOT contort to conceal profile names.
- **No new external I/O, no crypto, no filesystem surface.** Compilation is
pure; verification reuses the existing `jwtauth` path.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (AC → test):**

| AC | Scenario |
|---|---|
| 1 | `jwtauth` table test: assertion with/without `principal_type`+`scope` → Principal exposes both / empty, no error. Plus `--principal-header` and loopback unchanged. |
| 2 | `Policy.Validate` rejects two baselines both listing `app`; error names both blocks. |
| 3 | Explicit pinned test: `principal_type: user` (and an unknown string) → unrestricted. Must not drift. |
| 4 | Two-part: `redact:` — hide named fields, then add a field to the type fixture and assert it still shows. `visible:` — inverse: added field is hidden. |
| 5 | `Validate` rejects `visible:` + `redact:` for the same type in one block. |
| 6 | `deny_write: ["*"]` → create, update and delete all denied. |
| 7 | The ceiling-never-grants proof: same token + scope, two users (privileged / read-only) → strictly less for the lesser user. |
| 8 | `deny_permissions` withholds `history:read` and a command `permission:` the user's role grants. |
| 9 | `deny_read` → 404 identical to a real 404, entity absent from lists, count not incremented. |
| 10 | `rela acl map --as <profile>` golden output. |
| 11 | `aclaudit` table test: uncovered principal_type; no-op baseline; undeclared type/field. |
| 12 | Deny reason names the ceiling, distinguishable from a role-grant denial. |
| 13 | `NopACL`/`ReadOnlyACL` behaviour byte-identical. |

**Integration approach:** the unit tests above sit in `internal/acl` (using the
`World` fixture in `testutil_test.go`) and `internal/aclaudit`. End-to-end
coverage goes in `internal/dataentry` alongside the ~20 existing `acl_*_test.go`
files — a real HTTP request carrying a verified assertion, asserting the
response body is redacted/404'd. `internal/visibility/visibilitytest/suite.go`
must still pass unchanged (the ceiling must not break the Reader contract).

**Edge Cases:**

- Empty `scope` string; whitespace-only scope; scope naming no known grant.
- `applies_to: []` (matches nothing — dead config, audit should flag).
- A baseline naming a type absent from the metamodel.
- `"*"` in both allowlist and denylist form.
- `deny_read` on a type the user cannot read anyway (no-op, must not error).
- A scope re-opening something the *user* lacks (bounded by intersection → still
denied; this is AC7).
- Very many scopes on one token (bound it like `maxRoles`).
- Unicode / whitespace-padded profile keys — RR-IK355A on TKT-RP3X3Q records
that a whitespace-padded `asserted_role_assignments` key loads clean but is
permanently unmatchable. Same trap here; normalize and reject blank keys.
- Historical/deleted entities: field redaction already fails closed
(`PermHistoryReadRedacted`, TKT-73C6B2) — confirm the ceiling composes and does
not accidentally *reveal*.

**Negative Tests:**

- Unsigned / wrong-issuer / expired assertion carrying `principal_type` → 401,
claims never reach the Principal.
- A composite-literal-constructed Principal cannot carry `principal_type`
(compile-time; assert via the existing `Verified`-only discipline).
- Overlapping `applies_to` → startup fails, server does not boot.
- Both spellings for one type → load error, not a silent merge.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort: L.** Touches jwtauth, principal, dataentry, acl, affordances,
aclaudit, aclmap and docs, but each change is shallow; the load-time-compilation
choice is what keeps it from being XL.

**Risks:**

| Risk | Mitigation |
|---|---|
| Compilation over-grants (`"*"` expansion, denylist→allowlist) — the one direction that fails open | Direct unit tests on the compiler, not just end-to-end; golden-file the compiled output |
| A ceiling that silently protects nothing (baseline for a principal_type the IdP never sends) | AC11 audit rule; `rela acl map --as` for manual verification |
| Adding a `SourceKind` is compiler-silent (every switch has a default → renders "unknown", sorts at 999) | Five tables in `source_test.go`, `sourceKindPriority`, both `String()`s and `lessSource` need entries; TKT-RP3X3Q suggested a reflective guard — worth adding here |
| `Principal` field addition touches nine methods + the audit wire format | Follow the TKT-RP3X3Q precedent exactly; `principalJSON` uses `omitempty` so old consumers ignore new keys |
| `syncContext` already drops all claims (BUG-0Q8MCZ) — new fields drop too | Filed separately with a preventive measure (MEAS-PRINCIPAL-RESTAMP); not a blocker but adds a second silently-attenuated surface until fixed |
| `rela acl map` cannot model asserted claims today (`mapall.go:71-74`) | AC10 requires extending it; scope it as part of this ticket |
| 9-minute stale-claim window (assertion TTL hard-coded upstream, `signer.go:22`) | Document; do not attempt to fix |
| Perf: the clamp runs per request | It is a set intersection over already-resolved grants, and `Request` already caches `GlobalRoles`; benchmark against `internal/affordances/bench_test.go` if the field path shows up |

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/acl-security.md` — the trust-boundary section (why `principal_type`
is only trustworthy post-verification, why `Tool` was rejected as a selector)
and the row-level-denial semantics of `deny_read`
- [x] `docs/acl-overview.md` — the resolver vocabulary gains baselines and
scope grants; **pinned by `internal/acl/docfields_test.go` and
`docs_example_test.go`**, so these are not optional
- [x] `docs/cli-reference.md` — `rela acl map --as`, new `rela acl audit` rules
- [x] `CLAUDE.md` — decided YES during implementation. Added a "Restrictions
compile at LOAD time; the evaluator has no denial primitive" rule naming the
clamp point and the guard test, because "don't add a runtime deny" is exactly
what the next person would reintroduce.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel changes)
- [x] ~~`README.md`~~ (N/A: no project-level change)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design
was settled interactively with the user across several rounds; the rejected
alternatives and their reasoning are recorded in the "Why this shape" section of
TKT-IAC8TX. A full code review ran instead, after implementation.)
- [x] All critical/significant findings addressed — see the review responses
linked from TKT-IAC8TX (RR-B6DRAG critical, RR-48B6MT significant, both
addressed).

**Design Review Findings:** RR-B6DRAG (critical, addressed), RR-48B6MT
(significant, addressed), RR-UYB99D and RR-U41F3D (minor, addressed), RR-MUJUWS
(minor, deferred), RR-9K9NSJ (nit, wont-fix).
