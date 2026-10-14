---
id: RR-IAO5NC
type: review-response
title: Nop resolver chosen for a policy with only client ceilings
finding: ResolverFromProfile selects the Nop resolver when the policy has no affordance grants, which ignores client_baselines field ceilings. Gate and GET stay consistent, but a ceiling-only policy may redact nothing anywhere. Unverified.
severity: minor
reason: 'Unverified and separate from this bug: list gate and GET stay consistent under the Nop resolver. Needs its own investigation of ResolverFromProfile with a ceiling-only policy.'
status: deferred
---
