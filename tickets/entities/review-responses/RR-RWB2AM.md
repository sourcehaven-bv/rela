---
id: RR-RWB2AM
type: review-response
title: ListAnchor can pass more than MaxReadKeys keys to Reads
finding: 'ListEntity returns every channel of the entity. channels_per_entity allows 200 per type per anchor, so several types across faces can exceed 2000 stored channels. Reads then answers ErrInvalidKey and the list becomes a 500. Fix: the service chunks Reads by MaxReadKeys (the budget becomes "one Reads per 2000 channels"), or Reads chunks internally as it already does for SQL. Add a test at 2001 channels.'
severity: minor
resolution: Service chunks Reads by MaxReadKeys in ListAnchor; budget reads "one Reads per 2000 channels". TestListAnchor_ReadsChunks at 2001 channels.
status: addressed
---
