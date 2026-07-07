---
id: RR-R1LAKE
type: review-response
title: Whitespace-only header is truthy in resolver
finding: listHeaderMarkdown uses `header || description`, so a whitespace-only header ('   ') is truthy. It currently works because renderMarkdown collapses it to '' and the v-if gates on rendered output, but that's incidental. Add .trim() so intent is explicit and the guard is robust regardless of what gates downstream.
severity: nit
resolution: Added .trim() to listHeaderMarkdown and listFooterMarkdown so whitespace-only values are treated as unset explicitly. Added two unit tests (whitespace-only header falls back to description; both whitespace → '').
status: addressed
---
