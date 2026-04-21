---
id: RR-UHIF6
type: review-response
title: e2e readiness probe accepts any 403 as 'server up'
finding: 'isServerRunning was relaxed to accept 403 because the probe''s origin_missing rejection made it look down. That also masks 403s from real breakage (misconfigured allowed-origins, future auth). Better: send a matching Origin header from the probe, or hit a non-sensitive path.'
severity: significant
resolution: waitForServer / isServerRunning now take an `origin` parameter and send it as the Origin header on the probe. The broad 403-accepts-as-up was removed — genuine auth/config 403s will now make the probe look down, as they should. backend fixture passes baseUrl as the origin.
status: addressed
---
