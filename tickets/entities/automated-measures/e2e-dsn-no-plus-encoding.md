---
id: 'e2e-dsn-no-plus-encoding'
type: 'automated-measure'
title: 'Spec: the postgres e2e DSN never encodes a space as "+"'
description: 'Guards against BUG-GZQTN0. Asserts directly that the built DSN contains no "+" and that pgDsnForSchema pins the schema through a readable `search_path` parameter. The negative assertion is deliberate: it names the exact literal the server rejects, so a regression reports "+search_path" rather than a connection timeout 300 specs later. It needs no browser and no live database, which is the point — the defect it covers killed rela-server during startup, so every browser spec in the job failed with an error that pointed at postgres configuration instead of at the harness that wrote the DSN. Confirmed to fail against the old URLSearchParams-built `options=-c search_path=...` form and pass against the new one.'
kind: 'test'
location: 'e2e/tests/pg-dsn.spec.ts'
status: 'active'
---
