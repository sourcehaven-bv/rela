---
id: RR-FJHKLK
type: review-response
title: 'aggregateLifetime fails the whole lifetime listing when a lineage has no rows left'
finding: 'internal/store/sqlitestore/relation_version.go: the aggregate scanned min(created_at)/max(created_at) into string. On an empty id-set count(*) is 0 and both are NULL, which database/sql rejects with a conversion error rather than sql.ErrNoRows — so ListRelationLifetimes fails entirely. Reachable rather than theoretical: purging a whole lineage with --all while a newer lifetime on the same key survives leaves a head whose rows are gone.'
severity: significant
resolution: 'Scan into *string/*int and treat NULL as a zero lifetime, returning the count the query actually reported. The comment names the reachable path so the nullable scan does not look like defensive noise.'
status: addressed
---
