---
id: RR-SM1A2S7
type: review-response
title: Plain http allowed to the IdP operator API
finding: The operator API answers are not signed, but -tenant-idp-org-api accepted plain http to any host, so anyone on the path could suspend or resume an org.
severity: minor
resolution: 'Fixed in 3a5b2d9ff: plain http is accepted only for localhost or a loopback IP literal; https and unix sockets are accepted. Any other address stops startup. Pinned by idporg/client_test.go.'
status: addressed
---
