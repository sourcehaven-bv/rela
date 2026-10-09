---
id: RR-MJKTC4
type: review-response
title: kvpiles reads and writes the whole document for every user
finding: All owners' piles live in one JSON value; every read parses and every write rewrites the whole document under one mutex, so cost scales with all users' piles rather than the caller's.
severity: minor
reason: The kv tier serves desktop and small single-user fs servers; multi-user deployments run on postgres where each owner is a separate row set. Sharding by hashed owner changes the stored layout and the rename/delete hooks (which must then scan every key), so it belongs in its own ticket if fs-tier multi-user load appears.
status: deferred
---
