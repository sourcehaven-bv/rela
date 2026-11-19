---
id: RR-5V46IZQ
type: review-response
title: Settle blocks the response for up to 30 s
finding: The settle runs on the creating request after the commit.
severity: minor
reason: A manual start settles inline the same way, and a settle cut off at 30 s is left to the flow worker. Moving it off the request needs a supervised background runner; documented in the guide.
status: deferred
---
