---
id: RR-ZXMHNA
type: review-response
title: --config-editing without acl.yaml gives every caller config:edit
finding: With no acl.yaml, nopReadGate.HoldsPermission returns true (dataentry/readgate.go:150) and the principal is 'unknown' without an identity source. A LAN-bound server with the flag would let anyone rewrite the schema and run ACL-bypassing migrations.
severity: critical
resolution: 'Plan changed: the flag is refused at startup unless acl.yaml is loaded and a verified identity source is configured (JWT or principal header); handlers refuse a zero or unknown principal.'
status: addressed
---
