---
id: RR-R9YHP2
type: review-response
title: sqlite first backlink read reindexed in one unbounded transaction
finding: After the v13 upgrade the first MentionSources call loaded every pending body into memory in one write transaction.
severity: minor
resolution: reindexMentions now fills 500 rows per transaction until none are pending, like pgstore.
status: addressed
---
