---
id: IMPL-MQXV67
type: implementation-checklist
title: 'Implementation: Audit already-deleted relations when a cascade delete fails partway'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — two in `internal/store/fsstore`
(`TestDeleteEntity_PartialCascade_ReportsWhatWasRemoved`, and the
first-relation-fails boundary), placed beside the `#888` fail-secure test they
extend and reusing its `storage.NewErrorFS` fault injection.
- [x] Integration tests written — `TestAudit_PartialCascadeDelete_AuditsWhatWasRemoved`
drives `Manager.DeleteEntity` end to end against a store that reports a partial
cascade, asserting on the real audit sink.
- [x] Happy path implemented
- [x] Edge cases from planning handled — first-relation-fails, `os.IsNotExist`
not counted, entity-file and attachment-dir abort paths.
- [x] Error handling in place — the error still propagates unchanged; only the
accompanying result is new.

## Test Quality

- [x] Using fixture builders or factories for test data — `openStore` /
`newConfig` / `storage.NewErrorFS`, the helpers the neighbouring recovery tests
already use.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — the fault injector targets one
relation file by substring; everything else delegates to the real FS.
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Both new behaviours were verified by MUTATION.* A test that asserts "the log
records what happened" is worthless unless it is known to fail when the log
records nothing.

**AC1 — store reports the partial set.** Reverting the store to `return nil,
err`:

```
--- FAIL: TestDeleteEntity_PartialCascade_ReportsWhatWasRemoved
    Messages: a partial cascade must report what it removed
--- FAIL: TestDeleteEntity_PartialCascade_FirstRelationFails
```

**AC2 — manager audits it.** Removing the `auditPartialCascade` call:

```
--- FAIL: TestAudit_PartialCascadeDelete_AuditsWhatWasRemoved
    audit_test.go:739: want 1 delete-relation record for the relation actually removed, got 0
```

**The integration test earned its place immediately.** With only the store and
the outer manager hook in place it still failed — `deleteEntityInTx` was
discarding the partial result (`return nil, nil, fmt.Errorf(...)`) before the
outer `res` was ever assigned. A store-level unit test would have passed while
the feature did nothing end to end; the missing propagation is now explicit and
commented.

**AC3 — no over-reporting, and code review found I had the mirror-image bug.**

My first version returned a partial result from the attachment-dir abort with
`DeletedEntities` empty, and added a comment asserting that could never be
wrong. It was: by that point the entity file is **already removed**, so the
result under-reported the entity — #929's own failure mode reproduced one layer
down by the fix for #929 (RR-UE2XS7). Attachment-dir failure is now non-fatal:
the delete has materially succeeded at that point, so failing it would report
failure for an operation that worked. Pinned by
`TestDeleteEntity_AttachmentDirFailure_StillSucceeds`, mutation-verified.

Review also found that making `DeletedRelations` non-empty on the error path and
consuming it for **audit only** broke RR-181AFY's rule that it is the single
source for ALL relation-delete capture — the audit log would say the relations
died while version history had no delete marker, unrecoverably (RR-JA8WRT). The
helper now records both.

Original AC3 reasoning, still true for the two genuine abort paths: The same test asserts **zero** `delete-entity`
records. The entity survives a failed cascade (the store aborts before touching
its file, per #888's ordering), so recording its deletion would be the opposite
error — and a log that over-reports is harder to catch than one that
under-reports. The `os.IsNotExist` branch is likewise not counted as
deleted-by-us: this call did not remove that file.

**AC4 — success path untouched.** `git diff` shows no edit to any existing
success-path test, and the full suite passes: `go test ./...` **exit 0**. Builds
under all four backend tags.

**AC5 — contract, not requirement.** The godoc says a partial result MAY
accompany an error and that transactional backends return nil; `storetest`
conformance is unchanged, so pgstore/sqlitestore are not newly obliged. Their
`storetest` runs still pass.

**Gates:** `just lint` 0 issues (it caught two `testifylint` `require-error`
violations in the new tests), `comment-lint`, `arch-lint`, `plimsoll`, `lint-md`
all clean.

## Quality

- [x] Code follows project patterns — the manager's partial-audit helper reuses
the success path's `recordRelationAudit` and the identical
`cascade:delete-entity:<id>` label, rather than inventing a second audit shape.
`cascadeHost.DeleteEntity` uses the same label for the `if_exists: replace`
route, so all three stay consistent.
- [x] Checked for DRY opportunities — the three abort paths in the store share
one `removed` accumulator rather than each rebuilding a result; the manager's
loop is one small helper rather than a copy of the success loop. Not merged with
the success path: that one also records the entity, which is exactly what must
NOT happen here.
- [x] No security issues introduced — this is an audit-log change, so the
direction of error matters: it can only ADD records for deletions that genuinely
occurred. Authorization is untouched (the ACL check runs before the store call
and a partial failure does not re-enter it), and the records carry relation
identity only, same as the success path.
- [x] No silent failures — the whole ticket is the removal of one: a deletion
that happened and was not logged.
- [x] No debug code left behind.

**One contract note for review.** Returning a non-nil value beside a non-nil
error departs from Go's usual "error means ignore the value", so it is
documented explicitly on `store.EntityWriter.DeleteEntity` rather than left for
a reader to infer — including that a partial result is *not* success and that
`DeletedEntities` stays empty whenever the entity survived.
