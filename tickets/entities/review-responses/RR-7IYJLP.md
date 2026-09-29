---
id: RR-7IYJLP
type: review-response
title: sqlitestore keeps a caller-supplied UpdatedAt
finding: 'Create and update stored e.UpdatedAt when set. entitymanager updates a clone of the stored entity, so updated_at never advanced: the sweep settle window and store.Freshness never saw the edit. pgstore, memstore and fsstore stamp the time themselves.'
severity: significant
resolution: Both writes now stamp time.Now(). New storetest case Entity/UpdateAdvancesUpdatedAt fails on the old code and passes on every backend.
status: addressed
---
