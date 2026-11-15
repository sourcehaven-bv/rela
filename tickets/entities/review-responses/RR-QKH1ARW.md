---
id: RR-QKH1ARW
type: review-response
title: Batch gate repeats per-row reads and condition compiles
finding: Gating each address through boundEntity reads one row and compiles the when lookup per address; at 50 rows that is a per-row lookup on a collection path.
severity: significant
resolution: 'Plan: compile the lookup once, read rows through the gated reader in one pass, fill bodies in one read, run one MatchPage; a storetest.Counting budget test pins it.'
status: addressed
---
