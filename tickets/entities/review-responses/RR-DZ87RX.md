---
id: RR-DZ87RX
type: review-response
title: No end-to-end test of refused traversal fallback
finding: Only appbuild covered unsupported→fallback with a fake gate; a regression treating unsupported as denied would turn a 422 into a silent empty page unnoticed.
severity: minor
resolution: TestListPushdown_RefusedTraversalFailsTheRequest asserts 422 query_scope_unsupported for bob on a chain landing on an editor-of type then walking backwards; mutation (unsupported treated as denied) fails it.
status: addressed
---
