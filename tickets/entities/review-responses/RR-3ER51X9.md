---
id: RR-3ER51X9
type: review-response
title: No test isolates a hidden target in related
finding: The tests do not show a readable source whose target the actor may not read being left out.
severity: minor
reason: related resolves each target through the same gated Reader.Resolve that get uses; that row gate is pinned by the visibility tests. A dedicated fixture with split read grants is not worth it for a test helper.
status: deferred
---
