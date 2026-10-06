---
id: RR-2KK5SV
type: review-response
title: Batch world ranking not compared with the single read
finding: The batch ranks worlds with store.ResolveWorldPrimes while InWorld lets the store rank; nothing compared them.
severity: nit
resolution: Added TestResolver_ResolveHeadersAgreesWithInWorld covering chain, FallbackDefaultState and exclude.
status: addressed
---
