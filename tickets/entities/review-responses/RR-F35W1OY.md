---
id: RR-F35W1OY
type: review-response
title: Gated relation page trickles one round trip per hidden edge
finding: relationpage.go set q.Limit = want - len(out) on every refill. Near a full page or with a small limit each raw read asked for 1 or 2 rows so K hidden edges cost about K store reads plus K gate reads. Both reviewers (cranky critical; security significant as resource exhaustion).
severity: critical
resolution: The first raw page asks for exactly q.Limit; refills ask for at least headerGateChunk (512) rows. When the page fills mid raw page cursorAfter re-reads that raw page up to the last served row so the cursor still follows a visible row. TestScriptReader_ListRelationsPageHiddenEdgesCostChunks pins 2000 hidden edges at limit 1 to at most 6 raw reads. A raw-scan budget is deferred to TKT-OGL697S (store pushdown of the peer gate) which removes the scan.
status: addressed
---
