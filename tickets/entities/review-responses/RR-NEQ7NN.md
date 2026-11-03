---
id: RR-NEQ7NN
type: review-response
title: Backfill does not fit the migration mechanisms
finding: pgstore.Migrate is one SQL transaction; sqlitedb cannot parse markdown.
severity: significant
resolution: 'Migrations stay DDL-only. pg: a background reindexer fills rows whose mention_targets is NULL in batches of 500, at open and every five minutes. sqlite: the first MentionSources call fills pending rows in batches of 500. fs builds lazily from files.'
status: addressed
---
