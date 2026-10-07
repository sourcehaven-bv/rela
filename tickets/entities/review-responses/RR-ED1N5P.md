---
id: RR-ED1N5P
type: review-response
title: 'Design: tagging current state is not compare-and-set'
finding: An edit between a connector's push and its tag call would be absorbed into the sync base and never pushed.
severity: critical
resolution: TagCurrent takes an expected CAS token and refuses with ErrConflict when the live row moved; Lua exposes rela.version_token (R1).
status: addressed
---
