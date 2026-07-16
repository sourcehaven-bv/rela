---
id: RR-QOA1Z
type: review-response
title: Read-path guard doc claimed equivalence with the write-path guard's inert tier that it lacks
finding: The read guard fails closed on nil request always; the write-path guard has a policy-active tier (no policy -> inert allow; policy active -> fail closed). The comment claimed 'identical to the write-path guard', which is false. Harmless today because TransitionVerdicts is only wired under an active policy (HasAffordanceGrants), but the comment misrepresents the safety property and would mislead a future no-policy/CLI caller.
severity: significant
resolution: 'Corrected transitionGuard/requestGuard doc: states it fails closed unconditionally, has NO inert tier, and is safe only because it is wired solely under an active policy - with an explicit warning not to wire it from a no-policy path without adding the policy-active tier first. No behavior change (the tier is unreachable by construction today).'
status: addressed
---

## Finding

The read-path guard fails closed on a nil request unconditionally; the
write-path guard (RR-UOBUC) has a `policyActive` tier (inert-allow when no
policy, fail-closed when policy active). The comment claimed the two are
"identical" — false. Harmless today (the machine-backed resolver is only
constructed under an active policy), but the comment lies about the safety
property and would mislead a future no-policy/CLI caller.

## Fix

Rewrote the doc to state: fails closed always; no inert tier; safe only because
wired solely under an active policy; explicit "do not wire from a no-policy path
without adding the policy-active tier" warning. Behavior unchanged (tier is
unreachable by construction).
