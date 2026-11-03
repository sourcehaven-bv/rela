---
id: RR-SEXZD5B
type: review-response
title: The appbuild PolicyRedactor primes traversals but not local roles
finding: 'Design review: visibility.PolicyRedactor.PrimeTraversals only primes traversals. A page prime that relies on the redactor alone leaves the per-row local-role probes in place on the appbuild path.'
severity: significant
resolution: primePage calls acl.PrimeLocalRoles itself before PrimeTraversals, so every TraversalPrimer redactor gets both.
status: addressed
---
