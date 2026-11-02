---
id: RR-RU46NRB
type: review-response
title: Read-mark hints need their own kind
finding: The hint has only kinds m and v plus flat key and reader lists. A receiver would apply mark-read keys through markAs (replaying other subscribers as durable changes) or lose which keys belong to which reader.
severity: significant
resolution: 'Added hint kinds r (one reader and its keys; applied only through markFor) and a (one key with targets; applied through markAddressed). Test: a remote mark read leaves other subscribers'' Keys and Durable empty.'
status: addressed
---
