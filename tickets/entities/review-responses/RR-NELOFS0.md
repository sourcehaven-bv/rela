---
id: RR-NELOFS0
type: review-response
title: 'Design: unhashed rows drop out of range sums'
finding: Clearing a stored element_address makes an unhashed row invisible to a range scan, so two sides can match while they differ.
severity: significant
resolution: 'Plan changed: the address is not stored. It is an index expression over the key columns, held in one Go constant per backend. An unhashed row keeps its address; step 3 compares count(*) with count(element_hash).'
status: addressed
---
