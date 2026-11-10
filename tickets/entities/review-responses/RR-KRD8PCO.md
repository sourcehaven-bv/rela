---
id: RR-KRD8PCO
type: review-response
title: An empty outside_edits block builds a ceiling
finding: outside_edits with no rules still built a ceiling that costs run lookups and locks nothing.
severity: nit
resolution: An empty block warns 'declares no rules' and compiles to nil; a table case pins it.
status: addressed
---
