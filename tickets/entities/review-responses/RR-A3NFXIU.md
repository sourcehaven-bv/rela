---
id: RR-A3NFXIU
type: review-response
title: Move gate test passes on any error and has no allowed case
finding: The manager test accepted any error and had no case where the gate allows the move.
severity: minor
resolution: The gate returns a sentinel matched with errors.Is; allowed outgoing and incoming cases assert the swap and the judged sources.
status: addressed
---
