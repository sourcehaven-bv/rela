---
id: RR-F9KMYG0
type: review-response
title: Lost cross-process index updates let a state survive a purge
finding: The index is read-modify-write under a process-local lock, so two processes can lose an entry and a purge misses that key.
severity: significant
resolution: A purge writes a random per-entity generation; open refuses a state sealed under an older one.
status: addressed
---
