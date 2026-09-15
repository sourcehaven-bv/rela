---
id: 'dsn-query-reencoding-test'
type: 'automated-measure'
title: 'Test: raising pool_max_conns leaves every other DSN parameter byte-for-byte intact'
description: 'Guards against BUG-P1QKMB. The load-bearing detail is that both tests assert through pgx (pgconn.ParseConfig and pgxpool.ParseConfig) rather than url.Query(). That is deliberate: url.Query() decodes "+" back to a space, so a test written against it agrees with itself no matter what ensurePoolFloor wrote to the wire, and the original defect was invisible to the pre-existing tests for exactly that reason. Parsing the way the server parses is what makes these tests able to fail. TestEnsurePoolFloor_PreservesOptionsEncoding first asserts a precondition (pgx reads the fixture DSN correctly) so a broken fixture cannot pass as a preserved one, then requires the options runtime parameter to be identical before and after sizing the pool. TestEnsurePoolFloor_RaisesWithoutReEncoding covers the other branch, the one that REPLACES an existing too-small pool_max_conns rather than appending a missing one, since the append path alone would leave half the function unguarded. Both were confirmed to fail against the pre-fix implementation with SQLSTATE 42704, not merely to pass against the new one.'
kind: 'test'
location: 'internal/jobs/pgqueue_pool_test.go:TestEnsurePoolFloor_PreservesOptionsEncoding, TestEnsurePoolFloor_RaisesWithoutReEncoding'
status: 'active'
---
