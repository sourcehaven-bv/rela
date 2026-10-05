---
id: RR-6OHQDK
type: review-response
title: Faced identity type plus a write grant gave two contradicting errors
finding: 'A faced user or group type named in a create grant was reported twice: once asking to remove its faces and once asking to name a face.'
severity: minor
resolution: validateIdentityStructure returns the set of refused canonical types and the write-grant check skips them. Pinned by a count assertion in TestValidateAgainstMetamodel_IdentityStructure.
status: addressed
---
