---
id: RR-GTUN3Y4
type: review-response
title: No EventSource in node for the contract test
finding: Comparing SSE frames needs an EventSource client in node, which would add a dependency.
severity: minor
resolution: The contract test reads /api/v1/_events with fetch streaming and parses the frames itself.
status: addressed
---
