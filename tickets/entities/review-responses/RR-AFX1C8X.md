---
id: RR-AFX1C8X
type: review-response
title: Test gaps around table row editing
finding: 'No EntityDetail-level test for per-address instances or handleTableRowApplied; no verdict-flip or cap-boundary page test; budget test lacks visible: related grants and grouped tables; no real-policy wire test for hidden _props; e2e covers only the happy path.'
severity: minor
reason: The pure parts are unit-tested (index, apply, cap, verdict gating, save address); _props uses copyVisibleProperties, already covered for cards and list rows; the e2e spec covers the happy path. The page-level and policy tests are a follow-up.
status: deferred
---
