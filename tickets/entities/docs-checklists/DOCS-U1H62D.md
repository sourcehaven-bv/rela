---
id: DOCS-U1H62D
type: docs-checklist
title: 'Documentation: Surface org_id and roles from verified identity assertions (TKT-RP3X3Q)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Godoc on new exported types and functions

`jwtauth.AssertionClaims` / `VerifyAssertion`, `principal.Verified` / `Clone` /
`Sanitized` / `IsZero` / `Equal` / the three accessors, `acl.RoleList` /
`AssertedGrant` / `AssertedGrants` / `SourceAsserted`,
`aclmap.ConditionalGrant`, `dataentry.AssertedIdentity`.

- [x] Rationale recorded where a reader would otherwise ask "why"

The comments that carry weight rather than restating the code:

- `principal.Principal` — why the claims are unexported (the ACL trusts the
Principal absolutely, so a role from an unverified source is a bypass, not a
degradation).
- `principal.UnmarshalJSON` — why it is safe (reads back what this process
wrote) and the one thing that would make it unsafe (pointing it at request-path
input).
- `principal.Clone` — that this is the one place the compiler stops enforcing
the guarantee the unexported fields provide.
- `acl.normalizeAssertedRoles` — the exact failure it prevents, and why it
lives in `Validate` rather than `LoadPolicyBytes`.
- `aclmap.ConditionalGrant` — why it is NOT merged into `Everyone`, in terms
of the question the report exists to answer.
- `acl.numSourceKinds` — that it must stay last, and what breaks otherwise.

- [x] `internal/principal` package doc updated

The doc explicitly gated growth on "a future ACL ticket". That gate is now
recorded as opened once, with the reason, and the new limit stated so the
warning keeps its force rather than reading as spent.

## Project documentation

- [x] `docs/acl-overview.md` — new "Roles from a verified identity assertion"
section: both YAML forms, and a "Rules and fallbacks" list matching the idiom
every other policy key follows (no-match, undeclared role, `everyone` rejection,
exact-after-trim matching, absent-key no-op, multi-claim accumulation,
no-user-entity case).
- [x] `docs/acl-security.md` — new section covering the three load-bearing
properties: the trust boundary and why it is structural; the claim→role
indirection as the second control; and revocation lag stated concretely (**a
revoked role survives in rela for up to one token lifetime — 9 minutes with
Pratique — and rela has no revocation channel**), because an operator planning
incident response will otherwise assume IdP revocation is immediate. Plus a
dedicated "`org_id` is recorded, not enforced" subsection.
- [x] `docs/server-security.md` — the widened claim surface on the JWT path,
what is optional, and pointers to the two ACL docs.
- [x] `docs/audit-log.md` — the new record fields, a worked example, the
backward-compatibility note, and the org-is-not-isolation warning repeated where
an operator reading the log will hit it.
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `docs/data-entry.md`~~
(N/A: no metamodel, CLI-surface or UI changes — the `acl who-can` text output
changed but its flags and invocation did not)

## Verification

- [x] Documented examples are executable

`TestDocs_AclOverviewExampleWorks` parses the YAML copied **verbatim** from
`docs/acl-overview.md` and asserts the documented union-and-dedup behaviour. A
documented example that no longer parses is worse than no example, because a
reader will trust it — so this fails if the schema drifts.

- [x] Warnings placed where the reader is, not only where the author was

The org-is-not-isolation warning appears in three places (`principal` godoc,
`acl-security.md`, `audit-log.md`) because the three audiences who could
mistakenly rely on it — an implementer reading the type, an operator writing
policy, an investigator reading the log — each arrive from a different door.
