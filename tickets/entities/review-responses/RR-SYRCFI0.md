---
id: RR-SYRCFI0
type: review-response
title: RoundTrip modified the request and ignored its context
finding: RoundTrip replaced req.Body, which the RoundTripper contract forbids, and answered a cancelled request.
severity: minor
resolution: The body is no longer replaced (form parsing clones the request); a cancelled context returns its error first.
status: addressed
---
