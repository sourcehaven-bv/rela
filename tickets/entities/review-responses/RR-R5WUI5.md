---
id: RR-R5WUI5
type: review-response
title: Candidate query reads all content on every tick
finding: The column gate loaded content and properties of every settled row and its latest version on every tick (measured 285 ms vs 3 ms at 50k rows).
severity: minor
resolution: The hash gate compares two short text columns; no content is loaded for rows that are not candidates.
status: addressed
---
