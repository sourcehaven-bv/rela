---
id: RR-SZ1NFMN
type: review-response
title: KV backend orphans snapshots and rewrites a large document
finding: Definition keys written outside the document cannot be listed so orphans leak; 10000 runs means MB rewrites per write.
severity: minor
resolution: Definitions live in a second KV document so prune finds every one; cap lowered to 2000 runs and the cost documented.
status: addressed
---
