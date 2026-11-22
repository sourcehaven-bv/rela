---
id: RR-6P2JE80
type: review-response
title: Test gaps in the differential
finding: No non-constant V compared with the predicate engine; no EndpointMatch RowCond case.
severity: minor
resolution: Added gina (status shown on tickets titled b) to the MCP differential, which also covers a hidden comparison with related(...) in one Where, and an EndpointMatch case to RowCond_matches_the_query.
status: addressed
---
