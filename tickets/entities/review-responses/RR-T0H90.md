---
id: RR-T0H90
type: review-response
title: Legacy /api/entities POST silently broke for callers supplying id on non-manual types
finding: 'Before this PR, POST /api/entities with {id: "TKT-CUSTOM", type: "ticket", ...} was accepted (workspace honored the custom ID for any type, gated only by duplicate check). After this PR validateCreateIDOpts is also called from handleAPICreateEntity at internal/dataentry/handlers_api.go:386, so the same payload now returns 400/422. APICreateEntityRequest is a documented public-facing JSON shape used by mobile clients. Pre-fix this was zero-test-coverage on the legacy path.'
severity: significant
resolution: Added TestHandleAPICreateEntity_IDValidation in handlers_api_test.go covering rejection for non-manual types, unknown prefix, and whitespace-trim behavior. Status switched to 422 for consistency with the v1 handler. The break is intentional — accepting arbitrary client-supplied IDs for short/sequential types undermines the whole id_type concept — and is now pinned by tests so it cannot silently regress further.
status: addressed
---
