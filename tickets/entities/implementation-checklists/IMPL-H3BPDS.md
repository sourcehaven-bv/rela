---
id: IMPL-H3BPDS
type: implementation-checklist
title: 'Implementation: Audit history:read-redacted reveals'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Three pieces:

1. `audit.OpHistoryReveal = "history-reveal"` (`internal/audit/audit.go`), with
a doc comment covering what it records, why it is a NEW op rather than a flag on
an existing one, why it is not `acl-bypass-read` despite the family resemblance,
and what it deliberately omits.
2. `(*App).recordHistoryReveal` (`internal/dataentry/history_handler.go`) —
builds the record from the stored snapshot and the request principal.
3. One call, on the reveal arm of `serveHistoryVersion` only, and only when
`revealIsPrivileged(a.acl)` -- a closed switch on the ACL implementation.

That third condition is not belt-and-braces. Under NopACL/ReadOnlyACL no
middleware attaches a read gate, so `readGateFromContext` returns `nopReadGate`,
whose `HoldsPermission` is true for every permission -- so the reveal arm is
taken on EVERY history read in an unconfigured deployment, with nothing redacted
and therefore nothing revealed. See RR-KBD2T2.

The tests drive the HTTP handler, not the helper: calling `recordHistoryReveal`
directly would prove the helper works while leaving "does the handler actually
call it" untested — which is the entire defect being fixed.

Edge cases from planning, all covered:

- no ACL configured → NOT recorded (`TestHistoryReveal_NoACL_NotAudited`).
The reveal arm is reached, but by a permit-all gate rather than a granted
permission, and nothing is redacted to reveal.
- reveal on an entity with nothing redacted → still recorded
(`TestHistoryReveal_AuditedEvenWhenNothingWasHidden`). The permission being
exercised is the auditable event, not whether it happened to uncover anything.
- ordinary redacted read → no record (`TestHistoryReveal_OrdinaryReadNotAudited`).
- nonexistent version → 404 before the reveal branch, so no record; unchanged
by this ticket and already covered by the existing history tests.
- `audit.Nop` sink → no panic. `auditSink` is non-nil by construction
(`NewApp` rejects nil at `app.go:748`, requiring an explicit `audit.Nop{}`), so
there is no nil branch to write and none to test.

Error handling: the reveal is deliberately NOT blocked on the audit write. Sink
errors are the sink's concern, exactly as for every other op — making this one
op fail the request would be a different (and much larger) decision about audit
durability.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Reuses the file's existing harness — `buildPolicyApp`, `permGate`,
`getHistoryVersion`, `decodeHistoryEntity`, `snapshot`, `historyRedactionACL` —
rather than inventing a parallel one. `revealAuditFixture` factors out the seed
shared by two of the three tests; the third seeds a deliberately different
entity (no redactable field), so sharing there would have obscured the point.

Both positive tests assert a PRECONDITION first — that the reveal actually
revealed, or that the ordinary read actually redacted — so a green audit
assertion cannot be produced by a request that took the wrong branch. Without
that, `TestHistoryReveal_OrdinaryReadNotAudited` would pass trivially if the
fixture ever stopped redacting.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Mutation-checked — the fix was broken and the tests re-run, then restored:

| mutation | expected | observed |
| --- | --- | --- |
| delete the `recordHistoryReveal` call | positive tests fail | `TestHistoryReveal_Audited` and `..._AuditedEvenWhenNothingWasHidden` FAIL |
| hoist the call ABOVE the permission branch (audit every read) | the negative fails | `TestHistoryReveal_OrdinaryReadNotAudited` FAIL, other two pass |
| flip the Nop/ReadOnly arm of `revealIsPrivileged` to `true` (the original bug) | the no-ACL test fails, alone | `TestHistoryReveal_NoACL_NotAudited` FAIL |
| flip its default arm to `false` (never audit) | the positives fail, alone | `..._Audited` and `..._AuditedEvenWhenNothingWasHidden` FAIL |
| restored | green | ok |

The last two mutations each redden EXACTLY the tests for the case they break and
leave the others green, which is what shows the three scenarios (configured +
reveal, configured + ordinary read, unconfigured) are independently covered
rather than one test standing in for all three.

The second mutation is the one that matters. Without
`TestHistoryReveal_OrdinaryReadNotAudited`, an implementation that audits every
history read passes the whole suite — and that implementation is worse than
useless, because a row that appears for every read cannot distinguish the
privileged disclosures it exists to surface.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows `audit.OpACLBypassRead` / `RecordElevatedRead` (TKT-ACSBSA), the
established shape for auditing a privileged READ: same principal attribution
(`principal.From(ctx)` — the real identity, not a system user), same
`TriggeredByFrom(ctx)`, same rule that disclosed content never enters the log.

DRY: deliberately NOT extracted into `internal/audit` alongside
`ElevationRecorder`. That type exists there because it is shared by a package
(`lua`) that must not depend on `principal`; this emit has exactly one call site
inside `dataentry`, which already imports both. A second recorder type for one
caller would be indirection without a contract.

A harness inaccuracy this exposed was also fixed: `buildPolicyApp` built a
Declarative resolver but left `app.acl` as `NopACL`, so a test handing it a
policy modelled a CONFIGURED deployment for field redaction and an UNCONFIGURED
one for anything switching on the ACL implementation. It now installs the policy
as `app.acl` too. Left alone, the new guard would have looked correct while
being tested against the wrong wiring.

Security: the recorded entity TYPE is taken from `snap.Type` (the stored
snapshot), never the caller-supplied URL segment — the record is forensic
evidence, and sourcing it from the request would let a caller write a type of
their choosing into the audit log. Same reasoning already documented for
relation history, where trusting a caller-supplied `fromType` would let a
principal key a grant to a type they do not hold.

The record carries no revealed values and no revealed field names; a test
asserts the revealed value does not appear in the Summary.
