---
id: RR-QSKDTL
type: review-response
title: Context candidate must prime verdicts and use the gated matcher
finding: redactedForSuggestion relies on primeVerdicts; CandidateFunc has no entity param.
severity: minor
resolution: 'Planned: the request closure captures the parsed id; primeVerdicts on the single entity before redaction; related() goes through gatedNextActionMatchers; ACL test with a hidden related entity.'
status: addressed
---
