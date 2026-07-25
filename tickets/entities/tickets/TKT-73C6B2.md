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

## Re-verification (2026-07-25, against develop dd0fe649)

Premise STILL VALID — and now SHARPER. TKT-9E57 ("predicate-backed _fields
resolver", done) made the field-visibility resolver genuinely conditional-grant
capable: `internal/affordances/resolver.go:629-654` evaluates a `When` predicate
per grant via `prog.Eval`, and the bindings resolve `has_edge`/`count_relations`
against the LIVE store (`internal/dataentry/affordances_policy.go:96-106`). For a
deleted entity the live store returns no edges, so a conditional `visible:` grant
flips at read time — exactly the under-redaction this ticket describes, now a live
policy capability rather than a hypothetical. History is still redacted at read
time against the live ACL (`internal/dataentry/history_handler.go:194-208` →
`entityserializer.go:117` → `affordances.go:895` stripHiddenProperties); the
capture path stores no frozen verdict (`store.go` VersionSnapshot carries only
content/properties/schema-projection). This ticket's fix is unimplemented and its
value went UP with TKT-9E57.

Coupled with TKT-B1F5Q1: both say "build the capture-time freeze with a shared
design" — 73C6B2 is the entity-side freeze, B1F5Q1 adds relation-side `visible:`
which needs the same freeze if relation grants are conditional. Design them
together.
