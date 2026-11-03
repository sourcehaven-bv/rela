---
id: RR-F41C5J
type: review-response
title: A searched list gets no live busy updates
finding: 'liveListPath returns null when q is set (frontend/src/api/live.ts:63-64), because the server refuses q on a live path (livepath.go:52-57). A list with a search term therefore has no subscription. Its busy indicators keep counting minutes on the client clock but never change state until the next refetch. The plan and the GUIDE-realtime ''When it updates'' text do not mention it. Fix: document it next to the grouped-list 100-row limit, and add an EntityList.busy.test.ts case asserting that the list request still carries busy when q is set.'
severity: minor
resolution: ADDRESSED step 11, scope out and GUIDE-realtime When it updates now name the searched-list live limit; AC 6 adds a case that a searched list still sends busy
status: addressed
---
