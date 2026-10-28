---
id: RR-CQ6LFI
type: review-response
title: Security text claims refused posts are rate limited, but they are not
finding: 'The security section says a caller who spams wrong-state posts "is limited by the per-person channel write budget like any refused write". Refusals before limiter.take (service.go:386) charge nothing: postDenied, signals_not_kept and channel_not_open are free, and the plan puts session_wrong_state in the same place. The check is cheap (one head read that the call does anyway), so there is no amplification, but the sentence is false. Replace it with "a refusal costs one head read, as a role denial does".'
severity: minor
resolution: The false sentence is replaced with "a pre-check refusal costs one head read, as a role denial does".
status: addressed
---
