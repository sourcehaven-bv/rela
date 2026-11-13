---
id: RR-LSBD0CF
type: review-response
title: gofmt fails on the unpaged-read allowlist
finding: Removing the ingest entry left two blank lines in unpagedread_allowlist_test.go.
severity: critical
resolution: Ran gofmt -w on the file.
status: addressed
---
