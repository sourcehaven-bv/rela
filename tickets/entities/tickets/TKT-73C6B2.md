---
id: TKT-73C6B2
type: ticket
title: Historical field redaction fails closed — deny-by-default with a history:read-redacted reveal permission
kind: enhancement
priority: medium
effort: m
status: done
---

Follow-up to TKT-9INY0Y / review finding RR-TPATBK. Design superseded 2026-07-27
(see Design decision) — the original "freeze the visibility verdict/inputs at
capture" framing is REPLACED by deny-by-default + an audit reveal permission.

## Problem

serveHistoryVersion redacts a historical snapshot by reconstructing an
*entity.Entity and running it through the dataentry serializer (forWire ->
stripHiddenProperties -> FieldVerdicts). FieldVerdicts resolves field visibility
against the LIVE ACL context, which no longer matches the moment the version was
captured:

- relation-dependent grants (`visible: has_relation(...)` / `count_relations(...)`)
  evaluate against the LIVE store's outgoing edges — empty/changed for a
  deleted or drifted entity → a grant that was DENIED at write time flips OPEN.
- the subject-side half of role grants (`has_role(...)`) resolves via `ForEntity`
  against the live ACL graph — for a deleted entity only `everyone` resolves, so
  the entity confers no roles and role-gated fields mis-evaluate.

So a field correctly hidden at write time can UNDER-REDACT (leak) in the
snapshot the moment the policy uses a CONDITIONAL `visible:` grant. TKT-9E57
(predicate-backed `_fields` resolver, done) made conditional grants a live
capability, so this is a real leak, not a hypothetical.

The three inputs to a field verdict, and how history must treat each:

| Input | Example binding | History treatment |
|---|---|---|
| ACL policy (`visible:` rules) | the `when:` predicate itself | LIVE — today's policy |
| Reader | `current_user`, roles the reader holds | LIVE — the reader as they are now |
| Subject-world | entity's own edges / roles it confers (`has_relation`, subject half of `has_role`) | CANNOT be reconstructed for deleted/drifted entities |

The reader-side (Scenario 3/5) is already correct: reader roles are live and
that IS the intended stance (a promoted reader gains historical visibility). The
BUG is purely the subject-world: the live store can no longer answer
"what edges did this entity have at version V", and a conditional grant that
consults it fails OPEN.

## Design decision (2026-07-27)

Do NOT freeze subject-world inputs at capture. That approach ("replay the world
as-of-V") is only as safe as our fidelity in capturing every input a predicate
could touch — a subtle capture bug leaks — and was rejected as too magical.

Instead, **fail closed**: any historical field the current policy cannot
AFFIRMATIVELY grant `visible` to (including every conditional grant whose
subject-world inputs cannot be trusted for a historical/deleted entity) is
HIDDEN by default. Security becomes structural — unknown ⇒ hidden — not
reconstructed.

Revealing those fields requires a new global named permission
**`history:read-redacted`**, the field-grained sibling of the existing
`history:read` (PermHistoryRead, internal/acl/policy.go:44) audit
super-permission. Semantics are **OVERRIDE** (all-or-nothing, exactly like
`history:read`): a holder sees ALL frozen historical fields, skipping the strip
step entirely. Supplement semantics ("only un-hide fields that failed for the
right reason") were rejected as the same magic we deleted. Granted via a role's
`permissions:` list like the delegate-X permissions.

The frozen content already CONTAINS every field (`VersionSnapshot.Properties`
holds the full record; redaction is a serialization-time strip), so the reveal
is purely "skip the strip for this reader" — NO capture-schema change, no new
table, no frozen-edge-set.

## Invariant

> Reading version V shows a field to a normal reader only if TODAY's ACL policy
> affirmatively grants it `visible` against the inputs that can be safely
> evaluated; every grant that would need untrusted subject-world state fails
> CLOSED (hidden). A holder of `history:read-redacted` sees all frozen fields.

- Policy → live.  Reader → live.  Subject-world → NOT reconstructed → fail closed.

## Acceptance criteria (scenarios → tests)

Policy under test (conditional field grants):

```yaml
visible:
  - field: bonus_note
    when: not has_relation("reviewed-by")   # subject-side
  - field: salary
    when: has_role("hr")                     # reader-side (routed through subject)
```

1. **Subject edge gone** — V3 captured with a `reviewed-by` edge (bonus_note
   hidden). Edge later removed. Reading V3 as a normal reader: `bonus_note`
   HIDDEN (fail closed — the subject-world grant cannot be affirmed). No leak.
2. **Deleted entity** — entity hard-deleted; normal reader requests V5: every
   subject-conditional field HIDDEN (fail closed). Reader with
   `history:read-redacted`: all fields shown.
3. **Reader-side role, two readers** — V2, `salary` grant `has_role("hr")`. HR
   reader: salary VISIBLE (live reader role). Plain viewer: HIDDEN. Same version,
   per-reader — reader side stays live and correct.
4. **Mixed / over-redaction is acceptable** — `salary` visibility routed through
   a since-removed subject edge. Normal reader (even one who legitimately held
   the role via that edge): HIDDEN (fail closed — we do NOT reconstruct the edge;
   over-redaction is the safe failure). `history:read-redacted` reader: shown.
5. **Reader promoted after capture** — reader gains `hr` after V1 captured;
   reading V1 now shows `salary` (reader roles are live — intended). Demotion
   symmetrically re-hides. (Confirmed acceptable.)
6. **Relation history (TKT-B1F5Q1)** — same rule for relation `visible:`:
   relation history fails closed, same `history:read-redacted` reveal.
7. **Since-removed property** — today's policy references `entity.department`
   but V's record predates the rename (had `dept`). Frozen record binds
   `department` as Nil (DR-C2), predicate fails → field HIDDEN (fail closed).
   Pinned as a test — live policy over a drifted schema over-redacts, never
   leaks.

## Notes

- Touch points: serveHistoryVersion redaction (internal/dataentry/history_handler.go
  ~194-208 → entityserializer.go:117 → affordances.go:895 stripHiddenProperties);
  the resolver's fail-closed behavior (internal/affordances/resolver.go passes()
  already fails closed on predicate error — the work is ensuring subject-world
  bindings for a historical/deleted entity route to that fail-closed path rather
  than to a live-store lookup that silently returns "no edges" → grant flips).
- New permission constant `PermHistoryReadRedacted = "history:read-redacted"`
  beside PermHistoryRead; documented in docs/acl-security.md alongside it.
- No entity_versions / schema_versions schema change. No capture-path change.
- FUTURE (RR-73CA/L1): the sounder long-term design is to FREEZE the
  subject-world (edge/role state) at capture time — like the schema projection
  — so both edge and role predicates answer as-of-version instead of being
  blinded. That removes the over-redaction this deny-by-default accepts, and
  removes the need to enumerate every subject-world input to neuter (the
  enumeration that originally missed role resolution). Deliberately out of scope
  here (larger capture-path lift); deny-by-default is the security-complete
  stopgap.

## Re-verification (2026-07-25, against develop dd0fe649)

Premise STILL VALID — and now SHARPER. TKT-9E57 ("predicate-backed _fields
resolver", done) made the field-visibility resolver genuinely conditional-grant
capable: `internal/affordances/resolver.go:629-654` evaluates a `When` predicate
per grant via `prog.Eval`, and the bindings resolve `has_relation`/`count_relations`
against the LIVE store (`internal/dataentry/affordances_policy.go:96-106`). For a
deleted entity the live store returns no edges, so a conditional `visible:` grant
flips at read time. History is still redacted at read time against the live ACL
(`internal/dataentry/history_handler.go:194-208`); the capture path stores no
frozen verdict.

Coupled with TKT-B1F5Q1 — the relation-side `visible:` — which inherits this
same deny-by-default + reveal rule. Design them together.
