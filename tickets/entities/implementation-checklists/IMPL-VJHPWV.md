---
id: IMPL-VJHPWV
type: implementation-checklist
title: 'Implementation: Audit rejected attachment uploads (CONTROL-8-15)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — two in
`internal/dataentry/handlers_attachment_audit_test.go`.
- [x] Integration tests written — both drive the real upload handler end to end
(multipart request → `handleV1AttachmentRoute` → attachment service → audit
sink), not the helper in isolation.
- [x] Happy path implemented
- [x] Edge cases from planning handled — the negative case is the point: an
upload that *succeeds* must produce no denied-write record.
- [x] Error handling in place — the helper is a no-op for any error that is not
`attachment.ErrRejected`.

## Test Quality

- [x] Using fixture builders or factories — `newTestAppV1`, `seedEntity`,
`mustNewACL` and `putAttachmentAs`, the helpers the neighbouring attachment
tests already use.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — one entity, one role granting exactly
read+update so the ACL passes and the *processor* is what refuses.
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Mutation-verified.* Removing the `auditRejectedUpload` call from the upload
handler:

```
--- FAIL: TestAttachmentUpload_RejectionIsAudited
    no denied-write audit record for the rejected upload; got 0 record(s)
```

*The rejection is real, not simulated.* The test uploads PNG magic bytes to the
fixture's `screenshot` property, which declares `accept: [text/plain]`. The
sniffed MIME fails the allowlist and the attachment service returns
`ErrRejected` — so the test exercises the actual policy path rather than a
stubbed error.

*The negative case is deliberate.*
`TestAttachmentUpload_NonRejectionIsNotAudited` uploads an accepted file and
asserts **no** denied-write record. Without it, "audit every failed upload"
would pass — and that would dilute the op until `op == "denied-write"` stopped
distinguishing anything. A size cap or an at-capacity property is a client
error, not a security event.

*Field-level assertions.* The record must name the principal (who tried), the
subject entity, the filename, the target property, and the reason. Asserting
only "a record exists" would let an empty one through, and an empty security
record is worse than none — it looks like coverage.

## Quality

- [x] Code follows project patterns — mirrors the ACL denial already recorded on
this same handler, down to the `OpDeniedWrite` op, the `Subject` shape and the
`TriggeredBy` propagation.
- [x] Checked for DRY opportunities — deliberately **not** merged with the ACL
denial's record. They fire at different points for different reasons; a shared
helper would need both a decision and an error and would obscure which gate
refused.
- [x] No security issues introduced — this **adds** a security-log record. The
one judgement worth stating: it reuses `OpDeniedWrite` rather than adding a new
op, so an operator filtering `op == "denied-write"` sees both kinds of refused
upload without needing to know there are two. The Summary distinguishes them.
- [x] No silent failures — that is precisely what this removes.
- [x] No debug code left behind.

**Placement note.** The audit call sits at the upload call site rather than
inside `writeAttachmentWriteError`, because that helper is a package function
with neither the audit sink nor the entity in scope — and the record needs both.
Its godoc says so, so the next person does not try to move it there.
