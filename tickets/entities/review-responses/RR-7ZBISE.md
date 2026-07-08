---
id: RR-7ZBISE
type: review-response
title: Reused entity ID silently merged two entities' version histories
finding: 'cranky-code-reviewer: lineage() in pgstore/version.go used unbounded `entity_id = ANY(ids)` with no vseq fence. rela permits id reuse (rename A→B frees id A). So after rename A→B, a new unrelated entity reclaiming id A would (a) have its versions bleed into A''s old timeline and (b) get pulled into B''s lineage via the rename walk — silent cross-entity data corruption, and `restore B 1` could restore B to a different entity''s content. Fatal for an audit feature.'
severity: critical
resolution: Rewrote the lineage query as a WITH RECURSIVE CTE producing [lo,hi) vseq-fenced segments per id. The head id's lower bound is the vseq of the most recent rename that renamed it AWAY (0 if never); each predecessor hop is bounded above by its rename's vseq and below by its own prior rename-away. A reclaimed id contributes only its in-window rows. Also fixed a COLLATE C mismatch on the recursive head term. Regression test TestReusedIDDoesNotMergeHistories (DB-verified) asserts neither the reclaimed A nor B sees the other's content.
status: addressed
---
