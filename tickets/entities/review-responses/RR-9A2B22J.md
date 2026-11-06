---
id: RR-9A2B22J
type: review-response
title: Stamp mutation contract undocumented
finding: collabStampUser mutates and returns its argument; the hook's godoc did not say that is allowed.
severity: nit
resolution: ConnectionConfig.StampAwareness godoc says the stamp may change and return its argument.
status: addressed
---
