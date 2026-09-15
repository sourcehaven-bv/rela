---
id: RR-MZE4IA
type: review-response
title: Document the _export routing premise at the dispatch site and add a guard test; note body redaction is unchanged
finding: 'Three documentation/test gaps, no behavior change. (1) The routing premise holds (entity.ValidateID rejects a leading underscore at id.go:211, ParseStateRef validates the base before splitting on @ at face.go:89, and isSafePathSegment rejects @ anyway), but ValidateID''s own godoc says the leading-character rule is ''a well-formedness rule, NOT a security control''. Using it for routing is acceptable since an ambiguous route has no structural alternative, but the dispatch site must say so, and a guard test should pin it. The entity router at api_v1.go:238-241 relies on the same premise with a one-line comment and no test. (2) exportFilename: isSafePathSegment permits ''.'', so a document named report.tar yields report.tar.pdf. safeAttachmentFilename sanitizes stem and extension separately and rejoins with one dot, so this is cosmetic, not injection, but deserves a test case. (3) docs/transforms.md should state that document export exposes entity BODIES exactly as the HTML render does, because visibility.Redact leaves Content verbatim (the body-redaction TODO in internal/visibility/policyreader.go). This ticket adds no new exposure, but the docs should not imply export is redaction-equivalent to the entity path.'
severity: minor
resolution: 'Accepted. Dispatch requires parts[2]==_export exactly for 3 segments and treats parts[1]==_export as the standalone export for 2, with a comment citing entity.ValidateID''s leading-character rule as the premise and a guard test pinning it. validateDocuments rejects a document named _export. exportFilename gains a test for a dotted document name. docs/transforms.md states that document export carries entity bodies exactly as the on-screen render does. Confirmed and recorded: document export must NOT add field-level redaction, since Lua reads through VisibleReader and re-redacting would violate visibility.Redact''s non-composability contract. Implementation pending.'
status: addressed
---

## Resolution

1. Dispatch on segment count and exact position — for a 3-segment path require
`parts[2] == "_export"` and 404 otherwise; for 2 segments treat `parts[1] ==
"_export"` as the standalone export and anything else as an entity id. Add a
comment citing `entity.ValidateID`'s leading-character rule as the premise, plus
a guard test so a future grammar relaxation fails loudly rather than silently
shadowing the route. Keep the `validateDocuments` rule rejecting a document
named `_export` — that is the half that actually closes the gap.
2. Add an `exportFilename` test for a document name containing a dot.
3. In `docs/transforms.md`, state plainly that a document export carries entity
bodies exactly as the on-screen document does.

## Confirmed by the review, recorded so it is not re-litigated

**Document export must NOT add field-level redaction.** Entity and list export
route through `visibility.Reader` because they render entity properties
directly. A document renders whatever its Lua prints, and that Lua reads only
through `lua.ReadDeps.VisibleReader` — a nil VisibleReader denies and never
falls back to a raw handle (RR-X9NVHI), and TKT-80EWGM removed the last raw
handle. Redaction is therefore already applied one level below the renderer.
Applying `visibility.Redact` again at the export boundary would violate its
non-composability contract (raw store entities in, exactly once).
