---
id: RR-JLN7M6W
type: review-response
title: Status page loads every remote's ledger
finding: 'Cranky 5: GET /_remotes reads the ledger of each remote to count linked rows; cost grows with the number of remotes.'
severity: minor
reason: A project has a handful of remotes and the ledger read is one KV get each; the budget test pins per-remote cost at 10 and 50 entries. Revisit if remotes grow past tens.
status: deferred
---
