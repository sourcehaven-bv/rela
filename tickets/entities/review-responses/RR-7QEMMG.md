---
id: RR-7QEMMG
type: review-response
title: Lazy type-record write races Reload and other processes
finding: A post under snapshot S1 records family after Reload installs S2 (face) with no record yet; the next post records face and the sqlchannels upsert overwrites, stranding the family channel. Rolling deploys flip the shared record too.
severity: significant
status: open
---
