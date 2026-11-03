---
id: RR-3PIQKS
type: review-response
title: Generated ACL docs edited by hand
finding: docs/acl-overview.md and docs/acl-security.md are generated from docs-project guides; the guides were not changed.
severity: critical
resolution: The same edits are in GUIDE-acl-overview and GUIDE-acl-security. scripts/generate-docs.sh reproduces docs/ without a diff.
status: addressed
---
