---
id: RR-UCJL
type: review-response
title: Meaningless /tmp ProjectRoot in test
finding: 'runtime_test.go:2357: ws.services("/tmp") — the path is load-bearing nowhere in this test (no write_file). Pass "" or t.TempDir() instead so future readers don''t wonder.'
severity: nit
resolution: Replaced ws.services("/tmp") with ws.services(t.TempDir()) in both new tests. Eliminates the magic path.
status: addressed
---
