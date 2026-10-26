---
id: RR-1BLTJD
type: review-response
title: A literal-only set skips the field write gate, so acl.yaml is no longer the only authority on field writes
finding: 'internal/appbuild/realtime_outputs.go:160-161 sends a set with no authored values to the ungated o.mgr.PatchEntity. A poster who holds a transition role and row update, but whose acl.yaml fields: grant denies writing (for example) status, can set status to the configured literal by posting the cause message. The plan and docs/realtime.md accept this. The rule is also inconsistent: when one interpolated value is present (lines 162-165), the gated manager checks the whole patch, literals included, so the same literal is then refused as field_denied. The precedent cited (recipe and automation literals) covers creates and system writes, not a person''s update of an existing row. Fix: send every set through o.gated (or the inline grant), so literals are field-gated like everything else and the pre-check, the write and acl.yaml agree. If the bypass stays, document the mixed-set behavior and pin it with a test, and add a load-time warning when a literal set targets a property that has a fields: write restriction.'
severity: minor
resolution: 'Resolved with the mixed-set fix: every set is field-gated; a FailedCodes row checks a literal owner gives field_denied.'
status: addressed
---
