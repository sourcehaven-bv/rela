---
id: RR-IKY001
type: review-response
title: 'HTTPHandler comment gave the wrong production topology'
finding: 'The comment said rela-server binds 127.0.0.1. Under that bind relas own requireLocalHost rejects the public Host first; atlas binds 0.0.0.0 and pratique connects over loopback.'
severity: significant
resolution: 'Fixed. Comment and bug text now describe bind 0.0.0.0 with the proxy connecting over 127.0.0.1, matching roles/atlas atlas_http_addr.'
status: addressed
---
