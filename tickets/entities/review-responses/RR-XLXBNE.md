---
id: RR-XLXBNE
type: review-response
title: Retention purge can erase a pre-Tx tombstone of an in-flight or failed delete
finding: 'Plan 0.6 says erasure can never be lost to a rolled-back delete, but channelScope.BeforeDelete tombstones BEFORE the entity Tx. Any hourly or other-entity retention run in that window sees a delete-policy batch with grace 0 and purges it, so a failed Tx makes Untombstone return ErrNotFound and the channels of a still-existing entity are gone (the same happens when Untombstone is refused with OccupiedError). Fix: purge only batches whose commit is confirmed (the post-commit hook marks the batch or enqueues its handle) and/or require a minimum purge grace plus an anchor-absence check for whole-entity batches.'
severity: critical
resolution: Purge acts only on confirmed batches. The post-commit EntityDeleted hook marks the batch handle confirmed in the channel store. An unconfirmed batch whose anchor still exists after 1h is untombstoned (failed or rolled-back delete); an unconfirmed batch whose anchor is gone is confirmed by the sweep.
status: addressed
---
