---
id: RR-FT1Z65
type: review-response
title: AC2 covers only an added type; a removed or renamed type is not pinned
finding: 'internal/store/fsstore/index_scan_test.go:71-93 (TestIndex_SchemaChangeIsSeenOnReopen) tests only a type that the new schema adds. The design review also named a removed or renamed type: the old cache kept the entries of a type that the schema no longer declared, and they reappeared with a type the metamodel does not know. Add the inverse case: seed WID-1 under a schema that declares `widget`, close, reopen with defaultTestSchemas, and assert ErrNotFound. Use a table with both directions.'
severity: minor
resolution: TestIndex_SchemaChangeIsSeenOnReopen is a table with "type added" and "type removed" cases; both fail on develop.
status: addressed
---
