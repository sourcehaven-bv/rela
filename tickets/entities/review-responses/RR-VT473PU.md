---
id: RR-VT473PU
type: review-response
title: Sub-property types bypass refs and mentions
finding: Refs are only collected at top level and mentions only scanned in top-level markdown, so a ref or markdown sub-property would show raw ids or render unresolved mentions.
severity: significant
resolution: 'Plan: sub-properties are limited to string, integer, boolean, enum and string lists. No ref or markdown inside objects.'
status: addressed
---
