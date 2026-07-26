---
id: TKT-B1F5Q1
type: ticket
title: Relation field-level ACL redaction (visible:) — currently absent for relations, live and history
kind: enhancement
priority: medium
status: backlog
---

## Problem

The ACL has **no field-level (`visible:`) redaction for relations at all** —
neither on a live relation GET nor in relation history. Entities have it
(`serializer.forWire` → `stripHiddenProperties`, `entity.Inaccessible`);
relations do not:

- `entity.Relation.Inaccessible` exists but is **never populated** (zero writers).
- The live relation GET emits `rel["meta"] = edge.Properties` **raw** (api_v1.go).
- `RelationVerdicts` gates relation TYPE create/read, not per-field `visible:`.

TKT-92JL8P (relation versioning) documented this honestly and scoped around it:
relation history exposes exactly what a live relation GET exposes, no more, and
bounded relation-history visibility with **dual-endpoint gating** (RR-SDDYZO)
instead. But the underlying gap remains: any deployment that puts sensitive data
in relation properties has no way to hide individual fields from a reader who is
allowed to see the relation at all.

## Why it matters

Relations carry rich content (a property map + markdown body). A relation like
`(person, employed-by, org)` might carry `salary` or `approval_note` in its
properties. Today there is no `visible:` mechanism to redact those per-field the
way an entity's properties can be redacted — so relation properties are
all-or-nothing on the relation's read verdict.

## Fix direction

1. Extend the ACL policy schema to allow `visible:` grants on relation types /
relation fields (mirror the entity per-type `visible:` shape).
2. Populate `entity.Relation.Inaccessible` on the read path from the resolved
verdict (the field is already there, waiting).
3. Route the live relation GET (`rel["meta"]`) and the relation-serialization
path through a `stripHiddenProperties` equivalent.
4. Relation **history** then inherits it for free (the snapshots go through the
same serializer) — closing the "relation history has no redaction" caveat from
TKT-92JL8P (RR-BZNL0S) at the same time.

## Interaction with TKT-73C6B2

Freezing the visibility verdict at capture time (TKT-73C6B2) is the entity-side
correctness fix for CONDITIONAL grants. If relation `visible:` grants can also
be conditional, the same freeze applies — build these two with a shared design
so relation history doesn't reintroduce the conditional-grant under-redaction
hole.

## Origin

Confirmed as a gap in the TKT-92JL8P follow-up review — "acl does not allow
field level stuff [for relations], seems like a gap we need to address."

## Re-verification (2026-07-25, against develop dd0fe649)

Premise STILL VALID — relations still have no read-time field-visibility
redaction. Intervening ACL work (TKT-9E57 predicate-backed resolver, TKT-ZF2DTV
ACL-bound script reads) strengthened the ENTITY side but added nothing for
relations:

- The policy schema still cannot express a relation `visible:` grant:
  `internal/acl/policy.go:271-277` `RelationGrant` has only
  `Relation`/`Create`/`Remove`/`Fields`/`When` — no `Visible`.
- The live relation GET still emits properties raw:
  `internal/dataentry/api_v1.go:893,918,1043` all do `rel["meta"] =
  edge.Properties` with no stripping. `stripHiddenProperties` exists only for
  entities (`affordances.go:895`, called from `entityserializer.go:117,142`).
- `RelationVerdict.Fields` gates WRITES only (`affordances.go:226-229`;
  consumers are the write-path validators `validateRelationMetaWrite` +
  `_actions` emission) — never a read-side redaction.

CORRECTION to the Problem section: the claim "`entity.Relation.Inaccessible` …
never populated (zero writers)" is now imprecise. `fsstore/markdown.go:349`
(`buildInaccessibleRelation`) DOES write it — but only for git-crypt-encrypted
relation files (reason `git-crypt`), a filesystem-encryption stand-in, NOT wired
from any ACL verdict. So the substantive gap (no ACL-driven per-field relation
redaction) is intact; only the "zero writers" phrasing is stale. Fix direction
step 2 should say "populate from the ACL verdict" (distinct from the existing
git-crypt writer).

Still coupled to TKT-73C6B2 — see its re-verification note; design the
capture-time freeze once and cover both.
