---
id: RR-5AZRCI
type: review-response
title: Document export route has no faced test
finding: Nothing checks that /_documents/policy_report/POL-1@draft/_export is 404 for a published-only reader and renders the draft for an all-faces reader.
severity: significant
resolution: 'TestAnchoredDocumentExport_FaceGate: alice on POL-1@draft/_export is the uniform 404; bob gets the draft. export_render on entity export never sees a face because entity export 404s faced addresses (TKT-5SZG2L)\; there is no server-side per-entity document list.'
status: addressed
---
