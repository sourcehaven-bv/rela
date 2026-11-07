---
id: RR-UUKPM67
type: review-response
title: 'Code: one malformed stored hash fails the whole range'
finding: A stored element hash that does not parse returned an error for every call over that range.
severity: minor
resolution: Such a row is loaded and recomputed like an unfilled one and a warning is logged.
status: addressed
---
