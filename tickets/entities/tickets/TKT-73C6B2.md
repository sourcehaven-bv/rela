---
id: TKT-73C6B2
type: ticket
title: Freeze field-visibility verdict at version capture so historical redaction is context-correct
kind: enhancement
priority: medium
effort: l
status: backlog
---

Follow-up to TKT-9INY0Y / review finding RR-TPATBK.

## Problem

serveHistoryVersion redacts a historical snapshot by reconstructing an
*entity.Entity (content + properties only) and running it through the dataentry
serializer (forWire -> stripHiddenProperties -> FieldVerdicts). But
FieldVerdicts resolves field visibility against the LIVE ACL context:
- relation-dependent grants (visible: has_edge(...)) evaluate against the live store's relations — empty for a deleted entity → grant flips;
- roles come from ForEntity against the live ACL graph — for a deleted entity only 'everyone' resolves → role-scoped hidden fields un-hide.

So a field correctly hidden at write time can UNDER-REDACT (leak) in the
snapshot the moment the policy uses a CONDITIONAL visible: grant. Unconditional
per-type visible: grants redact correctly today; this ships v1 with that
documented boundary (docs/acl-security.md + a code comment at
serveHistoryVersion).

## Fix

Freeze the field-visibility verdict (or the inputs needed to recompute it) at
version-capture time and redact the snapshot against THAT — exactly the pattern
VersionSnapshot.Projection already uses to render against the
schema-as-of-version. This makes historical redaction correct-by-construction
and removes the whole 'verdict context mismatch' class.

## Notes

- Interacts with the entity_versions capture path (entitymanager version hook + sweep) and the store VersionSnapshot shape.
- Consider whether to store the full resolved verdict, or the (relations + roles) inputs, or a per-field visible/hidden bitmap.
- The relation-set-as-of question overlaps with relation history (TKT-VFJKMB).
