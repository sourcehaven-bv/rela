---
id: RR-KJDYTU
type: review-response
title: Title map must reach pgstore as plain config
finding: pgstore may not import metamodel; mcp_wiring_postgres.go does not exist.
severity: significant
resolution: 'Plan: map[string]string built in appbuild_postgres.go, values bound as parameters; file list corrected.'
status: addressed
---
