---
id: RR-T14RK0
type: review-response
title: attach_render target lookup reveals whether a hidden entity exists
finding: writeTarget read the raw stored type first. An absent id returned 'entity not found' before any gate ran; a hidden id ran the gated preflight and returned 'entity or its file property not found'. A script could tell hidden from missing in the message and in the timing (RR-NGMI). Raised by both the cranky and the security reviewer.
severity: critical
resolution: An absent id now runs the same gated preflight against the first type that declares the property, and an unparsable address, an empty stored type, a hidden row and a delete between check and write all return one message. The Refusals test expects the same text for the missing and the hidden case.
status: addressed
---
