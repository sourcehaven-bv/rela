---
id: DOCS-RJ8E1Q
type: docs-checklist
title: 'Documentation: unmatched_principal reject (TKT-0C3II2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Godoc on the new exported/load-bearing symbols

`Policy.UnmatchedPrincipal` + the `Unmatched{Anonymous,Reject,Provision}`
constants (each mode's meaning + the reserved-provision note);
`Policy.PrincipalPropertyLookupEnabled` (why an out-of-package caller needs
"attempted-vs-not-configured"); `acl.WithUnmatchedVerified` /
`UnmatchedVerifiedFrom` (what the flag means, who sets it, why it's wiring-state
not per-principal); the `AuthorizeWrite` reject branch (why before role
evaluation, why it's the single choke point).

- [x] Rationale recorded where a reader would ask "why"

- The `resolvePrincipalEntity` flag-set: why `jwtVerified && id=="" &&
lookupEnabled` (distinguishes verified-no-match from header and from
lookup-disabled).
- The `router.go` "SetJWTGate must precede NewRouter" invariant (RR-E12CN2 nit).
- The anti-bypass test comment naming the covered-by-construction write paths.

## Project documentation

- [x] `docs-project/entities/guides/GUIDE-acl-security.md` — new
"Unknown verified identities (`unmatched_principal`)" section: the three modes,
the write-only + reads-allowed reject posture, the data-entry-path scope, the
lookup-required load invariant, and the accurate 403-disclosure statement
(corrected per RR-E12CN2). **Edited the SOURCE entity**, not the generated
`docs/acl-security.md` (the lesson from TKT-RP3X3Q).
- [x] Confirmed `scripts/generate-docs.sh` then `git diff --exit-code docs/
README.md` is clean — the generated file reflects the source, no drift (the CI
Docs-job requirement).
- [x] ~~docs/server-security.md, metamodel, CLI, data-entry~~ (N/A: no server-
flag, schema, CLI, or UI surface change — the key lives in `acl.yaml` alongside
`principal_property`, documented in the ACL guide).

## Verification

- [x] The documented behaviour is machine-checked: the three modes and the
load invariant are pinned by `TestUnmatchedPrincipal_ValidatesEnum` /
`_RejectRequiresLookup`, and the write-only reject by the e2e suite — so the doc
claims can't drift from the code.
- [x] The 403-disclosure statement matches the actual wire body
(`writeForbiddenIfACLDenied`), verified in code review (RR-E12CN2).
