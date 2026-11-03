---
id: RR-QWWJYWQ
type: review-response
title: Unbounded IN list in sqlite Successors
finding: The id list from mentions.All has no cap. A body with more ids than SQLite's variable limit errors and drops all mentions.
severity: nit
resolution: sqlite Successors chunks the id list in batches of 500.
status: addressed
---
