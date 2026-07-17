---
id: RR-I3G8A2
type: review-response
title: Lone delete-version with no preceding create when create+delete fall inside the debounce window
finding: 'A relation created and deleted within the sweep debounce window produces a timeline that is a single `delete` version with no preceding `create`: the sync delete fires immediately, but the sweep''s candidate query only sees LIVE rows (FROM relations) and the row is already gone, so `create` never fires. Fix: on synchronous relation delete, if no prior version exists for that lineage, the `delete` VersionInput must carry the full pre-delete snapshot (design already does this) AND the timeline must render a lone `delete` as ''created and deleted within the debounce window'' (documented), OR emit a synthetic create+delete pair. Mirror the entity two-LATERAL logic so a re-created (from,t,to) after delete starts a fresh rel_record_id and does not dedup against the pre-delete content hash (cranky S1).'
severity: significant
resolution: 'Design revised: sync delete carries the full pre-delete snapshot even with no prior swept create; timeline renders a lone delete as ''created and deleted within the debounce window''; re-created triple after delete gets a fresh rel_record_id (no cross-delete dedup). See ''Create+delete inside the debounce window''.'
status: addressed
---
