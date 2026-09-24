---
id: RR-WHC2XQ
type: review-response
title: Per-row grant traversal binds on data-entry read paths
finding: 'Several data-entry paths (GET, includes, export, next-action, feed, caldav, sections) redacted rows without priming grant traversals, so a traversing when: bound once per row.'
severity: significant
resolution: Each path primes via primeVerdicts or visibility.PrimeTraversals; affRedactor implements TraversalPrimer; budget tests pin size independence.
status: addressed
---
