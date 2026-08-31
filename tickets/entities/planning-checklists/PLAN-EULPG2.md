---
id: PLAN-EULPG2
type: planning-checklist
title: 'Planning: Audit history:read-redacted reveals'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: emit one audit record when a history version read takes the
`history:read-redacted` REVEAL arm at `history_handler.go:224`.

OUT (each with a reason, so this is not silent under-delivery):

- *Relation history.* There is deliberately no reveal there: a deleted
relation's meta is served to nobody and a live one is redacted against today's
live policy (see the doc comment above `serveRelationHistoryVersion`). No
reveal, nothing to log.
- *Ordinary, non-reveal history reads.* What makes this disclosure notable is
that a permission overrode a redaction. Auditing every history read is a
different and much larger decision about read-auditing in general, and would
bury the reveals it is supposed to surface.
- *The listing endpoint.* It serves timeline metadata (version, op, who, when),
not frozen field VALUES, so it discloses nothing the reveal permission governs.

**Acceptance Criteria:**

1. A reveal read emits exactly one audit record with op `history-reveal`.
*Test:* memory audit sink; GET a version as a principal holding
`history:read-redacted`; assert one record, correct op.
2. The record identifies WHAT was revealed — entity type, id, and the version.
*Test:* assert `Subject.Kind == "entity"`, Type/ID match, and the version
appears in Summary.
3. The record identifies WHO, using the real principal.
*Test:* assert Principal matches the requesting principal.
4. An ORDINARY (non-reveal) history read emits NOTHING.
*Test:* same request as a principal without the permission; assert the sink is
empty. This is the discriminating case — a test that only checks the reveal arm
would pass on an implementation that logs every read.
5. Under NO configured policy, a history read emits NOTHING.
*Test:* NopACL app; GET a version; assert the sink is empty. **ADDED AFTER
REVIEW** (RR-KBD2T2) — see Risks.
6. The response body is unchanged by the audit call.
*Test:* assert the served JSON with auditing wired matches what it was before
(reveal still reveals; redaction still redacts).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, and the pattern is already settled in-repo.

**Existing Solutions:**

The governing prior art is `audit.OpACLBypassRead`
(`internal/audit/audit.go:69`, helper in `internal/audit/elevatedread.go`),
added by TKT-ACSBSA for the same shape of problem: an elevated READ that the
write-oriented audit log could not see. Its doc comment settles two questions
this ticket would otherwise re-litigate:

- **Separate op, not a flag on an existing one.** Folding a read into an
existing op silently changes what every stored query for that op means.
- **Never record the disclosed content.** Copying ACL-protected values into the
audit log is a wider disclosure than the read being recorded.

Both rules are adopted here.

Where this case DIFFERS from `OpACLBypassRead`, and why the difference is real
rather than a reason to reuse it: a bypass closure's read set is unbounded (one
`admin.list_entities` can walk the graph), so that record carries no subject and
is emitted once per closure. A history reveal is one entity at one version —
bounded, known, and cheap to name. So this record DOES carry a Subject. Reusing
`acl-bypass-read` would also be wrong on its face: no `bypass_acl` closure is
involved, and it would pollute an existing forensic query.

`internal/aclaudit` was checked and is not the right home — it records ACL
DECISIONS, whereas this records a disclosure that a decision permitted.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add `audit.OpHistoryReveal = "history-reveal"` next to `OpACLBypassRead`,
with a doc comment in the established house style: what it records, why it is
separate, what it deliberately omits, and the forensic query that isolates it.
2. Emit from the reveal branch in `serveHistoryVersion`, through the existing
`a.auditSink` (non-nil by construction — `NewApp` rejects a nil sink at
`app.go:748`), using `principal.From(ctx)` and `audit.TriggeredByFrom(ctx)`
exactly as `RecordElevatedRead` does.
3. Summary carries the version and the fact of reveal; it does NOT carry field
names or values. Field NAMES are borderline — they leak the shape of what is
hidden — and they are recoverable from the entity type plus the policy at that
time, so they are omitted.

Record shape:

Op:        audit.OpHistoryReveal Subject:   {Kind: "entity", Type: snap.Type,
ID: entityID} Principal: principal.From(ctx) TriggeredBy:
audit.TriggeredByFrom(ctx) Summary:   "history_reveal=true version=<n>"

**Alternatives considered:**

- *Reuse `OpACLBypassRead`.* Rejected: no bypass closure is involved, and it
would corrupt the meaning of an existing forensic query.
- *Log inside `forWireHistoricalReveal` (the serializer).* Rejected: the
serializer is a pure rendering step with no audit dependency, and the
entity/version identity it would need to name is the handler's knowledge.
Auditing there would also fire for any future caller that renders a reveal for a
non-disclosure reason.
- *Record which fields were revealed.* Rejected on the `OpACLBypassRead` rule —
the field list is a map of what the policy hides, which is close to the thing
being protected.

**Files to modify:**

- `internal/audit/audit.go` (new op constant + doc)
- `internal/dataentry/history_handler.go` (emit on the reveal arm)
- `internal/dataentry/history_handler_test.go` or a sibling test file

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `entityID` and `typeName` come from the URL path; `version` is already parsed
and range-checked (`version < 1` rejected) before this point. The entity TYPE
written to the record is taken from `snap.Type` — the stored snapshot — not from
the caller-supplied URL segment, so a caller cannot spoof the recorded type.
This mirrors the reasoning already documented for relation history, where
trusting a caller-supplied `fromType` would let a principal key a grant to a
type they do not hold.
- No new external input, no file paths, no crypto.

**Security-Sensitive Operations:**

This IS the security-sensitive operation: it makes a privileged disclosure
observable. Two properties matter and are tested — it must fire on every reveal
(AC1) and must NOT fire on ordinary reads (AC4), since a record that appears for
everything is as useless as one that never appears.

The audit write must not become a covert channel: the record contains no
revealed values, only the identity of the entity/version and the reader.

Failure mode: `audit.Filesystem` errors are handled inside the sink, as for
every other op. The reveal is not blocked on a successful audit write — the same
trade-off every existing op makes, and changing it for this one op is out of
scope.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one per acceptance criterion above, driven through the HTTP
handler with a `audit.Memory` sink, not by calling the emit helper directly —
otherwise the test proves the helper works while the handler never calls it.

The three scenarios that must be independently covered are (configured +
reveal), (configured + ordinary read) and (unconfigured). Each mutation of the
implementation must redden only its own scenario's test; if one mutation reddens
several, the tests are not actually distinguishing the cases.

**Edge Cases:**

- principal WITH the permission on an entity that has nothing redacted: still a
reveal read, still recorded. The permission is what is being audited, not
whether it happened to matter this time.
- version that does not exist: 404 before the reveal branch, no record.
- `audit.Nop` sink: no panic, no record.

**Negative Tests:**

- AC4 (ordinary reader emits nothing) is the load-bearing negative.
- Mutation check: delete the emit call and confirm AC1 reddens; make the emit
unconditional (move it above the permission branch) and confirm AC4 reddens.
Both are required — the first alone would pass on an implementation that logs
every history read.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *"Took the reveal arm" is not the same as "was granted a reveal".* This risk
was MISSED in planning and found in review (RR-KBD2T2). Under NopACL and
ReadOnlyACL no read gate is attached, so `readGateFromContext` returns
`nopReadGate`, whose `HoldsPermission` returns true for every permission — every
history read in an unconfigured deployment takes the reveal arm, with nothing
redacted to reveal.

The plan reasoned carefully about WHO holds the permission and never asked what
the permission check RETURNS when there is no policy to check against. That is
the generalizable lesson: on this codebase a permission probe is not a fact
about a principal, it is a question answered by whichever gate is wired, and the
default gate says yes. `permitsGatedUIElement` already existed as the settled
answer and the plan should have found it.

Mitigated by `revealIsPrivileged`, a closed switch on the ACL implementation,
and pinned by a test in both directions.

- *Audit write on a read path costs latency.* Bounded here: one record per
reveal request, and reveals are rare by construction (a small, deliberately
granted role). This is exactly why per-row logging was rejected for
`OpACLBypassRead` and is not a concern at this granularity.
- *A new op breaks a downstream log consumer.* Mitigated by adding a new op
rather than changing an existing one; consumers filtering on known ops are
unaffected.

**Effort:** s

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] Wherever the audit log's op vocabulary is documented, since this adds a
new op an operator can query for. To be confirmed against `docs/` during
implementation and recorded in the docs-checklist.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the two decisions worth flagging are recorded above
with their reasoning: a NEW op rather than reusing `acl-bypass-read`, and a
populated Subject where `acl-bypass-read` deliberately has none (its read set is
unbounded; this one is a single known entity version).
