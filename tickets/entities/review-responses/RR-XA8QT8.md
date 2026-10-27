---
id: RR-XA8QT8
type: review-response
title: Adding where props can write into the cached ACL template
finding: append on the shared Props backing array could bleed scope across requests.
severity: significant
resolution: visibility clones Props and Related before appending (slices.Clone); a test runs two roll-ups on one request and checks the template is unchanged.
status: addressed
---
