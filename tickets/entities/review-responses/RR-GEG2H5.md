---
id: RR-GEG2H5
type: review-response
title: pg backfill casts every message under an exclusive lock
finding: '0035 runs ALTER TABLE (ACCESS EXCLUSIVE on channel_messages) and then calls a plpgsql function that casts doc to jsonb for every row, all in the migration transaction. All chat is blocked until it ends. Fix: prefilter with plain text before the function, for example `WHERE counts = 1 AND (strpos(doc, ''"removed":'') > 0 OR strpos(doc, ''"triggered_by":"session:'') > 0) AND rela_message_counts(doc) = 0`. Both prefixes are what docOf writes. Add a Go test that the ''session:'' literal in both migrations equals channels.SessionMarker, so a later rename cannot drift silently.'
severity: minor
resolution: 0035 prefilters with strpos on '"removed":' and '"triggered_by":"session:' before the function. TestChannelReadsMigrations_SessionMarker checks both migrations against channels.SessionMarker.
status: addressed
---
