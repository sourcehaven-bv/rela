---
id: RR-BXXUC6
type: review-response
title: Fail closed on a missing read gate; model the endpoint 403 on documents, never permitsGatedUIElement
finding: 'readGateFromContext (readgate.go:154-173) returns nopReadGate when nothing is attached to ctx, and that gate permits everything including HoldsPermission=true. attachACLRequest (router.go:266-345) skips non-API paths, so the gate exists only on correctly-routed /api/v1/ requests — a new endpoint registered outside that prefix, or a test harness skipping middleware, silently gets full-graph reads. Mitigations to copy: register strictly under /api/v1/; have the gantt handler fail closed (visibility.DenyReader-style) if the ctx gate is absent, following gatedScriptReader (app.go:558-580) which returns DenyReader on any construction fault rather than reverting to raw reads (rationale at app.go:414-431). Separately: if the gantt endpoint gets a real permission gate (justified as an expense gate — the fold is O(tree)), the pattern is gateDocumentPermission (standalone_document_handler.go:41-51), the only per-surface 403 in the codebase. permitsGatedUIElement is a UX filter and its own doc (views_handler.go:262-265) forbids using it as authorization.'
severity: significant
resolution: 'Plan updated: handler registers strictly under /api/v1/ and fails closed (DenyReader-style, per gatedScriptReader app.go:558-580) when the ctx read gate is absent; a dedicated fail-closed test is in the test plan. Any future endpoint-level permission gate is modelled on gateDocumentPermission (standalone_document_handler.go:41-51); permitsGatedUIElement is confined to the sidebar entry.'
status: addressed
---
