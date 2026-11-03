---
id: RR-C7XPVH
type: review-response
title: Backfill runs on context.Background
finding: New runs the search-text backfill on context.Background, so a large upgrade cannot be cancelled.
severity: nit
resolution: sqlitestore.New now takes a context and runs the backfill on it; appbuild passes its context through. Lint (contextcheck) required the same change.
status: addressed
---
