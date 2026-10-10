---
id: RR-ZXMHNA
type: review-response
title: --config-editing without acl.yaml gives every caller config:edit
finding: With no acl.yaml, nopReadGate.HoldsPermission returns true (dataentry/readgate.go:150) and the principal is 'unknown' without an identity source. A LAN-bound server with the flag would let anyone rewrite the schema and run ACL-bypassing migrations.
severity: critical
resolution: 'The flag is refused at startup unless acl.yaml is loaded and an identity source is configured (JWT, principal header, or $RELA_DATAENTRY_USER); handlers refuse a zero or unknown principal. Since the CISO review on #1792 (issue #1793) $RELA_DATAENTRY_USER is refused beyond a loopback bind because it verifies nobody.'
status: addressed
---
