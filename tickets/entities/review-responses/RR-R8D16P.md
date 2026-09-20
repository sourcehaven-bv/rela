---
id: RR-R8D16P
type: review-response
title: Title ranking compared raw text to a lowercased needle
finding: search_text is lowercased at write; properties->>'title' is not, so similarity() against the lowercased needle under-scores mixed-case titles.
severity: critical
resolution: 'Plan: the rank expression wraps the CASE in lower(); test with titles differing only in case.'
status: addressed
---
