---
id: RR-99A0Z
type: review-response
title: API reference doc + CLAUDE.md mention + footgun callout all missing
finding: |-
    PLAN-CAK3L line 514 promised docs/data-entry/api-reference.md with comprehensive relations section (wire format, list-level rules, edge-level rules, propagation, atomicity caveats, data: [] footgun callout, JSON:API non-conformance note). File doesn't exist. Plan checklist line 672 ticked but nothing was written.

    Line 519 promised CLAUDE.md mention. Grep confirms not added.

    RR-6YF8F resolution promised the data:[] footgun callout in the API ref — doesn't exist (because the file doesn't exist). Frontend types in entities.ts have JSDoc on upsert semantics but NO callout that data:[] deletes everything. The auto-save data-loss risk this was designed to prevent is undocumented.

    Fix: write docs/data-entry/api-reference.md sections per the plan. Add CLAUDE.md note. Add the footgun callout to the JSDoc on RelationsUpdate too.
severity: significant
resolution: Created docs/data-entry/api-reference.md with comprehensive coverage of wire format, list-level and edge-level rules, symmetric/inverse propagation, validation status codes, atomicity caveats (with honest documentation of two-phase commit limits), SSE events, ETag, no-op suppression, out-of-scope notes. Updated CLAUDE.md with a 'Unified PATCH endpoint' section. Added a flashing-red footgun callout to RelationsUpdate JSDoc in frontend/src/api/entities.ts.
status: addressed
---
