---
id: RR-C3Y8ZQF
type: review-response
title: 'Code: docs overstate SQL-only fold and snapshots'
finding: storage.md said the fold is decided per row in SQL; the package doc said NULL rows are pending on every backend and implied one snapshot per call.
severity: minor
resolution: storage.md names the Go paths (elementnaive and the sqlite fallback); the store doc limits NULL-pending to the database backends; RangeEntry doc says elementnaive reads several times per call.
status: addressed
---
