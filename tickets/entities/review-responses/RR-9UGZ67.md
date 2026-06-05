---
id: RR-9UGZ67
type: review-response
title: primeWatermark scans attachments but catchUp doesn't — silent skip of entity/relation rows
finding: 'Both reviewers (architect + cranky) independently: primeWatermark (listener.go:182-184) computes max(seq) over entities UNION relations UNION attachments, but catchUp (listener.go:197-204) scans only entities UNION relations. Attachments consume rela_seq on every write (attachment.go) and emit NO events. So at startup the watermark can be primed ABOVE the highest entity/relation seq by however many recent attachment writes there were, silently eating the 100-row overlap budget. With >100 attachment writes just before a process starts, a late-committing low-seq entity write can fall below the primed watermark and be skipped forever (until its next mutation). The asymmetry is also undocumented — the next person will ''fix'' one query to match the other and may pick the wrong direction.'
severity: significant
resolution: 'Fixed: primeWatermark now scans only entities UNION relations (dropped attachments), matching catchUp exactly. Added a comment on both queries stating the table sets MUST match and why (attachments consume rela_seq but emit no events, so priming against them would silently shrink the overlap budget). No more skip path.'
status: addressed
---

## Resolution

Make primeWatermark scan only `entities UNION relations` (match catchUp).
There's no reason to prime against a table catch-up never scans. Attachments
emit no events, so excluding them from BOTH the prime and the catch-up is
consistent and correct. Add a comment noting the two queries MUST cover the same
tables.
