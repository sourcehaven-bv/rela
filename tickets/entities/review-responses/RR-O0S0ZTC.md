---
id: RR-O0S0ZTC
type: review-response
title: 'Design: a schema edit can make an unchanged trigger leak'
finding: A schema edit can widen what an unchanged trigger fires on (empty on.entity, a new subtype under a restricted abstract type) without it being judged.
severity: significant
resolution: When schema.yaml changes, every unchanged trigger is judged under the before and after inputs and only violations the edit introduces are refused.
status: addressed
---
