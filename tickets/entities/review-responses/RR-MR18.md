---
id: RR-MR18
type: review-response
title: No user-facing documentation
finding: 'Diff touches no docs. Users will discover the new operators and variables by trial and error. Need to document in data-entry.md: supported operators per type, supported variables, timezone policy, in/ne tokenization quirk.'
severity: minor
resolution: 'Updated docs-project/entities/guide/GUIDE-data-entry.md (which generates docs/data-entry.md) with: full operator table including type support, in operator, type-aware comparison rules, $today/$tomorrow/$yesterday variables with UTC timezone note, in-list variable substitution example, and the literal-$ caveat.'
status: addressed
---
