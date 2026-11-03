---
id: RR-MQKLQSZ
type: review-response
title: One-line docs example wrappers
finding: docsExampleSessionDisplay only calls docsYAMLAfter.
severity: nit
resolution: Removed the new wrapper; the case calls docsYAMLAfter directly. The older docsExampleChannelsDisplay is left as it was.
status: addressed
---
