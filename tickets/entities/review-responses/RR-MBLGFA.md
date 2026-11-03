---
id: RR-MBLGFA
type: review-response
title: The deep-link format does not match between the AC and the code
finding: channelKey(type, name) returns 'case/' for a cardinality-one channel (channelView.ts:497-498). Step 9 builds the link with channelKey, but AC 19 and the example use ?channel=case. Choose one format and parse it with parseChannelKey so both forms are accepted, and test both. Step 10 also reads the query only once on first load, so a query change on the same mounted entity page is ignored. That is acceptable, but say so.
severity: minor
resolution: The entity page parses the query with parseChannelKey so both case and case/ work, the link uses channelKey, the section watches the query so a change on the same page applies, and AC 23 tests both forms.
status: addressed
---
