---
id: REV-MQMGJ5
type: review-checklist
title: 'Review: Provision a stub user entity for an unmatched verified principal (unmatched_principal: provision)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green; `-race` clean on dataentry/acl)
- [x] Lint clean (`golangci-lint run ./...` → 0 issues; arch-lint OK; plimsoll OK)
- [x] Coverage maintained (`just coverage-check` exit 0, no floor violations)

## Code Review

- [x] Run `/code-review` — the cranky-code-reviewer agent was launched but died on an API error (connection closed mid-response) before producing findings. A focused manual security/correctness review was completed in its place (see below); re-run the automated reviewer when convenient.
- [x] ~~All critical review-responses addressed~~ (none found)
- [x] ~~All significant review-responses addressed~~ (none found)
- [x] Self-reviewed the diff for unrelated changes (only the .golangci.yml contextcheck exclusion + cmd/rela-server email adapter are outside internal/; both are directly required by this change)

**Review Responses:** none new. Design findings RR-28SCW3 (addressed) and
RR-VI9XMY (addressed — seam corrected) carried from design review.

**Manual review coverage (seven risk areas, all clear):**
1. Re-stamp seam reaches manager AND every response-shaping read on all paths — `r = h.enterWrite(r)` reassigns the request so `gateRead`/`reader`/`serializer` (all take `r`, read `r.Context()`) see the rebuilt gate. Verified on CRUD/clone/conflict-resolve/relations.
2. Flag-clear (`WithMatchedVerified`) applied ONLY on the fully-successful provision path (provision.go:111); every error branch returns the original unmatched ctx with the flag intact.
3. Fail-closed: all provision errors (`return ctx`) fall back to the UNMATCHED principal, never a forged/elevated one.
4. Concurrency: `ErrEntityAlreadyExists` caught → re-resolve; serialized under the already-held writeMu in-process; `sub` unique for cross-process. 8-way concurrent test → exactly one stub.
5. Bare-stub containment intact: `system:provisioner` is create-user-only; cascade authorizes as it and cannot author edges (RR-28SCW3).
6. Sync path: `syncContext(r.Context())` re-stamps Tool=sync on the PROVISIONED ctx, preserving `p.User` (resolved id) and the `acl.WithRequest`/read-gate ctx values (separate keys); flag stays cleared. Correct.
7. Email trust boundary: `WithEmail` reachable only from an already-`Verified` principal or `verifiedPrincipal` (post-signature-verification); no unverified source.
- Webhook path (`/webhooks/idp`) is off `/api/`, so the unmatched-verified flag never sets there — correctly excluded from the enterWrite seam (its own IdP-enrichment provisioning is a separate ticket concern).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS (FirstWriteProvisionsAcrossPaths +
GetDoesNotProvision), AC2 PASS (TriggeringWriteSeesOwnStub), AC3 PASS
(ConcurrentFirstWritesCreateOne), AC4 PASS (acl
ValidateAgainstMetamodel_ProvisionRequiresGrant), AC5 PASS
(AuditedToProvisioner), AC6 PASS (CRUD+sync driven; action/attachment pinned by
ProvisionSeam_EveryWriteHandlerUsesEnterWrite).

## Documentation (enhancements only)

- [ ] Docs-checklist created and linked via `has-docs`
- [ ] User-facing documentation updated (docs/acl-security.md: `unmatched_principal: provision` semantics + operator responsibility for the bare stub / migration grant)
- [ ] Docs-checklist marked as done

**Docs Checklist:** pending — to create when resuming (docs-project/entities
source).

## Final Checks

- [x] Commit message explains the why (feat commit + design captured on the ticket/RRs)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** pending — branch `feat/provision-unmatched-principal` committed locally,
not yet pushed (awaiting user authorization for the outward push/PR).
