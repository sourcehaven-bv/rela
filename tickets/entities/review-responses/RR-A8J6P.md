---
id: RR-A8J6P
type: review-response
title: Capture original source bytes on inline nodes for round-trip-faithful AST
finding: Reviewer suggested an optional `_raw` field with `_dirty` flag. The renderer prefers `_raw` if the node was unmodified. Cleanly solves code-span fences, link URL escaping, and unknown-inline preservation.
severity: minor
reason: Bigger architectural change; right shape for the long term. Filed for follow-up after current refactor stabilizes.
status: deferred
---
