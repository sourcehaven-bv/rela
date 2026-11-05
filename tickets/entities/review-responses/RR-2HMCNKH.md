---
id: RR-2HMCNKH
type: review-response
title: No-identity relation verdict leaves meta open
finding: denyAllRelations left MetaClosed false, so MetaWritable allowed every key when the request had no identity.
severity: minor
resolution: denyAllRelations sets MetaClosed.
status: addressed
---
