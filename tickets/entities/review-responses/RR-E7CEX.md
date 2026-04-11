---
id: RR-E7CEX
type: review-response
title: Negative timeout not validated in config
finding: Config validation does not check for negative timeout values. A negative timeout creates an already-cancelled context.
severity: significant
resolution: Added negative timeout validation in config.validate(). Added test TestParseConfig_negativeTimeout
status: addressed
---
