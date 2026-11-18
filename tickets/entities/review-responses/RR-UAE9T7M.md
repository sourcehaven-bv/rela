---
id: RR-UAE9T7M
type: review-response
title: '[security] Prune deletes tampered rows without checking'
finding: '[security] Prune deleted rows without verifying them, so a prune could erase evidence of an edit.'
severity: significant
resolution: Prune walks the rows it would delete and refuses with ErrChainBroken on a break. Pinned by PruneRefusesBrokenChain.
status: addressed
---
