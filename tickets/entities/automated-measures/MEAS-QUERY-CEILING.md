---
id: MEAS-QUERY-CEILING
type: automated-measure
title: Per-request query ceiling and N+1 shape detector
description: rela-server --query-ceiling (default 500) logs a warning for a request over the ceiling, with its route and query shape and the statement it repeated most. The e2e harness fails a test whose server logged it. Only database backends count statements.
kind: monitoring
location: internal/dataentry/requeststats.go (warnQueryCeiling); internal/store/querystats.go (RecordStatement/MostRepeated); e2e/tests/fixtures.ts (failOnQueryCeiling)
status: active
---
