---
id: RR-NVOXIC
type: review-response
title: postgres build silently falls back to node-local kvpiles
finding: '[security] internal/appbuild/piles_postgres.go + piles.go: when pgstore.HandleFor returns nil the postgres build uses the tenant-independent cache-dir kvpiles; with SharedBase several tenants would share one piles.json keyed by login name. Should fail instead of falling back.'
severity: minor
resolution: On the postgres build a store without a pg handle gets an in-memory backend and an error log, never the shared cache-dir file. Not a hard error because untagged appbuild tests assemble a memstore under -tags postgres. TestPiles_NonDatabaseStoreFallback.
status: addressed
---
