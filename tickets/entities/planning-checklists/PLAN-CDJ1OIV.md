---
id: PLAN-CDJ1OIV
type: planning-checklist
title: 'Planning: Record entity and relation audit rows inside the write transaction (fail closed)'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the audit record of every stored entity and relation write
(create, update, patch, delete, soft delete, restore, purge, rename, convert,
copy, move/renumber, replace relations, batch, linked create, automation
cascades) is appended on the write's own transaction view on postgres and
sqlite. A failed append fails and rolls back the write. fs/memory keep the
best-effort sink, which now receives records only after the commit. Out:
denied-write, acl-bypass, version-tag and rename-refusal records (they describe
no stored write) stay on the async sink. Read API (TKT-X1ZFO04).

**Acceptance Criteria:**
1. On sqlite and postgres a create whose audit append fails returns the error and leaves no entity (TestAuditFailureRollsTheWriteBack).
2. On sqlite and postgres an update whose audit append fails leaves the stored value unchanged (same test).
3. A successful create's record is in the database log when the call returns (same test, Verify rows = 1 without waiting).
4. On a rollback store with a best-effort sink, a rolled-back batch or linked create records nothing (TestCreateBatch_MidBatchFailureRollsBack, TestCreateLinked_InTxErrorRollsBack).
5. On fs/memory a kept partial write is still recorded (existing kept-entity tests).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-HVW3B7O already covers it)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal wiring of the existing sqlaudit log)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows the in-repo records/flowruns join pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-HVW3B7O; decision DEC-U78WD7Y.

**Existing Solutions:**
- appbuild/records_tx_*.go and flowruns_tx_*.go: join a store transaction from a view (pgstore.HandleFor, sqlitestore.TxConn + sqliteexec.NewInTx).
- jobs.WithDeferral: hold side effects until commit; mirrored by heldAudit for the best-effort sink.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** entitymanager declares TxAudit {RecordTx(ctx, view, rec)
error}, type-asserted from Deps.Audit (Deps is at its plimsoll field cap).
emitWrite routes a record to RecordTx on the write's view, or to the best-effort
sink. auditTx opens (or joins) the transaction for every auditing write path and
holds best-effort records until commit. appbuild wraps the sqlaudit sink in
txAuditSink whose join builds a pgaudit log over the pgx.Tx handle or a
sqliteaudit log over the tx connection (savepoint "audit"). Alternatives: a new
Deps field (rejected: plimsoll cap); a store decorator that buffers on commit
(rejected: CLAUDE.md forbids wrapping Tx); keeping post-commit async append
(rejected by DEC-U78WD7Y).

**Files to modify:**
internal/entitymanager/{audittx,audithook,core,manager,owning,batch,linked,cardinality,joinedreplace,ifmatch_delete,recreate,conversion,conversion_flow,manager_order,softdelete,cascadehost,copy,unique}.go;
internal/appbuild/{auditlog,audit_tx_postgres,audit_tx_sqlite,audit_tx_nodb}.go;
internal/audit/sqliteaudit/sqlite.go; docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new input. Records are built from the same
write data as before. The savepoint name is a fixed constant.

**Security-Sensitive Operations:** The audit chain lock is taken inside the
write transaction (pg advisory xact lock), so a write and its chain row commit
atomically; a join on a non-transaction handle is refused (errAuditNotTxView) so
a record can never commit independently of its write. Fail closed: an append
error rolls the write back.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1-3: appbuild TestAuditFailureRollsTheWriteBack on sqlite
and postgres builds (trigger on audit_log raising). AC4-5: entitymanager
batch/linked tests over memstore, fsstore, sqlite.

**Edge Cases:** Nested writes (cascade inside a write) join the open view
instead of opening a second Tx. Writes on fs that fail part-way record what
stayed.

**Negative Tests:** Append failure for create and update returns the trigger's
error and stores nothing.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Lock contention: every write now also takes the chain lock until
commit; writes were already serialized by the store's advisory write lock, so no
new ordering. A write path that records outside its view gets
errAuditOutsideTx/errAuditNotTxView in tests rather than a silent async record.
Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/audit-log.md (Delivery), docs/content-states.md
(copy audit), internal/entitymanager/CLAUDE.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design settled in accepted DEC-U78WD7Y and RES-HVW3B7O; implementation goes through /code-review)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
