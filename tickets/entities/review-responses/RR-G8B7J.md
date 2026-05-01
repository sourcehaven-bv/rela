---
id: RR-G8B7J
type: review-response
title: Hardcoded title strings in add test
finding: e2e/tests/reverse-relations.spec.ts lines 103/110 duplicate the fixture title 'Refactor auth module'. Extract to a const so the search query and option text and assertion all derive from one source.
severity: nit
resolution: Extracted candidateTitle and tileTitle constants in both new tests. Search query, option text, tile-by-text lookup, and remove-tile call now all derive from one local.
status: addressed
---
