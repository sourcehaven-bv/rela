---
id: RR-NO6P3N
type: review-response
title: Early-return comment overstates unreachability
finding: 'internal/dataentry/watcher.go:423-424 says the real verifier rejects an expired token, so the check "only guards the handler". The gate does more work after verification (provisioning and principal resolution), so a token verified a few milliseconds before exp legitimately reaches this check. Say what the check does instead: a token that expires between verification and subscribe ends here, before any subscription or live registration.'
severity: nit
resolution: The comment now says a token that expired between verification and this point ends the stream before any subscription or live registration.
status: addressed
---
