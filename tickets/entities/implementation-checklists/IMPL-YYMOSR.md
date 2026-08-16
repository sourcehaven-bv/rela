---
id: IMPL-YYMOSR
type: implementation-checklist
title: 'Implementation: Provision a stub user entity for an unmatched verified principal (unmatched_principal: provision)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (migration Detect/Apply/idempotency/lockstep; acl AC4 grant validation; principal email threading via existing suites)
- [x] Integration tests written (provision_e2e_test.go drives CRUD/sync through the real router; seam invariant test)
- [x] Happy path implemented (lazy stub create under system:provisioner, re-stamp + gate rebuild, write proceeds)
- [x] Edge cases from planning handled (concurrent → unique-constraint catch+re-resolve; GET doesn't provision; matched principal untouched; declared-only stub props)
- [x] Error handling in place (provision failure logs + falls back to unmatched, never fail-open; AC4 fails at LOAD)

## Test Quality

- [x] Using fixture builders (provisionApp mirrors rejectApp; gateVerifier stub)
- [x] No hardcoded values where object is in scope (assert stub props against the verified claims)
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object

## Manual Verification

- [x] Feature manually tested end-to-end (e2e tests drive the production router; race detector clean)
- [x] Each acceptance criterion verified with a test scenario
- [x] Edge cases verified (concurrent 8-way create → exactly one stub)

**Verification Evidence:**
- AC1: TestProvision_FirstWriteProvisionsAcrossPaths (CRUD create/update, sync) + TestProvision_GetDoesNotProvision.
- AC2: TestProvision_TriggeringWriteSeesOwnStub (read-back of the provisioned person succeeds → re-stamp + gate rebuild took effect).
- AC3: TestProvision_ConcurrentFirstWritesCreateOne (8 concurrent first-writes → 1 stub).
- AC4: acl TestValidateAgainstMetamodel_ProvisionRequiresGrant (LOAD fails without the provisioner grant).
- AC5: TestProvision_AuditedToProvisioner (only system:provisioner could have created the person; audit inherits via Manager).
- AC6: seam covers CRUD+sync driven; action/attachment share enterWrite, pinned by TestProvisionSeam_EveryWriteHandlerUsesEnterWrite.
- Full: `go test ./...` green; `-race` clean on dataentry/acl; `golangci-lint run ./...` 0 issues; arch-lint OK; plimsoll OK; coverage-check exit 0.

## Quality

- [x] Code follows project patterns (migration mirrors acl_scheduler_grant; consumer-side metaView interface; enterWrite mirrors syncContext discipline)
- [x] Checked for DRY (single maybeProvision helper + one enterWrite per handler type; shared provisionCtx closure)
- [x] No security issues introduced (stub built only from verified claims; create-only system:provisioner containment RR-28SCW3; fail-closed on provision error; email/org are attribution-only, not ACL-evaluated)
- [x] No silent failures (provision errors logged AND fall back to unmatched authorization)
- [x] No debug code left behind
