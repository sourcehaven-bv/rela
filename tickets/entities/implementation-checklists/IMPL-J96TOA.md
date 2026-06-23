---
id: IMPL-J96TOA
type: implementation-checklist
title: 'Implementation: Sync 3/5: public ApplyEntity/ApplyRelation (id-preserving upsert, automation-suppressed)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — internal/entitymanager/apply_test.go
- [x] Integration tests written — exercises the real Manager + memstore + automation engine (suppression control path proves the automation actually fires through CreateEntity)
- [x] Happy path implemented — `ApplyEntity`/`ApplyRelation` in apply.go (id-preserving upsert via the existing internal upsertEntity/upsertRelation, with full ACL+validation+audit framing, no automation/cascade)
- [x] Edge cases handled — nil/empty-id/locked guards; create-vs-update audit op by existence; soft (DEC-HWZHA) vs hard validation; missing relation endpoint → ErrEntityNotFound; invalid relation type rejected
- [x] Error handling in place — validation errors surface as *ValidationError (API maps to 422); ACL denial surfaces; endpoint-missing is ErrEntityNotFound

## Test Quality

- [x] Using fixture builders or factories — reuses parseMeta/nopTemplater/countingStore/newManagerWithStoreAndAudit harness
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end — `go test -race ./internal/entitymanager/` green
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**
- `go test -race ./internal/entitymanager/` → ok; `golangci-lint` → 0 issues; `just arch-lint` → clean; `go build ./...` → OK; dataentry/cli consumers still pass.
- Coverage: ApplyEntity 91.7%, ApplyRelation 88.9%; package 86.4%.
- AC "ApplyEntity creates a short/sequential-id entity on the peer with the given id (no rejection), idempotent on second call": **PASS** — TestApplyEntity_PreservesExplicitSequentialID (also asserts CreateEntity *would* reject the same id) + TestApplyEntity_Idempotent.
- AC "audit record written; ACL gates; invalid content rejected (→422)": **PASS** — TestApplyEntity_AuditsCreateThenUpdate (create then update ops), TestApplyEntity_RejectsInvalidContent (hard error → *ValidationError, nothing persisted). Note: missing-required is SOFT per DEC-HWZHA (rides as warning) — TestApplyEntity_SoftWarningStillApplies pins sync mirrors that policy rather than inventing a stricter one.
- AC "automation suppression — applying a status change does NOT auto-create a checklist": **PASS** — TestApplyEntity_SuppressesAutomation, with a control path proving the same automation DOES fire through CreateEntity (so the assertion is meaningful, not vacuous).
- AC "existing CreateEntity/UpdateEntity behavior unchanged": **PASS** — full existing suite green; apply.go adds new methods only, touches no existing path.

## Quality

- [x] Code follows project patterns — mirrors CreateRelation's endpoint+ACL+validate sequence; uses the existing upsertEntity/upsertRelation and audit hooks; godoc explains why it differs from CreateEntity and from raw upsert
- [x] Checked for DRY opportunities — reuses upsertEntity/upsertRelation, authorizeAndAudit, partitionValidationErrors, recordEntityAudit/recordRelationAudit rather than duplicating
- [x] No security issues introduced — ACL still gates every apply; locked-entity guard prevents cleartext-over-ciphertext overwrite; audit always recorded
- [x] No silent failures — all errors returned; create-vs-update op is explicit
- [x] No debug code left behind

**Interface note:** ApplyEntity/ApplyRelation are public methods on *Manager but
intentionally NOT added to the transitional EntityManager producer interface
(slated for removal per its godoc + CLAUDE.md consumer-side-interface rule). The
sync consumers (sub-tickets 4 & 5) declare their own narrow interfaces at the
call site.
