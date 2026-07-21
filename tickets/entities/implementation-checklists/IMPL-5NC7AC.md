---
id: IMPL-5NC7AC
type: implementation-checklist
title: 'Implementation: Surface org_id and roles from verified identity assertions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] All edge cases from planning handled

**What was built** — five layers, as planned:

1. `internal/jwtauth`: `AssertionClaims` + `VerifyAssertion` beside the
existing `VerifySubject`; `stringSliceClaim` helper; bounds (32 roles × 256
runes) applied in the projection so every future consumer inherits them. jwtauth
stays an arch-lint leaf.
2. `internal/principal`: `orgID`/`orgSlug`/`roles` as **unexported** fields
behind `Verified()`, plus `OrgID()`/`OrgSlug()`/`Roles()` accessors, `IsZero()`,
`Equal()`, `Sanitized()`, and custom JSON marshalling.
3. `internal/dataentry`: own `AssertedIdentity` type + widened
`assertionVerifier` seam; `cmd/rela-server` adapter translates from
`jwtauth.AssertionClaims`; `resolvePrincipalEntity` rebuilt via `Verified` so
claims survive the re-stamp.
4. `internal/acl`: `asserted_role_assignments` (`map[string]RoleList` with
scalar-or-list YAML), `SourceAsserted` + `Source.Claim`, injection in
`computeGlobals`, `Validate` rejects blank keys and `everyone` targets.
5. `aclmap`/`cli`/`audit`: separate "conditional grants" reporting,
`Claim` on `Route` + `lessRoute`, per-element audit sanitisation.

## Manual verification

- [x] Feature tested end-to-end
- [x] Each acceptance criterion verified
- [x] Verification evidence documented

**Evidence — each AC exercised by a named test:**

| AC | Test | Result |
|---|---|---|
| AC1 | `TestVerifyAssertion_ProjectsAllClaims`, `TestJWTPrincipalResolver_CarriesOrgAndRoles` | PASS |
| AC2 | `TestVerifyAssertion_AbsentClaimsAreNotAnError` (5 cases incl. explicit JSON null), `TestJWTPrincipalResolver_AbsentClaimsAreNotAnError` | PASS |
| AC3 | Existing `TestHeaderPrincipalResolver_*` pass **unmodified**; `TestE2E_HeaderModeUnaffected` | PASS |
| AC4 | Existing default-resolver tests unmodified; `TestNonJWTResolvers_CannotSetAssertedClaims/default_resolver` | PASS |
| AC5 | `TestAssertedRoles_ClaimGrantsMappedRole` (asserts `SourceAsserted` + `Claim`), `_ClaimGrantsSeveralRoles`, `TestE2E_AssertedClaimAuthorizesRequest` | PASS |
| AC6 | `TestAssertedRoles_UndeclaredRoleDroppedSilently` | PASS |
| AC7 | `TestPrincipal_Sanitized`, audit round-trip tests | PASS |
| AC8 | Existing `nop_test.go` / `readonly_test.go` unmodified | PASS |
| AC9 | `TestWhoCan_ReportsAssertedGrantsSeparately`, `_AssertedGrantsAreNotPrincipals`, `_NoAssertedMappingsOmitsSection` | PASS |
| AC10 | `TestAssertedRoles_UnmatchedPrincipalKeepsAssertedRoles`, `TestE2E_UnmatchedPrincipalStillCarriesClaims` | PASS |

**Two guards were fault-injected rather than assumed:**

1. **The exhaustive SourceKind guard.** First version was WRONG — it derived
the count from the hand-maintained list it was meant to check, so adding an
unregistered kind passed. Rewritten against `numSourceKinds` (from the const
block itself). Re-injected the fault: now fails with *"allSourceKinds has 7
entries but 8 kinds are declared"*. Also confirmed the `exhaustive` linter does
**not** catch this (`default-signifies-exhaustive: true` means the `default:`
clauses satisfy it) — the test is the only guard.
2. **The re-stamp drop.** Reverted `resolvePrincipalEntity` to the original
composite literal; `TestE2E_ClaimsSurvivePrincipalPropertyReStamp` failed with
*"resolvePrincipalEntity dropped the claims when it rebuilt the Principal"*.
Restored; passes. This is the review's RR-HVN18E scenario made mechanically
detectable.

**Documented example verified:** `TestDocs_AclOverviewExampleWorks` parses the
YAML copied verbatim from `docs/acl-overview.md` and asserts the documented
union-and-dedup behaviour, so the docs cannot drift silently.

## Quality

- [x] Code follows project patterns
- [x] No silent failures (errors surfaced, not just logged)

- `just test` — full suite passes
- `just lint` — 0 issues
- `just arch-lint` — OK, no warnings (**jwtauth remained a leaf**, confirming
the adapter approach from RR-CHW2AA)
- `just coverage-check` — PASS (76.3%, both thresholds)
- `go test -race` on all five touched packages — clean

**Deliberate non-changes:**

- `isUnstamped` untouched (RR-HVN18E). `TestAssertedRoles_UnstampedPrincipalStillRejected`
pins that a claims-bearing but identity-less principal is still rejected.
- `aclmap` schemaVersion left at 1: both new fields are `omitempty`, so a
policy without asserted mappings serialises byte-identically
(`TestWhoCan_NoAssertedMappingsOmitsSection` asserts the JSON).

**Opportunistic fix:** `Principal.RawUser` was never sanitised in the audit
writer (safe by construction, but an unpinned invariant in the exact function
this ticket edits). Now covered via `Principal.Sanitized`.
