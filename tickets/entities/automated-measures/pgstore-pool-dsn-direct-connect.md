---
id: pgstore-pool-dsn-direct-connect
type: automated-measure
title: 'pgstore: every direct connection accepts a pool-tuned DSN'
description: A test opens the store with a DSN carrying pool_max_conns (and other pool_* parameters) and asserts the change-feed listener connects and delivers a cross-process event. Fails for any direct pgx.Connect that forwards pool-only parameters to the server.
kind: test
location: internal/store/pgstore/listener_test.go (TestCrossProcessPropagation_PoolTunedDSN)
status: active
---

## What it checks

The store is opened against a DSN with `pool_max_conns` set, and a write from a
second store must arrive through the LISTEN/NOTIFY feed. Every code path that
calls `pgx.Connect` on the configured DSN is covered this way, so a new direct
connection that forwards pool-only parameters fails the test.
