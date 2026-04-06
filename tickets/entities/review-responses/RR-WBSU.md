---
id: RR-WBSU
type: review-response
title: 'daily-parse TOCTOU: crash between create and annotate causes duplicates'
finding: If script crashes between create_entity and update_entity, task exists but annotation is never written. Next run creates duplicate. Should write annotation immediately or check spawned relations.
severity: significant
reason: TOCTOU window is very small (between create_entity and update_entity in same script run). Proper fix requires GFM task list support to check spawned relations instead of annotations. Acceptable risk for personal PIM.
status: deferred
---
