---
id: RR-1LEDJ4R
type: review-response
title: Nested sections look runs up once per child
finding: buildNestedTree emitted rows with _fields whose children were not primed, so each child cost a run lookup.
severity: significant
resolution: The nested builder primes all emitted rows once; the top-level priming skips nested sections.
status: addressed
---
