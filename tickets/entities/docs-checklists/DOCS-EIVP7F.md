---
id: DOCS-EIVP7F
type: docs-checklist
title: 'Documentation: Asserted roles inert on the production JWT gate (TKT-OJL2GN)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code documentation

- [x] Godoc on new/changed exported and load-bearing internal symbols

- `verifiedPrincipal` (router.go) — the shared projection: what it does, that
it is the single place claims become a Principal (so gate and resolver can't
drift), and the request-path trust-boundary caveat.
- `JWTGateConfig.Verifier` — why it is `assertionVerifier` not
`subjectVerifier`, naming the bug a subject-only verifier reintroduces.
- `assertionVerifier` interface — updated to name both consumers (gate +
deprecated resolver), replacing the stale "so the resolver is testable" text.
- `assertionVerifierAdapter` (main.go) — mirrors `webhookVerifierAdapter`; why
it consumes `VerifyAssertion`.
- `webhook.go` — stale `subjectVerifier` reference corrected to
`assertionVerifier`.

## Project documentation

- [x] ~~docs/server-security.md, docs/acl-*.md~~ (N/A — see below)

**Documentation Impact: none.** This is a corrective code-only change. The
user-facing behaviour it delivers — asserted roles actually granting through the
production JWT gate — is exactly what the shipped docs (from TKT-RP3X3Q) already
describe. Those docs were correct in intent and wrong only in that the code did
not match them on the gate path; this change makes the code match the docs.
Nothing to add, change, or regenerate.

- [x] Confirmed no `generate-docs` drift: the change touches no `docs/` or
`docs-project/` file, so `git diff --exit-code docs/ README.md` after
`scripts/generate-docs.sh` is clean (the CI Docs job requirement).
- [x] ~~docs/cli-reference.md, docs/data-entry.md, docs/metamodel.md~~
(N/A: no CLI, UI, or metamodel surface change)

## Verification

- [x] The behaviour the docs promise is now pinned by
`TestJWTGate_AssertedRolesReachACL` (asserted role → ACL grant through the real
router) and `TestRequireVerifiedJWT_StampsAssertedClaims` (org/roles on the
stamped Principal) — so the doc/code agreement is machine-checked, not just
asserted here.
