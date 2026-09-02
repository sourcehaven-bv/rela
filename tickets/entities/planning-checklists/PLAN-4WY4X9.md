---
id: PLAN-4WY4X9
type: planning-checklist
title: 'Planning: Audit rejected attachment uploads (CONTROL-8-15)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** an upload the attachment processor refuses — disallowed MIME type,
or a failed/positive scan — returns HTTP 422 and writes **no audit record**.
GitHub issue #1050 (IB-review rela#1026), CONTROL-8-15.

A refused upload is a security-relevant exception: it may be an attempt to place
a disallowed file type or malware into the project, and the audit log is the
only place that question can be answered afterwards.

**Scope — IN:** one audit record on the `attachment.ErrRejected` path, carrying
principal, entity, property, filename and reason.

**Scope — OUT:**

- Auditing other upload failures. A size cap, an at-capacity property or a
transient I/O error are ordinary client/server errors; recording them would
dilute the op until it distinguishes nothing.
- A new audit op. The ACL denial on this same handler already uses
`OpDeniedWrite`; a second op would split one operator question across two
filters.
- Anything about *detection* — what the processor rejects is unchanged.

**Acceptance Criteria:**

1. A rejected upload produces one audit record naming the principal, the subject
entity, the property, the filename and the rejection reason.
2. A *successful* upload produces no denied-write record.
3. The record is distinguishable from the ACL denial recorded on the same
handler, without being a different op.

## Research

- [x] For larger features: run `/research`
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, with an exact in-tree template.

**Existing Solutions:** the ACL denial **on this same handler**
(`handlers_attachment.go`, `h.audit().Record(...)` with `OpDeniedWrite`) is the
template: same op, same `Subject` shape, same `TriggeredBy` propagation, Summary
carrying the reason. So the gap is specifically the *processor-policy* rejection
sitting beside an already-audited ACL rejection — which is also the argument for
reusing the op rather than inventing one.

`audit.OpDeniedWrite`'s own doc frames the forensic question this serves: *"what
did this user try to do that they weren't allowed to?"*

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** an `auditRejectedUpload` method on `attachmentHandler`,
called at the `WriteAttachment` error site, which returns early unless the error
is `ErrRejected`.

**Placement:** at the call site, not inside `writeAttachmentWriteError` where
the 422 is written — that helper is a package function with neither the audit
sink nor the entity in scope, and the record needs both.

**Files to modify:** `internal/dataentry/handlers_attachment.go`, plus tests.

**Alternatives considered:**

1. *A new `OpRejectedUpload`.* Rejected — an operator asking "what uploads were
refused?" would have to know to query two ops. The Summary carries the
distinction.
2. *Audit inside `internal/attachment`.* Rejected — the service has no audit sink
and should not gain one; CLAUDE.md puts audit on the write path, and the CLI
attach path would then double-record.
3. *Audit every upload failure.* Rejected — see Scope; it would dilute the signal.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the filename is caller-supplied. It is written
into the Summary string of a JSONL record, so the framing question matters: the
audit writer escapes the field, and a hostile filename cannot break the record.
It is *not* echoed back to the client beyond the existing 422 body, which is
unchanged.

**Security-Sensitive Operations:** this adds to the security log rather than
changing behaviour. Two properties worth stating:

- *No rejected bytes are recorded* — only the filename and the policy reason.
Recording content would put the very thing the policy refused into the audit
log, which is a durable, widely-read file.
- *The reason comes from the processor*, which produces policy text
("disallowed MIME type"), not attacker-controlled content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

- **AC1** — upload PNG magic bytes to a `text/plain`-only property; assert 422
and one `OpDeniedWrite` record with principal, subject, filename, property and
reason. Uses the real allowlist, not a stubbed error.
- **AC2** — upload an accepted file; assert **no** denied-write record. Without
this, "audit every failure" would pass.
- **AC3** — assert the Summary contains `rejected upload`, distinguishing it from
the ACL denial's `denied: ...` wording.

**Edge Cases:** a size-capped upload and an at-capacity property must not record
— both are covered by AC2's shape (any non-`ErrRejected` error is a no-op in the
helper).

**Negative Tests:** AC2 is the negative test, and it is the one that stops the
op from becoming meaningless.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| Auditing too much dilutes `denied-write` | Helper returns early for any non-`ErrRejected` error; AC2 pins it |
| A record exists but is empty (looks like coverage, is not) | Field-level assertions on principal, subject, filename, property, reason |
| Someone later moves the call into `writeAttachmentWriteError` | The godoc says why it cannot live there |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/audit-log.md~~ (N/A: the guide documents the record *shape* and the
`triggered_by` vocabulary, neither of which changes. This adds an occurrence of
an already-documented op, not a new kind of record.)
- [x] ~~CLI reference / metamodel docs~~ (N/A: no flag, command or schema change)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the shape is
dictated by the ACL denial recorded fifteen lines away in the same file. The
only judgement calls — reuse the op, and audit only `ErrRejected` — are recorded
under Alternatives and Scope.)
- [x] All critical/significant findings addressed in plan — none raised.

**Design Review Findings:** N/A.
