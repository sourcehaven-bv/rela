---
id: RR-VNAJZ2S
type: review-response
title: Docs say a machine token goes straight to rela
finding: The JWT-mode text implied the client sends its token to rela. With Pratique the client authenticates to the proxy, which forwards its own assertion; and the client and server header defaults differ (Authorization vs X-Auth-Assertion).
severity: significant
resolution: server-security, acl-security, cli-reference and DEC-QKLMMD4 now say the client authenticates to the OIDC proxy, which forwards a signed assertion; --auth-header names the header the proxy reads.
status: addressed
---
