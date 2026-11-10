---
id: RR-LTI9QZS
type: review-response
title: Plain http allowed without a credential
finding: Without a token plain http to any host was accepted and the environment proxy stayed active; requests still carry local addresses and answers can be forged.
severity: significant
resolution: Plain http to a host that is not loopback is refused unless --allow-insecure-http; the proxy is disabled for every plain-http request.
status: addressed
---
