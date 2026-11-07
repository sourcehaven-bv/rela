---
id: RR-X2YUJXP
type: review-response
title: Definition bytes on every Create and loose version check
finding: Each create sends and compares the snapshot bytes; version check allowed 1 to 64 hex though flowcfg makes 16.
severity: minor
resolution: A def_sha256 column compares digests; version must be exactly 16 lowercase hex.
status: addressed
---
