---
id: RR-XK1JR9
type: review-response
title: +N order is by name, not by relevance
finding: 'busy.go:124-126 sorts by channel name, and the card shows the first entry plus "+N". For cardinality many, the visible entry is the lexically first channel, not the longest-running or most recent one. Fix: sort by Since (oldest first, the likeliest stall), then by name, and document the order in the BusyView godoc and api-reference.'
severity: nit
resolution: Entries now sort by since (oldest first) then name, so the shown entry is the longest in its state; godoc, API reference and guide updated; TestBusy_ManyChannels checks the order.
status: addressed
---
